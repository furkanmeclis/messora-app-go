package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The campaign detail's analysis center. Every figure says where it comes
// from: "meta" for what Meta reported (sent, delivered, read, failed and
// its errors, the pricing category of each message), "system" for what
// Messora computed (audience, consent blocks, replies, reactions, opt-outs,
// forecasts, costs from the imported rate card).

// Metric sources.
const (
	SourceMeta   = "meta"
	SourceSystem = "system"
)

// Metric is one figure of the analysis center.
type Metric struct {
	Key    string
	Value  int64
	Source string
}

// CostCategory is the campaign's cost of one pricing category and type.
type CostCategory struct {
	Category, Type, Cost, Currency string
	Messages, Unpriced             int64
}

// AICost is the AI cost of a campaign in one currency.
type AICost struct {
	Currency, Cost string
	Requests       int64
}

// CostDay is Meta's reconciled cost of one day and category of the
// campaign's number (all its messages, not only the campaign's).
type CostDay struct {
	Day                      time.Time
	Category, Cost, Currency string
	Volume                   int64
}

// AccountEvent is one event of the quality timeline.
type AccountEvent struct {
	Kind       string
	OccurredAt time.Time
	Payload    map[string]any
}

// Analysis is the analysis center of one campaign.
type Analysis struct {
	Counts  map[string]int64
	Metrics []Metric
	Rates   Rates
	// Cost: the per-message estimate (system: Meta's pricing category ×
	// the rate card) and Meta's reconciled daily totals for the number.
	Currency      string
	CostEstimate  string
	Priced        int64
	Unpriced      int64
	CostBreakdown []CostCategory
	Reconciled    []CostDay
	// Spam and quality.
	NumberQuality   string
	TemplateStatus  string
	TemplateQuality string
	Timeline        []AccountEvent
	Classes         map[string]int64
	Hourly          []HourlyPoint
	// AICost is the AI usage attributed to the campaign (P7; empty until
	// then) per currency; TotalCost is Meta + AI when they share the
	// currency (the campaign's operating cost), "" otherwise.
	AICost    []AICost
	TotalCost string
	// QuickReplies are the template's quick reply button texts (the keys of
	// a reaction mapping).
	QuickReplies []string
}

