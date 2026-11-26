package campaigns

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// The delivery engine's per-tick sizing (docs/ARCHITECTURE.md §6.3, §7).
//
// Pacing happens at send time (messaging.Throttle: number bucket, campaign
// lane, per-campaign share). The dispatcher only decides how much of a
// campaign is in flight and whether the portfolio's messaging limit lets it
// reach more customers. Each tick it
//  1. reads the number's capacity (Meta throughput, AIMD rate, reserved
//     share),
//  2. computes the weighted fair share of every running campaign on the
//     number that still has recipients (priority weights, rate caps) and
//     stores its own share for the send path,
//  3. keeps share × inflightSeconds messages in flight (window_size, when
//     set, overrides this),
//  4. with a limited messaging tier, counts the customers the account
//     reached outside their service window in the last 24 hours (and the
//     campaign messages still in flight) under a per-account advisory lock,
//     and queues new customers only while the limit allows.

// NumberCapacity is a number's capacity as the dispatcher uses it.
type NumberCapacity struct {
	messaging.Capacity
	PhoneNumberID uuid.UUID
}

func (s *Service) numberCapacity(ctx context.Context, q *db.Queries, pn uuid.UUID) (NumberCapacity, error) {
	if s.sched != nil {
		return NumberCapacity{Capacity: s.sched.Capacity(ctx, pn), PhoneNumberID: pn}, nil
	}
	ceiling := messaging.DefaultThroughput
	v, err := q.GetPhoneNumberThroughput(ctx, pn)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return NumberCapacity{}, err
	}
	if v != nil && *v > 0 {
		ceiling = int(*v)
	}
	r := messaging.DefaultReservedShare
	return NumberCapacity{PhoneNumberID: pn, Capacity: messaging.Capacity{Ceiling: ceiling, Rate: ceiling, ReservedShare: r,
		Campaigns: float64(ceiling) * (1 - r)}}, nil
}

// contenders are the running campaigns on c's number that still have
// recipients, with c itself always included.
func (s *Service) contenders(ctx context.Context, q *db.Queries, c db.Campaign) ([]messaging.Contender, error) {
	rows, err := q.ContendingCampaigns(ctx, c.PhoneNumberID)
	if err != nil {
		return nil, err
	}
	out := make([]messaging.Contender, 0, len(rows)+1)
	self := false
	for _, r := range rows {
		if r.ID == c.ID {
			self = true
			r.Priority, r.MaxMps = c.Priority, c.MaxMps
		}
		out = append(out, contender(r.ID, r.Priority, r.MaxMps))
	}
	if !self {
		out = append(out, contender(c.ID, c.Priority, c.MaxMps))
	}
	return out, nil
}

func contender(id uuid.UUID, priority string, maxMPS *float64) messaging.Contender {
	k := messaging.Contender{ID: id, Weight: messaging.WeightOf(priority)}
	if maxMPS != nil {
		k.Cap = *maxMPS
	}
	return k
}

func shareOf(capacity float64, cs []messaging.Contender, c db.Campaign) float64 {
	return messaging.FairShares(capacity, cs)[c.ID]
}

// limitBudget is how many more customers the portfolio may reach outside
// their service window in this tick.
type limitBudget struct {
	left      int64
	accountID uuid.UUID
}

// counts reports whether a template to contact now would count against the
// limit: not when their service window with the number is open, nor when
// they already count (reached in the last 24 hours).
func (b *limitBudget) counts(ctx context.Context, q *db.Queries, pn, contact uuid.UUID) (bool, error) {
	open, err := messaging.WindowOpenFor(ctx, q, contact, pn)
	if err != nil || open {
		return false, err
	}
	reached, err := q.ReachedRecently(ctx, db.ReachedRecentlyParams{AccountID: b.accountID, ContactID: contact})
	if err != nil {
		return false, err
	}
	return !reached, nil
}

// limitState is a portfolio's messaging limit and its use.
type limitState struct {
	AccountID uuid.UUID
	Tier      string
	// Limit is 0 when the tier is unknown or unlimited.
	Limit, Used, InFlight int64
	FreesAt               *time.Time
}

