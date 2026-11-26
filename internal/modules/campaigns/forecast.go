package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/phonecountry"
)

// Pre-launch analysis (Phase 16C acceptance 6). A forecast tells, before a
// campaign starts, who would get it (consent preflight), how long it takes
// with the number's real capacity and the portfolio's messaging limit, what
// Meta would charge by the imported rate card, and — only when there is
// enough history — how it would perform. Nothing is guessed: a missing rate
// is counted as "not in the rate card", missing history is reported as
// insufficient. The forecast is stored, unchangeable, when the campaign
// starts (campaign_forecasts) and compared with the actual figures
// (forecast vs actual) using the same definitions (docs/CAMPAIGN_ANALYTICS.md).

// History needed before performance is forecast: completed campaigns with
// a template of the same category, and recipients across them.
const (
	MinHistoryCampaigns  = 3
	MinHistoryRecipients = 500
)

// Rate statuses.
const (
	RatesOK                  = "ok"
	RatesInsufficientHistory = "insufficient_history"
)

// ForecastInput is what a forecast is made for.
type ForecastInput struct {
	Audience         Audience
	PhoneNumberID    *uuid.UUID
	TemplateName     string
	TemplateLanguage string
	Priority         string
	MaxMPS           *float64
	// StartAt is when sending starts (nil = now).
	StartAt *time.Time
	// CampaignID excludes the campaign itself from history and contenders.
	CampaignID *uuid.UUID
}

// Forecast is a campaign's pre-launch analysis.
type Forecast struct {
	GeneratedAt    time.Time         `json:"generated_at"`
	Category       string            `json:"category"`
	Total          int64             `json:"total"`
	Eligible       int64             `json:"eligible"`
	Blocked        int64             `json:"blocked"`
	Warned         int64             `json:"warned"`
	BlockedReasons []PreflightReason `json:"blocked_reasons"`
	Capacity       ForecastCapacity  `json:"capacity"`
	Limit          ForecastLimit     `json:"limit"`
	StartAt        time.Time         `json:"start_at"`
	// DurationSeconds and FinishAt are nil when the campaign would get no
	// capacity at all.
	DurationSeconds *int64         `json:"duration_seconds"`
	FinishAt        *time.Time     `json:"finish_at"`
	Cost            ForecastCost   `json:"cost"`
	Rates           ForecastRates  `json:"rates"`
	Expected        ExpectedCounts `json:"expected"`
}

// ForecastCapacity is the share the campaign would get on its number.
type ForecastCapacity struct {
	CeilingMPS          int                 `json:"ceiling_mps"`
	RateMPS             int                 `json:"rate_mps"`
	ReservedShare       float64             `json:"reserved_share"`
	CampaignCapacityMPS float64             `json:"campaign_capacity_mps"`
	ShareMPS            float64             `json:"share_mps"`
	Others              []ForecastContender `json:"others"`
}

// ForecastContender is another running campaign sharing the number.
type ForecastContender struct {
	ID       uuid.UUID `json:"id"`
	Priority string    `json:"priority"`
	ShareMPS float64   `json:"share_mps"`
}

// ForecastLimit is the portfolio's messaging limit as it affects the
// campaign. Counting is how many eligible customers would count against it
// (service window closed and not already counted in the last 24 hours);
// Windows how many 24-hour limit windows sending needs (1 = fits now).
type ForecastLimit struct {
	Tier      string `json:"tier"`
	Limit     int64  `json:"limit"`
	Used      int64  `json:"used"`
	InFlight  int64  `json:"in_flight"`
	Remaining int64  `json:"remaining"`
	Counting  int64  `json:"counting"`
	Windows   int    `json:"windows"`
}

