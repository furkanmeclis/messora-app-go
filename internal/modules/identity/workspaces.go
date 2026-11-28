package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// ListMemberships returns the workspaces the signed-in person can enter
// (active memberships of active workspaces), the current one marked.
func (s *Service) ListMemberships(ctx context.Context, p *principal.Principal) ([]api.Membership, error) {
	if p.Support != nil {
		return []api.Membership{}, nil
	}
	var rows []db.ListMembershipsOfUserRow
	if err := s.platformTx(ctx, func(q *db.Queries) error {
		var err error
		rows, err = q.ListMembershipsOfUser(tenant.Platform(ctx), p.ID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]api.Membership, 0, len(rows))
	for _, r := range rows {
		if r.Status != "active" || r.TenantStatus != tenant.StatusActive || r.Host == "" {
			continue
		}
		m := api.Membership{Current: r.TenantID == p.TenantID, Url: s.o.Hosts.URL(r.Host)}
		m.Role.Key, m.Role.Name = r.RoleKey, r.RoleName
		m.Tenant.Id, m.Tenant.Slug, m.Tenant.Name = r.TenantID, r.Slug, r.TenantName
		if w, err := s.workspaceOf(ctx, r.TenantID); err == nil {
			m.Tenant.Name = w.Name
			if w.LogoURL != "" {
				logo := w.LogoURL
				if m.Current {
					logo = strings.TrimPrefix(logo, w.URL) // same origin
				}
				m.Tenant.LogoUrl = &logo
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// handoff is a pending hand-over to another workspace (Redis, single use).
type handoff struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	// GrantID marks a support session of a platform admin.
	GrantID  *uuid.UUID `json:"grant_id,omitempty"`
	ReadOnly bool       `json:"read_only,omitempty"`
}

// handoffKey is platform-global: the token is issued at one workspace and
// redeemed at another (its payload names the target).
func handoffKey(token string) string {
	return tenant.GlobalRedisKey(fmt.Sprintf("handoff:%x", crypto.HashToken(token)))
}

// newHandoff stores h and returns the link that completes it at the
// workspace's address.
func (s *Service) newHandoff(ctx context.Context, h handoff) (string, error) {
	token, err := crypto.NewToken(handoffTokenPrefix)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(h)
	if err := s.rdb.Set(ctx, handoffKey(token), b, handoffTTL).Err(); err != nil {
		return "", err
	}
	w, err := s.workspaceOf(ctx, h.TenantID)
	if err != nil {
		return "", err
	}
	return w.link("/auth/handoff", "token", token), nil
}

// SwitchWorkspace hands the person over to another workspace they belong
// to: sessions stay bound to one workspace, so the target signs them in
// with a one-time token.
func (s *Service) SwitchWorkspace(ctx context.Context, p *principal.Principal, target uuid.UUID) (string, error) {
	if p.Support != nil {
		return "", apperr.Forbidden("FORBIDDEN", "Support sessions cannot switch workspaces")
	}
	var m db.GetMembershipInRow
	err := s.platformTx(ctx, func(q *db.Queries) error {
		var err error
		m, err = q.GetMembershipIn(tenant.Platform(ctx), db.GetMembershipInParams{TenantID: target, UserID: p.ID})
		return err
	})
	switch {
	case isNoRows(err):
		return "", apperr.Forbidden("FORBIDDEN", "You are not a member of this workspace")
	case err != nil:
		return "", err
	case m.Status != "active":
		return "", apperr.Forbidden("ACCOUNT_DISABLED", "Your access to this workspace was disabled")
	case m.TenantStatus != tenant.StatusActive:
		return "", apperr.Forbidden("TENANT_SUSPENDED", "This workspace is suspended")
	}
	return s.newHandoff(ctx, handoff{UserID: p.ID, TenantID: target})
}

var errHandoffInvalid = apperr.Unauthorized("HANDOFF_INVALID", "This sign-in link is invalid or has expired. Sign in again.")

// CompleteHandoff exchanges a hand-over token for a session at the
// request's workspace. The token is consumed on first use, whatever the
// outcome, and works only at the workspace it was issued for.
func (s *Service) CompleteHandoff(ctx context.Context, token string) (api.SessionCreated, error) {
	if !strings.HasPrefix(token, handoffTokenPrefix) {
		return api.SessionCreated{}, errHandoffInvalid
	}
	raw, err := s.rdb.GetDel(ctx, handoffKey(token)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return api.SessionCreated{}, errHandoffInvalid
	}
	if err != nil {
		return api.SessionCreated{}, err
	}
	var h handoff
	if err := json.Unmarshal(raw, &h); err != nil || h.TenantID != tenant.ID(ctx) {
		return api.SessionCreated{}, errHandoffInvalid
	}
	u, err := s.q.GetUserByID(ctx, h.UserID)
	if err != nil || u.Status != "active" {
		return api.SessionCreated{}, errHandoffInvalid
	}
	if h.GrantID != nil {
		return s.openSupportSession(ctx, u, *h.GrantID, h.ReadOnly)
	}
	if role, err := s.q.GetUserRole(ctx, u.ID); err != nil || role.MembershipStatus != "active" {
		return api.SessionCreated{}, errHandoffInvalid
	}
	return s.memberSession(ctx, u)
}

// openSupportSession creates the session of a support grant in the
// request's workspace and records it in the workspace's audit log.
func (s *Service) openSupportSession(ctx context.Context, admin db.User, grantID uuid.UUID, readOnly bool) (api.SessionCreated, error) {
	g, err := s.q.GetSupportGrantForSession(ctx, grantID)
	if err != nil || g.EndedAt != nil || !s.now().Before(g.ExpiresAt) {
		return api.SessionCreated{}, errHandoffInvalid
	}
	token, err := crypto.NewToken(SessionTokenPrefix)
	if err != nil {
		return api.SessionCreated{}, err
	}
	ip, ua := requestOrigin(ctx)
	var sess db.UserSession
	err = s.tx(ctx, func(q *db.Queries) error {
		var err error
		sess, err = q.CreateSession(ctx, db.CreateSessionParams{UserID: admin.ID, TokenHash: crypto.HashToken(token), Ip: ip, UserAgent: ua,
			ExpiresAt: g.ExpiresAt, SupportGrantID: &grantID, ReadOnly: readOnly})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "support.session_open", EntityType: "support_grant", EntityID: &grantID,
			Actor:   &principal.Principal{Kind: principal.KindUser, ID: admin.ID, Name: admin.Name},
			Changes: map[string]any{"admin": admin.Email, "reason": g.Reason, "read_only": readOnly, "expires_at": g.ExpiresAt}})
	})
	if err != nil {
		return api.SessionCreated{}, err
	}
	view := toAPIUser(admin, "admin", "Messora Support")
	return api.SessionCreated{SessionToken: token, ExpiresAt: sess.ExpiresAt, User: view}, nil
}

// SupportHandoff returns the link that opens workspace tenantID for the
// support grant (the platform admin panel).
func (s *Service) SupportHandoff(ctx context.Context, adminID, tenantID, grantID uuid.UUID, readOnly bool) (string, error) {
	return s.newHandoff(ctx, handoff{UserID: adminID, TenantID: tenantID, GrantID: &grantID, ReadOnly: readOnly})
}

// ── E-mail address change ──────────────────────────────────────────────────

type emailChange struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Email    string    `json:"email"`
}

// emailChangeKey lives under the workspace the change was asked at: the
// link is confirmed there.
func emailChangeKey(ctx context.Context, token string) string {
	return tenant.RedisKey(ctx, fmt.Sprintf("email-change:%x", crypto.HashToken(token)))
}

// RequestEmailChange e-mails a confirmation link to the new address; the
// address changes once it is followed.
func (s *Service) RequestEmailChange(ctx context.Context, p *principal.Principal, raw string) error {
	if p.Support != nil {
		return errSupportForbidden
	}
	if err := s.requireReauth(p); err != nil {
		return err
	}
	email, err := normalizeEmail("email", raw)
	if err != nil {
		return err
	}
	if err := s.rateLimit(ctx, "email-change:"+p.ID.String(), ratelimit.Limit{Rate: 3, Period: time.Hour, Burst: 3}); err != nil {
		return err
	}
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return err
	}
	if email == u.Email {
		return apperr.Validation("email", "This is already your e-mail address")
	}
	if _, err := s.q.GetUserByEmail(ctx, email); err == nil {
		return errEmailTaken
	} else if !isNoRows(err) {
		return err
	}
	token, err := crypto.NewToken(emailTokenPrefix)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(emailChange{UserID: u.ID, TenantID: p.TenantID, Email: email})
	if err := s.rdb.Set(ctx, emailChangeKey(ctx, token), b, emailChangeTTL).Err(); err != nil {
		return err
	}
	w, err := s.workspaceOf(ctx, p.TenantID)
	if err != nil {
		return err
	}
	s.sendTo(ctx, w, email, u.Locale, templates.EmailChangeConfirm, templates.EmailChangeConfirmData{Common: w.common(u.Name),
		NewEmail: email, ConfirmURL: w.link("/confirm-email", "token", token), ExpiresIn: emailChangeTTL})
	return nil
}

