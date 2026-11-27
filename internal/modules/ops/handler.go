package ops

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/conversations"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves GET /ops/overview.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// GetOpsOverview implements api.ServerInterface.
func (h *Handler) GetOpsOverview(w http.ResponseWriter, r *http.Request, params api.GetOpsOverviewParams) {
	p := principal.From(r.Context())
	loc := time.UTC
	if params.Tz != nil && *params.Tz != "" {
		l, err := time.LoadLocation(*params.Tz)
		if err != nil || strings.Contains(*params.Tz, "..") {
			httpx.WriteError(w, r, apperr.Validation("tz", "Unknown time zone."), h.log)
			return
		}
		loc = l
	}
	var requested []string
	if params.Sections != nil {
		for _, s := range *params.Sections {
			requested = append(requested, string(s))
		}
	}
	out := api.OpsOverview{GeneratedAt: h.svc.now().UTC()}
	ctx := r.Context()
	g, gctx := errgroup.WithContext(ctx)
	for _, section := range Resolve(p, requested) {
		switch section {
		case SectionCampaigns:
			g.Go(func() error {
				c, err := h.svc.CampaignSection(gctx, StartOfDay(h.svc.now(), loc))
				if err == nil {
					out.Campaigns = campaignsToAPI(c)
				}
				return err
			})
		case SectionConversations:
			g.Go(func() error {
				c, err := h.svc.ConversationSection(gctx, p)
				if err == nil {
					out.Conversations = conversationsToAPI(c)
				}
				return err
			})
		case SectionActivity:
			g.Go(func() error {
				hours, err := h.svc.ActivitySection(gctx)
				if err == nil {
					a := &api.OpsActivity{Hours: make([]api.OpsActivityHour, len(hours))}
					for i, x := range hours {
						a.Hours[i] = api.OpsActivityHour{Hour: x.Hour.UTC(), Outbound: x.Outbound, Inbound: x.Inbound}
					}
					out.Activity = a
				}
				return err
			})
		case SectionAutomations:
			g.Go(func() error {
				a, err := h.svc.AutomationSection(gctx)
				if err == nil {
					out.Automations = automationsToAPI(p, a)
				}
				return err
			})
		case SectionAlerts:
			g.Go(func() error {
				a, err := h.svc.AlertSection(gctx)
				if err != nil {
					return err
				}
				out.Alerts = alertsToAPI(a)
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

func campaignsToAPI(c Campaigns) *api.OpsCampaigns {
	k := c.Counts
	out := &api.OpsCampaigns{
		Counts: api.OpsCampaignCounts{Running: k.Running, Preparing: k.Preparing, Scheduled: k.Scheduled, Paused: k.Paused,
			PausedLimit: k.PausedLimit, PausedSpam: k.PausedSpam, PausedOffer: k.PausedOffer, CompletedToday: k.CompletedToday,
			FailedToday: k.FailedToday},
		Active: make([]api.OpsCampaign, len(c.Active)),
		Queue:  campaigns.QueueToAPI(c.Queue),
	}
	for i, a := range c.Active {
		n := a.Counts
		out.Active[i] = api.OpsCampaign{Id: a.ID, Name: a.Name, Status: api.CampaignStatus(a.Status), Priority: api.CampaignPriority(a.Priority),
			PhoneNumberId: a.PhoneNumberID, DisplayPhoneNumber: a.DisplayPhoneNumber, TotalRecipients: a.TotalRecipients, Counts: n,
			MaxMps: a.MaxMps, SendRate: a.SendRate, EtaSeconds: campaigns.ETA(a.Status, a.SendRate, n), ScheduledAt: a.ScheduledAt,
			StartedAt: a.StartedAt, PausedReason: a.PausedReason, ResumeAt: a.ResumeAt, LastError: a.LastError,
			CostMicros: n["a_cost_micros"], Currency: a.Currency, SpamErrors: n["a_err_131048"] + n["a_err_131049"] + n["a_err_131050"]}
	}
	return out
}

func conversationsToAPI(c Conversations) *api.OpsConversations {
	out := &api.OpsConversations{AiActive: c.AIActive,
		AwaitingHuman:  make([]api.OpsEscalation, len(c.AwaitingHuman)),
		Unassigned:     make([]api.OpsWaitingConversation, len(c.Unassigned)),
		WaitingReply:   make([]api.OpsWaitingConversation, len(c.WaitingReply)),
		RecentInbound:  messagesToAPI(c.RecentInbound),
		RecentOutbound: messagesToAPI(c.RecentOutbound)}
	for i, e := range c.AwaitingHuman {
		out.AwaitingHuman[i] = api.OpsEscalation{ConversationId: e.ID, HandoffId: e.HandoffID, ContactId: e.ContactID, ContactName: e.ContactName,
			PhoneE164: e.PhoneE164, Reason: e.Reason, ReasonDetail: e.ReasonDetail, EscalatedAt: e.EscalatedAt.UTC(), AssigneeId: e.AssigneeID}
		out.AwaitingHumanTotal = e.Total
	}
	for i, u := range c.Unassigned {
		out.Unassigned[i] = api.OpsWaitingConversation{ConversationId: u.ID, ContactId: u.ContactID, ContactName: u.ContactName,
			PhoneE164: u.PhoneE164, WaitingSince: at(u.LastInboundAt), UnreadCount: int(u.UnreadCount),
			OwnerType: api.OpsWaitingConversationOwnerType(u.OwnerType)}
		out.UnassignedTotal = u.Total
	}
	for i, u := range c.WaitingReply {
		out.WaitingReply[i] = api.OpsWaitingConversation{ConversationId: u.ID, ContactId: u.ContactID, ContactName: u.ContactName,
			PhoneE164: u.PhoneE164, WaitingSince: at(u.LastInboundAt), UnreadCount: int(u.UnreadCount),
			OwnerType: api.OpsWaitingConversationOwnerType(u.OwnerType), AssigneeId: u.AssigneeID, AssigneeName: u.AssigneeName}
		out.WaitingReplyTotal = u.Total
	}
	return out
}

func at(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

func messagesToAPI(rows []db.OpsRecentMessagesRow) []api.OpsMessage {
	out := make([]api.OpsMessage, len(rows))
	for i, m := range rows {
		out[i] = api.OpsMessage{Id: m.ID, ConversationId: m.ConversationID, ContactId: m.ContactID, ContactName: m.ContactName,
			PhoneE164: m.PhoneE164, Direction: api.OpsMessageDirection(m.Direction), Type: m.Type, Status: m.Status, Source: m.Source,
			Preview: m.Preview, ErrorCode: m.ErrorCode, CreatedAt: m.CreatedAt.UTC()}
	}
	return out
}

// automationsToAPI links a run's conversation only when p may open it.
func automationsToAPI(p *principal.Principal, a Automations) *api.OpsAutomations {
	conv := func(id, assignee *uuid.UUID) *uuid.UUID {
		if id == nil || !conversations.Visible(p, assignee) {
			return nil
		}
		return id
	}
	out := &api.OpsAutomations{Running: make([]api.OpsAutomationRun, len(a.Running)), Failed: make([]api.OpsAutomationRun, len(a.Failed))}
	for i, r := range a.Running {
		steps, by := int(r.StepsExecuted), r.StartedBy
		out.Running[i] = api.OpsAutomationRun{Id: r.ID, AutomationId: r.AutomationID, AutomationName: r.AutomationName,
			Status: api.OpsAutomationRunStatus(r.Status), StepsDone: &steps, StartedBy: &by, At: r.CreatedAt.UTC(),
			ConversationId: conv(r.ConversationID, r.ConversationAssigneeID)}
		out.RunningTotal = r.Total
	}
	for i, r := range a.Failed {
		e, attempt := r.Error, int(r.Attempt)
		out.Failed[i] = api.OpsAutomationRun{Id: r.ID, AutomationId: r.AutomationID, AutomationName: r.AutomationName,
			Status: api.OpsAutomationRunStatus("failed"), Error: &e, ErrorCode: r.ErrorCode, Attempt: &attempt, At: at(r.FinishedAt),
			ConversationId: conv(r.ConversationID, r.ConversationAssigneeID)}
		out.FailedTotal = r.Total
	}
	return out
}

func alertsToAPI(a Alerts) *api.OpsAlerts {
	out := &api.OpsAlerts{FailedJobs: a.FailedJobs,
		Accounts: make([]api.OpsAccountAlert, len(a.Accounts)), Numbers: make([]api.OpsNumberAlert, len(a.Numbers)),
		Events: make([]api.OpsAccountEvent, len(a.Events))}
	for i, x := range a.Accounts {
		out.Accounts[i] = api.OpsAccountAlert{AccountId: x.ID, Name: x.Name, Status: api.OpsAccountAlertStatus(x.Status), LastError: x.LastError,
			TokenWarning: x.TokenWarning, TokenExpiresAt: x.TokenExpiresAt}
	}
	for i, x := range a.Numbers {
		out.Numbers[i] = api.OpsNumberAlert{PhoneNumberId: x.ID, AccountId: x.AccountID, DisplayPhoneNumber: x.DisplayPhoneNumber,
			VerifiedName: x.VerifiedName, QualityRating: api.OpsNumberAlertQualityRating(x.QualityRating)}
	}
	for i, x := range a.Events {
		out.Events[i] = api.OpsAccountEvent{Id: x.ID, AccountId: x.AccountID, AccountName: x.AccountName, PhoneNumberId: x.PhoneNumberID,
			DisplayPhoneNumber: x.DisplayPhoneNumber, Kind: x.Kind, OccurredAt: x.OccurredAt.UTC()}
	}
	if b := a.Budget; b != nil {
		limit := b.Limit
		out.AiBudget = &api.AIBudget{Limit: &limit, Spent: b.Spent, MonthStart: b.MonthStart.UTC(), Exceeded: b.Exceeded}
	}
	return out
}