// Analysis computes the analysis center of campaign id.
func (s *Service) Analysis(ctx context.Context, id uuid.UUID) (Analysis, error) {
	c, err := s.q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, apperr.NotFound("Campaign not found.")
	}
	if err != nil {
		return Analysis{}, err
	}
	k, err := counters(ctx, s.q, id)
	if err != nil {
		return Analysis{}, err
	}
	out := Analysis{Counts: countsOf(k), Rates: RatesOf(k), CostEstimate: k.Cost, Priced: k.Priced, Unpriced: k.Unpriced}
	out.Metrics = []Metric{
		{"audience", k.Recipients, SourceSystem},
		{"eligible", k.Recipients - k.OBlocked, SourceSystem},
		{"blocked", k.OBlocked, SourceSystem},
		{"pending", k.OPending, SourceSystem},
		{"sent", k.Sent, SourceMeta},
		{"delivered", k.Delivered, SourceMeta},
		{"read", k.Read, SourceMeta},
		{"failed", k.OFailed, SourceMeta},
		{"rejected", k.Rejected, SourceMeta},
		{"unknown", k.OUnknown, SourceMeta},
		{"replied", k.Replied, SourceSystem},
		{"positive", k.Positive, SourceSystem},
		{"negative", k.Negative, SourceSystem},
		{"neutral", k.Neutral, SourceSystem},
		{"opted_out", k.OptedOut, SourceSystem},
		{"err_131048", k.Err131048, SourceMeta},
		{"err_131049", k.Err131049, SourceMeta},
		{"err_131050", k.Err131050, SourceMeta},
	}
	cats, err := s.q.CampaignCostByCategory(ctx, &id)
	if err != nil {
		return out, err
	}
	out.CostBreakdown = []CostCategory{}
	for _, r := range cats {
		out.CostBreakdown = append(out.CostBreakdown, CostCategory{Category: r.Category, Type: r.Type, Cost: r.Cost, Currency: r.Currency,
			Messages: r.Messages, Unpriced: r.Unpriced})
		if out.Currency == "" {
			out.Currency = r.Currency
		}
	}
	pn, err := s.q.GetPhoneNumber(ctx, c.PhoneNumberID)
	if err != nil {
		return out, err
	}
	out.NumberQuality = pn.QualityRating
	if acc, err := s.q.GetWhatsAppAccount(ctx, pn.AccountID); err == nil && out.Currency == "" && acc.Currency != nil {
		out.Currency = *acc.Currency
	}
	ai, err := s.q.CampaignAICost(ctx, &id)
	if err != nil {
		return out, err
	}
	out.AICost = []AICost{}
	meta, _ := metacost.ParseMicros(out.CostEstimate)
	total := meta
	for _, x := range ai {
		out.AICost = append(out.AICost, AICost{Currency: x.Currency, Cost: x.Cost, Requests: x.Requests})
	}
	switch {
	case len(ai) == 0:
		out.TotalCost = total.String()
	case len(ai) == 1 && (ai[0].Currency == out.Currency || out.Currency == ""):
		v, _ := metacost.ParseMicros(ai[0].Cost)
		out.TotalCost = (total + v).String()
	}
	out.QuickReplies = []string{}
	if t, err := s.q.CampaignTemplate(ctx, id); err == nil {
		out.TemplateStatus, out.TemplateQuality = t.Status, t.QualityScore
		out.QuickReplies = QuickReplyTexts(t.Components)
	}
	from := c.CreatedAt
	if c.StartedAt != nil {
		from = *c.StartedAt
	}
	to := time.Now()
	if c.FinishedAt != nil {
		to = *c.FinishedAt
	}
	out.Reconciled = []CostDay{}
	if c.StartedAt != nil {
		days, err := s.q.CampaignCostDaily(ctx, db.CampaignCostDailyParams{PhoneNumberID: c.PhoneNumberID,
			FromDay: pgtype.Date{Time: from.UTC().Truncate(24 * time.Hour), Valid: true}, ToDay: pgtype.Date{Time: to.UTC(), Valid: true}})
		if err != nil {
			return out, err
		}
		for _, d := range days {
			out.Reconciled = append(out.Reconciled, CostDay{Day: d.Day.Time, Category: d.PricingCategory, Cost: d.Cost, Currency: d.Currency, Volume: d.Volume})
		}
	}
	evs, err := s.q.CampaignAccountEvents(ctx, db.CampaignAccountEventsParams{AccountID: pn.AccountID, FromAt: from.Add(-24 * time.Hour),
		ToAt: to, PhoneNumberID: &c.PhoneNumberID})
	if err != nil {
		return out, err
	}
	out.Timeline = []AccountEvent{}
	for _, e := range evs {
		p := map[string]any{}
		_ = json.Unmarshal(e.Payload, &p)
		if e.Kind == "template_change" && p["template"] != c.TemplateName {
			continue
		}
		out.Timeline = append(out.Timeline, AccountEvent{Kind: e.Kind, OccurredAt: e.OccurredAt, Payload: p})
	}
	if out.Classes, err = classCounts(ctx, s.q, id); err != nil {
		return out, err
	}
	out.Hourly, err = s.Hourly(ctx, id)
	return out, err
}

// SetReactionMap stores the campaign's quick reply mapping and relabels
// the recipients who already tapped a button (manual labels stay).
func (s *Service) SetReactionMap(ctx context.Context, id uuid.UUID, m map[string]string) (db.Campaign, error) {
	if len(m) > 50 {
		return db.Campaign{}, apperr.Validation("reaction_map", "At most 50 buttons can be mapped.")
	}
	for k, v := range m {
		if k == "" || len(k) > 256 {
			return db.Campaign{}, apperr.Validation("reaction_map", "Button payloads are 1–256 characters.")
		}
		if !ValidReaction(v) {
			return db.Campaign{}, apperr.Validation("reaction_map", "Each button is positive, negative or neutral.")
		}
	}
	raw, _ := json.Marshal(m)
	var c db.Campaign
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		var err error
		c, err = q.SetCampaignReactionMap(ctx, db.SetCampaignReactionMapParams{ID: id, ReactionMap: raw})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Campaign not found.")
		}
		if err != nil {
			return err
		}
		if _, err := q.RemapQuickReplyReactions(ctx, db.RemapQuickReplyReactionsParams{CampaignID: id, ReactionMap: raw}); err != nil {
			return err
		}
		return s.publish(ctx, tx, q, c)
	})
	return c, err
}

// QuickReplies lists the quick reply buttons of the campaign's template
// (the keys a reaction mapping can use: Meta sends the button text as the
// payload of template quick replies unless a payload was set at send time).
func (s *Service) QuickReplies(ctx context.Context, id uuid.UUID) ([]string, error) {
	t, err := s.q.CampaignTemplate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return QuickReplyTexts(t.Components), nil
}

// QuickReplyTexts returns the texts of a template's quick reply buttons.
func QuickReplyTexts(components json.RawMessage) []string {
	var comps []struct {
		Type    string `json:"type"`
		Buttons []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"buttons"`
	}
	_ = json.Unmarshal(components, &comps)
	out := []string{}
	for _, c := range comps {
		for _, b := range c.Buttons {
			if b.Type == "QUICK_REPLY" || b.Type == "quick_reply" {
				out = append(out, b.Text)
			}
		}
	}
	return out
}