func (s *Service) limitState(ctx context.Context, q *db.Queries, pnID uuid.UUID, lock bool) (limitState, error) {
	pn, err := q.GetPhoneNumber(ctx, pnID)
	if errors.Is(err, pgx.ErrNoRows) {
		return limitState{}, nil
	}
	if err != nil {
		return limitState{}, err
	}
	st := limitState{AccountID: pn.AccountID, Tier: pn.MessagingLimit, Limit: TierLimit(pn.MessagingLimit)}
	if st.Limit == 0 {
		return st, nil
	}
	if lock {
		if err := q.LockPortfolio(ctx, pn.AccountID); err != nil {
			return st, err
		}
	}
	u, err := q.PortfolioUsage(ctx, pn.AccountID)
	if err != nil {
		return st, err
	}
	st.Used = u.Used
	if u.Used > 0 {
		st.FreesAt = &u.FreesAt
	}
	if st.InFlight, err = q.PortfolioInFlight(ctx, pn.AccountID); err != nil {
		return st, err
	}
	return st, nil
}

// TierLimit turns Meta's messaging limit tier (TIER_250, TIER_2K, TIER_10K,
// TIER_100K …) into the number of unique customers per 24 hours; 0 means no
// cap (TIER_UNLIMITED, or unknown).
func TierLimit(tier string) int64 {
	t := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(tier)), "TIER_")
	if t == "" || t == "UNLIMITED" {
		return 0
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(t, "K"):
		mult, t = 1000, strings.TrimSuffix(t, "K")
	case strings.HasSuffix(t, "M"):
		mult, t = 1_000_000, strings.TrimSuffix(t, "M")
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n * mult
}

// tickPlan is the sizing of one dispatcher tick.
type tickPlan struct {
	Share    float64
	Inflight int64
	Budget   *limitBudget
	Limit    limitState
}

func (p tickPlan) limitDetail() map[string]any {
	resume := time.Now().Add(time.Minute)
	if p.Limit.FreesAt != nil && p.Limit.FreesAt.After(resume) {
		resume = *p.Limit.FreesAt
	}
	return map[string]any{"reason": PauseReasonLimit, "tier": p.Limit.Tier, "limit": p.Limit.Limit, "used": p.Limit.Used,
		"in_flight": p.Limit.InFlight, "resume_at": resume.UTC()}
}

func (s *Service) plan(ctx context.Context, q *db.Queries, c db.Campaign) (tickPlan, error) {
	capa, err := s.numberCapacity(ctx, q, c.PhoneNumberID)
	if err != nil {
		return tickPlan{}, err
	}
	cs, err := s.contenders(ctx, q, c)
	if err != nil {
		return tickPlan{}, err
	}
	p := tickPlan{Share: shareOf(capa.Campaigns, cs, c)}
	if s.sched != nil {
		s.sched.SetShare(ctx, c.PhoneNumberID, c.ID, p.Share)
	}
	p.Inflight = max(minInflight, int64(math.Ceil(p.Share*inflightSeconds)))
	if c.WindowSize > 0 {
		p.Inflight = int64(c.WindowSize)
	}
	if p.Limit, err = s.limitState(ctx, q, c.PhoneNumberID, true); err != nil {
		return p, err
	}
	if p.Limit.Limit > 0 {
		p.Budget = &limitBudget{accountID: p.Limit.AccountID, left: p.Limit.Limit - p.Limit.Used - p.Limit.InFlight}
	}
	return p, nil
}

// remaining is what a campaign still has to send, from its counts.
func remaining(counts map[string]int64) int64 {
	return counts["recipients_pending"] + counts["recipients_retry"] + counts["pending"] + counts["sending"] + counts["held"]
}

// eta estimates the seconds left at the measured rate (nil when unknown).
func eta(c db.Campaign, counts map[string]int64) *int64 { return ETA(c.Status, c.SendRate, counts) }

