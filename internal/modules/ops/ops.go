// Package ops is the operations center: GET /ops/overview gathers what needs
// attention now (campaigns, conversations, automations and alerts) in one
// call, each section filtered by the caller's permissions. The page stays
// current without polling: it follows the existing realtime events
// (campaign:all, the inbox channels) and the ops.updated hints on
// system:ops, and re-reads only the section that changed.
package ops

import (
	"context"
	"encoding/binary"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/conversations"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Sections of the overview.
const (
	SectionCampaigns     = "campaigns"
	SectionConversations = "conversations"
	SectionActivity      = "activity"
	SectionAutomations   = "automations"
	SectionAlerts        = "alerts"
)

// AllSections in the order of the response.
var AllSections = []string{SectionCampaigns, SectionConversations, SectionActivity, SectionAutomations, SectionAlerts}

// Bounds of the lists.
const (
	campaignLimit = 30
	listLimit     = 20
	eventLimit    = 10
)

// Permitted reports whether p may read section.
func Permitted(p *principal.Principal, section string) bool {
	if p == nil || p.Kind != principal.KindUser {
		return false
	}
	switch section {
	case SectionCampaigns:
		return p.Has("campaigns.read")
	case SectionConversations:
		return p.Has(conversations.PermRead) || p.Has(conversations.PermReadAll)
	case SectionActivity:
		// Message counts of every conversation.
		return p.Has(conversations.PermReadAll)
	case SectionAutomations:
		return p.Has("automations.read")
	case SectionAlerts:
		return p.Has("settings.read")
	}
	return false
}

// Resolve returns the requested sections p may read (all of them when
// requested is empty), in the order of AllSections.
func Resolve(p *principal.Principal, requested []string) []string {
	out := []string{}
	for _, s := range AllSections {
		if (len(requested) == 0 || slices.Contains(requested, s)) && Permitted(p, s) {
			out = append(out, s)
		}
	}
	return out
}

// Deps are the parts of other modules the overview reads.
type Deps struct {
	Campaigns *campaigns.Service
	// FailedJobs counts the tenant's part of the dead letter (nil: 0).
	FailedJobs func(context.Context) (int, error)
	// Budget reads the monthly AI budget (nil: none).
	Budget func(ctx context.Context) (ai.Budget, error)
}

// Service builds the overview.
type Service struct {
	q    *db.Queries
	deps Deps
	now  func() time.Time
}

// New returns the service.
func New(pool *pgxpool.Pool, deps Deps) *Service {
	return &Service{q: db.New(pool), deps: deps, now: time.Now}
}

// SinceID is the smallest UUIDv7 of instant t: rows with a UUIDv7 key
// created at or after t have a key at or above it, so a primary key range
// bounds a scan by time.
func SinceID(t time.Time) uuid.UUID {
	var u uuid.UUID
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(t.UnixMilli())) //nolint:gosec // after 1970
	copy(u[:6], ms[2:])
	u[6] = 0x70 // version 7
	u[8] = 0x80 // RFC 4122 variant
	return u
}

// StartOfDay is midnight of t's day in loc.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// Campaigns is the campaign section.
type Campaigns struct {
	Counts db.OpsCampaignStatusCountsRow
	Active []ActiveCampaign
	Queue  []campaigns.QueueNumber
}

// ActiveCampaign is a campaign with its counters.
type ActiveCampaign struct {
	db.OpsActiveCampaignsRow
	Counts map[string]int64
}

// CampaignSection reads counts by state, the active campaigns and the send
// queue.
func (s *Service) CampaignSection(ctx context.Context, dayStart time.Time) (Campaigns, error) {
	var out Campaigns
	var err error
	if out.Counts, err = s.q.OpsCampaignStatusCounts(ctx, dayStart); err != nil {
		return out, err
	}
	rows, err := s.q.OpsActiveCampaigns(ctx, campaignLimit)
	if err != nil {
		return out, err
	}
	out.Active = make([]ActiveCampaign, len(rows))
	for i, r := range rows {
		counts, err := s.deps.Campaigns.Stats(ctx, r.ID)
		if err != nil {
			return out, err
		}
		out.Active[i] = ActiveCampaign{OpsActiveCampaignsRow: r, Counts: counts}
	}
	out.Queue, err = s.deps.Campaigns.Queue(ctx)
	return out, err
}