// ForecastCost is the estimated Meta cost (per-message prices of the rate
// card valid at the start, by recipient country and the template's
// category; tiers at the month's start: an upper bound for tiered
// categories).
type ForecastCost struct {
	Currency string `json:"currency"`
	// Amount is a decimal string of the priced recipients' total.
	Amount string `json:"amount"`
	// Charged recipients with a price, free ones (utility inside an open
	// window before 2026-10-01), and charged ones whose country/category
	// has no rate in the card ("fiyat kartında yok").
	Priced        int64             `json:"priced"`
	Free          int64             `json:"free"`
	NotInRateCard int64             `json:"not_in_rate_card"`
	ByCountry     []ForecastCountry `json:"by_country"`
}

// ForecastCountry is the cost of one recipient country.
type ForecastCountry struct {
	Country    string  `json:"country"`
	Recipients int64   `json:"recipients"`
	Charged    int64   `json:"charged"`
	Price      *string `json:"price"`
	Amount     *string `json:"amount"`
}

// ForecastRates are performance rates from history (pooled over the
// campaigns: Σ numerator / Σ denominator). They are nil, with Status
// insufficient_history and what is missing, without enough history.
type ForecastRates struct {
	Status             string   `json:"status"`
	Campaigns          int      `json:"campaigns"`
	Recipients         int64    `json:"recipients"`
	CampaignsNeeded    int      `json:"campaigns_needed"`
	RecipientsNeeded   int64    `json:"recipients_needed"`
	SentRate           *float64 `json:"sent_rate"`
	DeliveryRate       *float64 `json:"delivery_rate"`
	ReadRate           *float64 `json:"read_rate"`
	ReplyRate          *float64 `json:"reply_rate"`
	PositiveRate       *float64 `json:"positive_rate"`
	NegativeRate       *float64 `json:"negative_rate"`
	OptOutRate         *float64 `json:"opt_out_rate"`
	BasedOnCampaignIDs []string `json:"based_on"`
}

// ExpectedCounts are the forecast counts (nil without history).
type ExpectedCounts struct {
	Sent      *int64 `json:"sent"`
	Delivered *int64 `json:"delivered"`
	Read      *int64 `json:"read"`
	Replied   *int64 `json:"replied"`
	Positive  *int64 `json:"positive"`
	Negative  *int64 `json:"negative"`
	OptedOut  *int64 `json:"opted_out"`
}