// ETA estimates the seconds a campaign in status, sending at the measured
// rate, needs for what is left in counts (nil when unknown).
func ETA(status string, rate *float64, counts map[string]int64) *int64 {
	if status != "running" || rate == nil || *rate <= 0 {
		return nil
	}
	v := int64(math.Ceil(float64(remaining(counts)) / *rate))
	return &v
}

// QueueCampaign is one campaign in the send queue view.
type QueueCampaign struct {
	ID              uuid.UUID
	Name, Status    string
	Priority        string
	Weight          float64
	MaxMPS          *float64
	ShareMPS        float64
	RateMPS         *float64
	Remaining       int64
	TotalRecipients int64
	ETASeconds      *int64
	PausedReason    *string
	ResumeAt        *time.Time
}

// QueueNumber is one phone number in the send queue view.
type QueueNumber struct {
	NumberCapacity
	Limit     limitState
	Campaigns []QueueCampaign
}

// Queue is the central send queue: per number, its capacity, the reserved
// share, and the campaigns sharing it with their share, rate and ETA.
func (s *Service) Queue(ctx context.Context) ([]QueueNumber, error) {
	rows, err := s.q.CampaignQueue(ctx)
	if err != nil {
		return nil, err
	}
	byNumber := map[uuid.UUID][]db.CampaignQueueRow{}
	var order []uuid.UUID
	for _, r := range rows {
		if _, ok := byNumber[r.PhoneNumberID]; !ok {
			order = append(order, r.PhoneNumberID)
		}
		byNumber[r.PhoneNumberID] = append(byNumber[r.PhoneNumberID], r)
	}
	out := make([]QueueNumber, 0, len(order))
	for _, pn := range order {
		capa, err := s.numberCapacity(ctx, s.q, pn)
		if err != nil {
			return nil, err
		}
		lim, err := s.limitState(ctx, s.q, pn, false)
		if err != nil {
			return nil, err
		}
		var cs []messaging.Contender
		for _, r := range byNumber[pn] {
			if r.Status == "running" && r.Waiting > 0 {
				cs = append(cs, contender(r.ID, r.Priority, r.MaxMps))
			}
		}
		shares := messaging.FairShares(capa.Campaigns, cs)
		qn := QueueNumber{NumberCapacity: capa, Limit: lim}
		for _, r := range byNumber[pn] {
			qc := QueueCampaign{ID: r.ID, Name: r.Name, Status: r.Status, Priority: r.Priority, Weight: messaging.WeightOf(r.Priority),
				MaxMPS: r.MaxMps, ShareMPS: shares[r.ID], RateMPS: r.SendRate, Remaining: r.Waiting + r.InFlight,
				TotalRecipients: r.TotalRecipients, PausedReason: r.PausedReason, ResumeAt: r.ResumeAt}
			rate := qc.ShareMPS
			if r.SendRate != nil && *r.SendRate > 0 {
				rate = *r.SendRate
			}
			if r.Status == "running" && rate > 0 {
				v := int64(math.Ceil(float64(qc.Remaining) / rate))
				qc.ETASeconds = &v
			}
			qn.Campaigns = append(qn.Campaigns, qc)
		}
		out = append(out, qn)
	}
	return out, nil
}

// maxResends bounds the new messages a recipient gets after Meta withheld
// an accepted one (131049): three attempts in all.
const maxResends = 2

// ScheduleResend is called, inside the webhook transaction, when Meta
// reports an accepted campaign message as withheld for ecosystem health
// (131049). While the campaign is running or paused, the recipient waits a
// day and then gets a new message (linked by messages.retry_of); consent
// is checked again then. A finished campaign leaves it failed: the
// "previous campaign's unreached customers" audience can target it.
func (s *Service) ScheduleResend(ctx context.Context, q *db.Queries, m db.Message) error {
	if m.CampaignID == nil {
		return nil
	}
	_, err := q.ScheduleRecipientResend(ctx, db.ScheduleRecipientResendParams{MessageID: &m.ID,
		AfterSeconds: whatsapp.EcosystemRetryAfter.Seconds(), MaxResends: maxResends})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}