var errEmailChangeInvalid = apperr.Forbidden("EMAIL_CHANGE_INVALID", "This confirmation link is invalid or has expired.")

// ConfirmEmailChange applies a confirmed address change and ends every
// other session of the account.
func (s *Service) ConfirmEmailChange(ctx context.Context, token string) error {
	if !strings.HasPrefix(token, emailTokenPrefix) {
		return errEmailChangeInvalid
	}
	raw, err := s.rdb.GetDel(ctx, emailChangeKey(ctx, token)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return errEmailChangeInvalid
	}
	if err != nil {
		return err
	}
	var c emailChange
	if err := json.Unmarshal(raw, &c); err != nil || c.TenantID != tenant.ID(ctx) {
		return errEmailChangeInvalid
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		before, err := q.GetUserByID(ctx, c.UserID)
		if err != nil {
			return err
		}
		if err := q.UpdateUserEmail(ctx, db.UpdateUserEmailParams{ID: c.UserID, Email: c.Email}); err != nil {
			if strings.Contains(err.Error(), "users_email_key") {
				return errEmailTaken
			}
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.email_change", EntityType: "user", EntityID: &c.UserID,
			Actor: &principal.Principal{Kind: principal.KindUser, ID: c.UserID}, Changes: map[string]any{"email": []string{before.Email, c.Email}}})
	})
	if err != nil {
		return err
	}
	return s.revokeEverywhere(ctx, c.UserID, nil)
}