// Forecast computes the pre-launch analysis of a campaign that is not
// stored yet (wizard) or not started yet.
func (s *Service) Forecast(ctx context.Context, in ForecastInput) (Forecast, error) {
	pre, err := s.Preflight(ctx, in.Audience, in.PhoneNumberID, in.TemplateName, in.TemplateLanguage)
	if err != nil {
		return Forecast{}, err
	}
	var pn db.PhoneNumber
	if in.PhoneNumberID != nil {
		pn, err = s.q.GetPhoneNumber(ctx, *in.PhoneNumberID)
	} else {
		pn, err = s.q.GetDefaultPhoneNumber(ctx)
	}
	if err != nil {
		return Forecast{}, err
	}
	if in.Priority == "" {
		in.Priority = "normal"
	}
	if err := validateSettings(&in.Priority, in.MaxMPS); err != nil {
		return Forecast{}, err
	}
	now := time.Now().UTC()
	start := now
	if in.StartAt != nil && in.StartAt.After(now) {
		start = in.StartAt.UTC()
	}
	out := Forecast{GeneratedAt: now, Category: pre.Category, Total: pre.Total, Eligible: pre.Eligible, Blocked: pre.Blocked,
		Warned: pre.Warned, BlockedReasons: pre.BlockedReasons, StartAt: start}
	if out.BlockedReasons == nil {
		out.BlockedReasons = []PreflightReason{}
	}
	tpl, err := s.q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: in.TemplateName, Language: in.TemplateLanguage})
	if err != nil {
		return out, err
	}
	rules, err := rulesFor(ctx, s.q, pn.ID, in.TemplateName, in.TemplateLanguage)
	if err != nil {
		return out, err
	}

	// Capacity: the number's campaign lane split by weight with the
	// campaigns running on it now.
	capa, err := s.numberCapacity(ctx, s.q, pn.ID)
	if err != nil {
		return out, err
	}
	self := uuid.Nil
	if in.CampaignID != nil {
		self = *in.CampaignID
	}
	rows, err := s.q.ContendingCampaigns(ctx, pn.ID)
	if err != nil {
		return out, err
	}
	cs := []messaging.Contender{contender(self, in.Priority, in.MaxMPS)}
	for _, r := range rows {
		if r.ID != self {
			cs = append(cs, contender(r.ID, r.Priority, r.MaxMps))
		}
	}
	shares := messaging.FairShares(capa.Campaigns, cs)
	out.Capacity = ForecastCapacity{CeilingMPS: capa.Ceiling, RateMPS: capa.Rate, ReservedShare: capa.ReservedShare,
		CampaignCapacityMPS: capa.Campaigns, ShareMPS: shares[self], Others: []ForecastContender{}}
	for _, r := range rows {
		if r.ID != self {
			out.Capacity.Others = append(out.Capacity.Others, ForecastContender{ID: r.ID, Priority: r.Priority, ShareMPS: shares[r.ID]})
		}
	}

	// Audience by consent, country, window and limit status.
	f, err := s.filter(ctx, in.Audience)
	if err != nil {
		return out, err
	}
	groups, err := s.contacts.ForecastGroups(ctx, f, pn.ID, pn.AccountID)
	if err != nil {
		return out, err
	}
	var eligible []costGroup
	var counting int64
	for _, g := range groups {
		if d := consent.Decide(g.State, rules.category, rules.policy); !d.Allowed() {
			continue
		}
		eligible = append(eligible, costGroup{Country: phonecountry.Of(g.Prefix), WindowOpen: g.WindowOpen, N: g.N})
		if !g.WindowOpen && !g.Reached {
			counting += g.N
		}
	}

	lim, err := s.limitState(ctx, s.q, pn.ID, false)
	if err != nil {
		return out, err
	}
	sec, windows := planDuration(out.Eligible, counting, out.Capacity.ShareMPS, lim)
	out.Limit = ForecastLimit{Tier: lim.Tier, Limit: lim.Limit, Used: lim.Used, InFlight: lim.InFlight,
		Remaining: max(0, lim.Limit-lim.Used-lim.InFlight), Counting: counting, Windows: windows}
	if lim.Limit == 0 {
		out.Limit.Remaining = 0
	}
	if sec != nil {
		out.DurationSeconds = sec
		fin := start.Add(time.Duration(*sec) * time.Second)
		out.FinishAt = &fin
	}

	// Cost.
	acc, err := s.q.GetWhatsAppAccount(ctx, pn.AccountID)
	if err != nil {
		return out, err
	}
	currency := ""
	if acc.Currency != nil {
		currency = *acc.Currency
	}
	card, err := metacost.LoadCard(ctx, s.q, currency)
	if err != nil {
		return out, err
	}
	out.Cost = estimateCost(card, eligible, metacost.TemplateCategory(tpl.Category), start)
	if out.Cost.Currency == "" {
		out.Cost.Currency = currency
	}

	// Performance from history.
	hist, err := s.q.ForecastHistory(ctx, db.ForecastHistoryParams{Category: tpl.Category, ExcludeID: in.CampaignID})
	if err != nil {
		return out, err
	}
	out.Rates = historyRates(hist)
	out.Expected = expected(out.Eligible, out.Rates)
	return out, nil
}

// planDuration estimates the seconds to send eligible recipients at share
// messages per second, when counting of them count against a portfolio
// limit (lim.Limit 0 = none). Over the limit, the first window takes what
// remains of it and every further 24-hour window up to the full limit; it
// returns the duration (nil without capacity) and the number of windows.
func planDuration(eligible, counting int64, share float64, lim limitState) (*int64, int) {
	windows := 1
	var waitSec float64
	var lastBatch = float64(eligible)
	if lim.Limit > 0 {
		remaining := max(0, lim.Limit-lim.Used-lim.InFlight)
		if counting > remaining {
			over := counting - remaining
			extra := int((over + lim.Limit - 1) / lim.Limit)
			windows += extra
			// The last window sends what is left of the counting customers
			// (customers that do not count went out in the first one).
			left := over - int64(extra-1)*lim.Limit
			waitSec = float64(extra) * 24 * 3600
			lastBatch = float64(left)
		}
	}
	if eligible == 0 {
		z := int64(0)
		return &z, windows
	}
	if share <= 0 || math.IsNaN(share) {
		return nil, windows
	}
	sendSec := float64(eligible) / share
	if windows > 1 {
		sendSec = math.Max(sendSec, waitSec+lastBatch/share)
	}
	v := int64(math.Ceil(sendSec))
	return &v, windows
}

