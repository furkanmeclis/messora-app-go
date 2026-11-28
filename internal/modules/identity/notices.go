package identity

import (
	"context"
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