// Conversations is the conversation section, limited to what the viewer
// may open.
type Conversations struct {
	AwaitingHuman  []db.OpsAwaitingHumanRow
	Unassigned     []db.OpsUnassignedWaitingRow
	WaitingReply   []db.OpsWaitingReplyRow
	AIActive       int64
	RecentInbound  []db.OpsRecentMessagesRow
	RecentOutbound []db.OpsRecentMessagesRow
}

// ConversationSection reads the conversation section for p.
func (s *Service) ConversationSection(ctx context.Context, p *principal.Principal) (Conversations, error) {
	var out Conversations
	var err error
	all := conversations.SeesAll(p)
	if out.AwaitingHuman, err = s.q.OpsAwaitingHuman(ctx, db.OpsAwaitingHumanParams{SeeAll: all, ViewerID: p.ID, Lim: listLimit}); err != nil {
		return out, err
	}
	// Unassigned conversations are visible to everyone with conversations.read.
	if out.Unassigned, err = s.q.OpsUnassignedWaiting(ctx, listLimit); err != nil {
		return out, err
	}
	if out.WaitingReply, err = s.q.OpsWaitingReply(ctx, db.OpsWaitingReplyParams{SeeAll: all, ViewerID: p.ID, Lim: listLimit}); err != nil {
		return out, err
	}
	if out.AIActive, err = s.q.OpsAIActiveCount(ctx, db.OpsAIActiveCountParams{SeeAll: all, ViewerID: p.ID}); err != nil {
		return out, err
	}
	since := SinceID(s.now().Add(-24 * time.Hour))
	recent := func(dir string) ([]db.OpsRecentMessagesRow, error) {
		return s.q.OpsRecentMessages(ctx, db.OpsRecentMessagesParams{Direction: dir, SinceID: since, SeeAll: all, ViewerID: p.ID, Lim: listLimit})
	}
	if out.RecentInbound, err = recent("inbound"); err != nil {
		return out, err
	}
	out.RecentOutbound, err = recent("outbound")
	return out, err
}

// ActivitySection reads messages per hour of the last 24 hours.
func (s *Service) ActivitySection(ctx context.Context) ([]db.OpsMessageHoursRow, error) {
	return s.q.OpsMessageHours(ctx, SinceID(s.now().Add(-24*time.Hour).Truncate(time.Hour)))
}

// Automations is the automation section.
type Automations struct {
	Running []db.OpsActiveRunsRow
	Failed  []db.OpsFailedRunsRow
}

// AutomationSection reads the runs in progress and those that failed in
// the last 24 hours.
func (s *Service) AutomationSection(ctx context.Context) (Automations, error) {
	var out Automations
	var err error
	if out.Running, err = s.q.OpsActiveRuns(ctx, listLimit); err != nil {
		return out, err
	}
	out.Failed, err = s.q.OpsFailedRuns(ctx, db.OpsFailedRunsParams{Since: s.now().Add(-24 * time.Hour), Lim: listLimit})
	return out, err
}

// Alerts is the alert section.
type Alerts struct {
	FailedJobs int
	Accounts   []db.OpsAccountProblemsRow
	Numbers    []db.OpsLowQualityNumbersRow
	Events     []db.OpsAccountEventsRow
	Budget     *ai.Budget
}

// AlertSection reads failed jobs, account and number problems, Meta's
// recent account alerts and the AI budget (when one is set).
func (s *Service) AlertSection(ctx context.Context) (Alerts, error) {
	var out Alerts
	var err error
	if s.deps.FailedJobs != nil {
		if out.FailedJobs, err = s.deps.FailedJobs(ctx); err != nil {
			return out, err
		}
	}
	if out.Accounts, err = s.q.OpsAccountProblems(ctx); err != nil {
		return out, err
	}
	if out.Numbers, err = s.q.OpsLowQualityNumbers(ctx); err != nil {
		return out, err
	}
	if out.Events, err = s.q.OpsAccountEvents(ctx, db.OpsAccountEventsParams{Since: s.now().Add(-7 * 24 * time.Hour), Lim: eventLimit}); err != nil {
		return out, err
	}
	if s.deps.Budget != nil {
		b, err := s.deps.Budget(ctx)
		if err != nil {
			return out, err
		}
		if b.Limit != "" {
			out.Budget = &b
		}
	}
	return out, nil
}