// costGroup is a number of eligible recipients in one country whose service
// window with the number is open or not.
type costGroup struct {
	Country    string
	WindowOpen bool
	N          int64
}

// estimateCost prices groups by the rate card at time at (metacost.Charged
// decides what Meta charges). Tiered categories use the month's first
// tier (no volume this month assumed: an upper bound).
func estimateCost(card metacost.Card, groups []costGroup, category string, at time.Time) ForecastCost {
	out := ForecastCost{Currency: card.Currency, ByCountry: []ForecastCountry{}}
	byCountry := map[string]*ForecastCountry{}
	var total metacost.Micros
	for _, g := range groups {
		fc, ok := byCountry[g.Country]
		if !ok {
			fc = &ForecastCountry{Country: g.Country}
			byCountry[g.Country] = fc
		}
		fc.Recipients += g.N
		if !metacost.Charged(category, at, g.WindowOpen, 0) {
			out.Free += g.N
			continue
		}
		fc.Charged += g.N
		rate, ok := card.RateFor(g.Country, category, at, 0)
		if !ok || g.Country == "" {
			out.NotInRateCard += g.N
			continue
		}
		out.Priced += g.N
		if out.Currency == "" {
			out.Currency = rate.Currency
		}
		p := rate.Price.String()
		fc.Price = &p
		amount := metacost.Micros(int64(rate.Price) * g.N)
		total += amount
		prev := metacost.Micros(0)
		if fc.Amount != nil {
			prev, _ = metacost.ParseMicros(*fc.Amount)
		}
		a := (prev + amount).String()
		fc.Amount = &a
	}
	out.Amount = total.String()
	for _, fc := range byCountry {
		out.ByCountry = append(out.ByCountry, *fc)
	}
	sort.Slice(out.ByCountry, func(i, j int) bool {
		if out.ByCountry[i].Recipients != out.ByCountry[j].Recipients {
			return out.ByCountry[i].Recipients > out.ByCountry[j].Recipients
		}
		return out.ByCountry[i].Country < out.ByCountry[j].Country
	})
	return out
}

// historyRates pools the rates of completed campaigns, or reports what is
// missing. The definitions are those of the actual figures (Rates).
func historyRates(rows []db.ForecastHistoryRow) ForecastRates {
	out := ForecastRates{Status: RatesInsufficientHistory, CampaignsNeeded: MinHistoryCampaigns, RecipientsNeeded: MinHistoryRecipients,
		BasedOnCampaignIDs: []string{}}
	var k Counters
	for _, r := range rows {
		if r.Recipients == 0 {
			continue
		}
		out.Campaigns++
		out.Recipients += r.Recipients
		out.BasedOnCampaignIDs = append(out.BasedOnCampaignIDs, r.ID.String())
		k.Recipients += r.Recipients
		k.OBlocked += r.OBlocked
		k.Sent += r.Sent
		k.Delivered += r.Delivered
		k.Read += r.Read
		k.Replied += r.Replied
		k.Positive += r.Positive
		k.Negative += r.Negative
		k.OptedOut += r.OptedOut
	}
	if out.Campaigns < MinHistoryCampaigns || out.Recipients < MinHistoryRecipients {
		return out
	}
	out.Status = RatesOK
	rt := RatesOf(k)
	out.SentRate, out.DeliveryRate, out.ReadRate, out.ReplyRate = rt.Sent, rt.Delivery, rt.Read, rt.Reply
	out.PositiveRate, out.NegativeRate, out.OptOutRate = rt.Positive, rt.Negative, rt.OptOut
	return out
}

