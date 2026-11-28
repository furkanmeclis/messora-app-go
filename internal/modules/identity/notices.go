package identity

import (
	"context"
	"strings"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// WebhookDisabledNotice describes an outgoing webhook endpoint Messora
// switched off.
type WebhookDisabledNotice struct {
	URL, Description, Reason string
	// Failures is the run of failed attempts (0 for 410 Gone).
	Failures     int64
	FailingSince *time.Time
	DisabledAt   *time.Time
	LastError    string
}

// NotifyWebhookDisabled e-mails the context tenant's active owners and
// admins (branded webhook_disabled template, each in their language) that
// an endpoint was switched off. Best effort: failures are logged.
func (s *Service) NotifyWebhookDisabled(ctx context.Context, n WebhookDisabledNotice) int {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return 0
	}
	w, err := s.workspaceOf(ctx, id)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "load workspace for e-mail", "tenant_id", id, "err", err)
		return 0
	}
	members, err := s.q.ListUsersWithRole(ctx)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "list admins for a webhook notice", "err", err)
		return 0
	}
	at := time.Now()
	if n.DisabledAt != nil {
		at = *n.DisabledAt
	}
	sent := 0
	for _, m := range members {
		if m.Status != "active" || (m.RoleKey != "owner" && m.RoleKey != "admin") {
			continue
		}
		loc := userLocation(m.Timezone)
		d := templates.WebhookDisabledData{Common: w.common(m.Name), EndpointURL: n.URL, Description: n.Description,
			Failures: n.Failures, DisabledAt: at.In(loc), LastError: n.LastError, ManageURL: w.link("/settings/webhooks")}
		if n.FailingSince != nil {
			d.FailingSince = n.FailingSince.In(loc)
		}
		if s.sendTo(ctx, w, m.Email, m.Locale, templates.WebhookDisabled, d) {
			sent++
		}
	}
	return sent
}

// userLocation is the person's time zone, UTC when unknown.
func userLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	if l, err := time.LoadLocation(name); err == nil {
		return l
	}
	return time.UTC
}

// PlanChangeNotice is a workspace's plan change request for the platform
// admins (Phase 17 WP8).
type PlanChangeNotice struct {
	// CurrentPlan and RequestedPlan name the plans in Turkish and English.
	CurrentPlan, RequestedPlan [2]string
	BillingCycle               string
	RequestedBy                string
	Message                    string
	RequestedAt                time.Time
}

// NotifyPlanChangeRequest e-mails every active platform admin, in their
// language, about ctx's workspace's plan change request. It reports how
// many e-mails went out (best effort, like every e-mail).
func (s *Service) NotifyPlanChangeRequest(ctx context.Context, n PlanChangeNotice) int {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return 0
	}
	w, err := s.workspaceOf(ctx, id)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "load workspace for e-mail", "tenant_id", id, "err", err)
		return 0
	}
	inbox := s.o.Hosts.AdminURL()
	if inbox == "" {
		return 0
	}
	inbox += "/plan-requests"
	admins, err := s.q.ListPlatformAdmins(tenant.System(ctx))
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "list platform admins for a plan request", "err", err)
		return 0
	}
	// Platform mail: the platform sender, never the company's own server.
	pctx := tenant.System(context.WithoutCancel(ctx))
	if !s.mail.Enabled(pctx) {
		return 0
	}
	sent := 0
	for _, a := range admins {
		tr := templates.Locale(a.Locale) == "tr"
		pick := func(v [2]string) string {
			if tr && v[0] != "" {
				return v[0]
			}
			return v[1]
		}
		d := templates.PlanChangeRequestData{Common: templates.Common{Name: firstName(a.Name)}, Workspace: w.Name, WorkspaceURL: w.link("/"),
			CurrentPlan: pick(n.CurrentPlan), RequestedPlan: pick(n.RequestedPlan), BillingCycle: n.BillingCycle,
			RequestedBy: n.RequestedBy, Message: n.Message, RequestedAt: n.RequestedAt.In(userLocation("")), InboxURL: inbox}
		msg, err := s.tpl.Message(a.Email, templates.PlanChangeRequest, a.Locale, d)
		if err != nil {
			s.o.Logger.ErrorContext(ctx, "render e-mail", "kind", templates.PlanChangeRequest, "err", err)
			continue
		}
		if err := s.mail.Send(pctx, msg); err != nil {
			s.o.Logger.ErrorContext(ctx, "send e-mail", "kind", templates.PlanChangeRequest, "err", err)
			continue
		}
		sent++
	}
	return sent
}

// BillingNotice is a billing e-mail of the context's workspace (WP9): it
// goes to the active owners and to the billing contact address of the
// invoice details (in the workspace's default language when it is not an
// owner's address).
type BillingNotice struct {
	Kind templates.Kind
	// Contact is the billing e-mail address ("" for none).
	Contact string
	// Build makes one recipient's data from the branding block, a builder
	// of absolute links into the workspace and the recipient's time zone.
	Build func(c templates.Common, link func(path string) string, loc *time.Location) templates.Data
}

// NotifyBilling sends a billing notice; it reports how many e-mails went
// out (best effort: failures are logged).
func (s *Service) NotifyBilling(ctx context.Context, n BillingNotice) int {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return 0
	}
	w, err := s.workspaceOf(ctx, id)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "load workspace for e-mail", "tenant_id", id, "err", err)
		return 0
	}
	members, err := s.q.ListUsersWithRole(ctx)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "list owners for a billing notice", "err", err)
		return 0
	}
	link := func(path string) string { return w.link(path) }
	sent := 0
	seen := map[string]bool{}
	for _, m := range members {
		if m.Status != "active" || m.RoleKey != "owner" {
			continue
		}
		seen[strings.ToLower(m.Email)] = true
		if s.sendTo(ctx, w, m.Email, m.Locale, n.Kind, n.Build(w.common(m.Name), link, userLocation(m.Timezone))) {
			sent++
		}
	}
	if c := strings.TrimSpace(n.Contact); c != "" && !seen[strings.ToLower(c)] {
		locale, zone := "tr", ""
		if st, err := s.settings.Get(ctx); err == nil {
			locale, zone = st.DefaultLocale, st.DefaultTimezone
		}
		if s.sendTo(ctx, w, c, locale, n.Kind, n.Build(w.common(""), link, userLocation(zone))) {
			sent++
		}
	}
	return sent
}