// Rates are a campaign's performance rates; one definition for forecast
// and actual (docs/CAMPAIGN_ANALYTICS.md): eligible = recipients − blocked;
// sent / eligible; delivered / sent; read / delivered; replied, positive,
// negative and opted out / delivered. nil when the denominator is 0.
type Rates struct {
	Sent, Delivery, Read, Reply, Positive, Negative, OptOut *float64
}

// RatesOf computes the rates of counters.
func RatesOf(k Counters) Rates {
	ratio := func(n, d int64) *float64 {
		if d <= 0 {
			return nil
		}
		v := math.Round(float64(n)/float64(d)*10000) / 10000
		return &v
	}
	eligible := k.Recipients - k.OBlocked
	return Rates{Sent: ratio(k.Sent, eligible), Delivery: ratio(k.Delivered, k.Sent), Read: ratio(k.Read, k.Delivered),
		Reply: ratio(k.Replied, k.Delivered), Positive: ratio(k.Positive, k.Delivered), Negative: ratio(k.Negative, k.Delivered),
		OptOut: ratio(k.OptedOut, k.Delivered)}
}

// expected turns rates into counts for an eligible audience.
func expected(eligible int64, r ForecastRates) ExpectedCounts {
	if r.Status != RatesOK || r.SentRate == nil {
		return ExpectedCounts{}
	}
	mul := func(base *int64, rate *float64) *int64 {
		if base == nil || rate == nil {
			return nil
		}
		v := int64(math.Round(float64(*base) * *rate))
		return &v
	}
	sent := mul(&eligible, r.SentRate)
	delivered := mul(sent, r.DeliveryRate)
	return ExpectedCounts{Sent: sent, Delivered: delivered, Read: mul(delivered, r.ReadRate), Replied: mul(delivered, r.ReplyRate),
		Positive: mul(delivered, r.PositiveRate), Negative: mul(delivered, r.NegativeRate), OptedOut: mul(delivered, r.OptOutRate)}
}

// forecastFor computes the forecast of a stored campaign.
func (s *Service) forecastFor(ctx context.Context, c db.Campaign) (Forecast, error) {
	var a Audience
	if err := json.Unmarshal(c.Audience, &a); err != nil {
		return Forecast{}, err
	}
	return s.Forecast(ctx, ForecastInput{Audience: a, PhoneNumberID: &c.PhoneNumberID, TemplateName: c.TemplateName,
		TemplateLanguage: c.TemplateLanguage, Priority: c.Priority, MaxMPS: c.MaxMps, CampaignID: &c.ID})
}

// snapshotForecast stores the forecast of a campaign that starts sending
// (once; later starts keep the first). A forecast that cannot be made is
// logged, never blocks sending.
func (s *Service) snapshotForecast(ctx context.Context, c db.Campaign) {
	if _, err := s.q.GetCampaignForecast(ctx, c.ID); err == nil {
		return
	}
	f, err := s.forecastFor(ctx, c)
	if err != nil {
		s.log.WarnContext(ctx, "campaign forecast not stored", "campaign_id", c.ID, "error", err)
		return
	}
	raw, _ := json.Marshal(f)
	if _, err := s.q.InsertCampaignForecast(ctx, db.InsertCampaignForecastParams{CampaignID: c.ID, Forecast: raw}); err != nil {
		s.log.WarnContext(ctx, "campaign forecast not stored", "campaign_id", c.ID, "error", err)
	}
}

// StoredForecast returns the forecast fixed when the campaign started.
func (s *Service) StoredForecast(ctx context.Context, id uuid.UUID) (*Forecast, time.Time, error) {
	row, err := s.q.GetCampaignForecast(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	var f Forecast
	if err := json.Unmarshal(row.Forecast, &f); err != nil {
		return nil, row.CreatedAt, err
	}
	return &f, row.CreatedAt, nil
}

// ForecastVsActual compares the stored forecast with the campaign's
// current (live) or final figures.
type ForecastVsActual struct {
	Final      bool
	Forecast   *Forecast
	StoredAt   time.Time
	Metrics    []Comparison
	ActualCost string
	Currency   string
}

// Comparison is one metric: forecast, actual and the difference (actual −
// forecast; relative to the forecast when it is not 0).
type Comparison struct {
	Metric           string
	Forecast, Actual *float64
	Delta, DeltaPct  *float64
}

// CompareForecast builds forecast vs actual for campaign id.
func (s *Service) CompareForecast(ctx context.Context, id uuid.UUID) (ForecastVsActual, error) {
	c, err := s.q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ForecastVsActual{}, apperr.NotFound("Campaign not found.")
	}
	if err != nil {
		return ForecastVsActual{}, err
	}
	f, at, err := s.StoredForecast(ctx, id)
	if err != nil {
		return ForecastVsActual{}, err
	}
	k, err := counters(ctx, s.q, id)
	if err != nil {
		return ForecastVsActual{}, err
	}
	out := ForecastVsActual{Final: c.Status == "completed" || c.Status == "cancelled" || c.Status == "failed", Forecast: f, StoredAt: at,
		ActualCost: k.Cost}
	if f == nil {
		return out, nil
	}
	out.Currency = f.Cost.Currency
	num := func(v int64) *float64 { x := float64(v); return &x }
	opt := func(v *int64) *float64 {
		if v == nil {
			return nil
		}
		return num(*v)
	}
	rate := func(v *float64) *float64 { return v }
	act := RatesOf(k)
	var duration *float64
	if c.StartedAt != nil {
		end := time.Now()
		if c.FinishedAt != nil {
			end = *c.FinishedAt
		}
		duration = num(int64(end.Sub(*c.StartedAt).Seconds()))
	}
	fcost, _ := metacost.ParseMicros(f.Cost.Amount)
	acost, _ := metacost.ParseMicros(k.Cost)
	add := func(metric string, fv, av *float64) {
		cmp := Comparison{Metric: metric, Forecast: fv, Actual: av}
		if fv != nil && av != nil {
			d := math.Round((*av-*fv)*10000) / 10000
			cmp.Delta = &d
			if *fv != 0 {
				p := math.Round((*av-*fv) / *fv * 10000) / 100
				cmp.DeltaPct = &p
			}
		}
		out.Metrics = append(out.Metrics, cmp)
	}
	add("total", num(f.Total), num(k.Recipients))
	add("eligible", num(f.Eligible), num(k.Recipients-k.OBlocked))
	add("blocked", num(f.Blocked), num(k.OBlocked))
	add("sent", opt(f.Expected.Sent), num(k.Sent))
	add("delivered", opt(f.Expected.Delivered), num(k.Delivered))
	add("read", opt(f.Expected.Read), num(k.Read))
	add("replied", opt(f.Expected.Replied), num(k.Replied))
	add("positive", opt(f.Expected.Positive), num(k.Positive))
	add("negative", opt(f.Expected.Negative), num(k.Negative))
	add("opted_out", opt(f.Expected.OptedOut), num(k.OptedOut))
	add("sent_rate", rate(f.Rates.SentRate), act.Sent)
	add("delivery_rate", rate(f.Rates.DeliveryRate), act.Delivery)
	add("read_rate", rate(f.Rates.ReadRate), act.Read)
	add("reply_rate", rate(f.Rates.ReplyRate), act.Reply)
	add("positive_rate", rate(f.Rates.PositiveRate), act.Positive)
	add("opt_out_rate", rate(f.Rates.OptOutRate), act.OptOut)
	add("cost", ptrF(float64(fcost)/1e6), ptrF(float64(acost)/1e6))
	add("duration_seconds", opt(f.DurationSeconds), duration)
	return out, nil
}

func ptrF(v float64) *float64 { return &v }
