package identity

import (
	"context"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

var errInvalidSession = apperr.Unauthorized("SESSION_INVALID", "Your session has ended. Sign in again.")

func requestOrigin(ctx context.Context) (*netip.Addr, *string) {
	m := principal.Meta(ctx)
	var ip *netip.Addr
	if m.IP.IsValid() {
		a := m.IP
		ip = &a
	}
	var ua *string
	if m.UserAgent != "" {
		ua = &m.UserAgent
	}
	return ip, ua
}

// newSignIn is set by createSession when the sign-in came from an address
// the user never used in this workspace; the caller e-mails a notice after
// the transaction commits.
type newSignIn struct {
	user db.User
	at   time.Time
	ip   string
	ua   string
}

// createSession issues a session token for u in the connection's tenant
// within q's transaction. The user must be an active member.
func (s *Service) createSession(ctx context.Context, q *db.Queries, u db.User) (api.SessionCreated, *newSignIn, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return api.SessionCreated{}, nil, err
	}
	token, err := crypto.NewToken(SessionTokenPrefix)
	if err != nil {
		return api.SessionCreated{}, nil, err
	}
	ip, ua := requestOrigin(ctx)
	// A sign-in from an address this user never signed in from here gets
	// an e-mail (not the very first sign-in).
	var alert *newSignIn
	if ip != nil {
		seen, err := q.CountSessionsFrom(ctx, db.CountSessionsFromParams{UserID: u.ID, Ip: ip})
		if err != nil {
			return api.SessionCreated{}, nil, err
		}
		if seen.Total > 0 && seen.FromIp == 0 {
			alert = &newSignIn{user: u, at: s.now(), ip: ip.String()}
			if ua != nil {
				alert.ua = *ua
			}
		}
	}
	expires := s.now().Add(time.Duration(st.SessionLifetimeDays) * 24 * time.Hour)
	sess, err := q.CreateSession(ctx, db.CreateSessionParams{
		UserID: u.ID, TokenHash: crypto.HashToken(token), Ip: ip, UserAgent: ua, ExpiresAt: expires,
	})
	if err != nil {
		return api.SessionCreated{}, nil, err
	}
	if err := q.RecordLoginSuccess(ctx, u.ID); err != nil {
		return api.SessionCreated{}, nil, err
	}
	view, err := s.userView(ctx, q, u)
	if err != nil {
		return api.SessionCreated{}, nil, err
	}
	actor := &principal.Principal{Kind: principal.KindUser, ID: u.ID, Name: u.Name}
	if err := audit.Record(ctx, q, audit.Entry{
		Action: "user.login", EntityType: "user", EntityID: uuidPtr(u.ID), Actor: actor,
		Changes: map[string]any{"session_id": sess.ID},
	}); err != nil {
		return api.SessionCreated{}, nil, err
	}
	return api.SessionCreated{SessionToken: token, ExpiresAt: sess.ExpiresAt, User: view}, alert, nil
}

// alertNewSignIn e-mails the "new sign-in" notice (login_alert).
func (s *Service) alertNewSignIn(ctx context.Context, a *newSignIn) {
	if a == nil {
		return
	}
	id := tenant.ID(ctx)
	s.sendToUser(ctx, id, a.user, templates.LoginAlert, func(w workspace) templates.Data {
		return templates.LoginAlertData{Common: w.common(a.user.Name), At: a.at, IP: a.ip, Device: deviceOf(a.ua),
			SecureURL: w.link("/forgot-password")}
	})
}

// memberSession creates a session in a transaction and sends the new
// sign-in notice afterwards.
func (s *Service) memberSession(ctx context.Context, u db.User) (api.SessionCreated, error) {
	var (
		sess  api.SessionCreated
		alert *newSignIn
	)
	err := s.tx(ctx, func(q *db.Queries) error {
		var err error
		sess, alert, err = s.createSession(ctx, q, u)
		return err
	})
	if err == nil {
		s.alertNewSignIn(ctx, alert)
	}
	return sess, err
}

// AuthenticateSession resolves a session token into a principal of the
// request's tenant. Idle and absolute timeouts are enforced here; disabled
// accounts and memberships are rejected. Support sessions of platform
// admins carry their grant.
func (s *Service) AuthenticateSession(ctx context.Context, token string) (*principal.Principal, error) {
	if !strings.HasPrefix(token, SessionTokenPrefix) {
		return nil, errInvalidSession
	}
	row, err := s.q.GetActiveSessionByTokenHash(ctx, crypto.HashToken(token))
	if isNoRows(err) {
		return nil, errInvalidSession
	}
	if err != nil {
		return nil, err
	}
	if row.UserStatus != "active" {
		return nil, errInvalidSession
	}
	support := row.SupportGrantID != nil
	if !support && (row.MembershipStatus == nil || *row.MembershipStatus != "active") {
		return nil, errInvalidSession
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if now.Sub(row.LastSeenAt) > time.Duration(st.SessionIdleHours)*time.Hour {
		_, _ = s.q.RevokeSession(ctx, db.RevokeSessionParams{ID: row.ID, UserID: row.UserID})
		return nil, errInvalidSession
	}
	if now.Sub(row.LastSeenAt) > sessionTouchPeriod {
		if err := s.q.TouchSession(ctx, row.ID); err != nil {
			return nil, err
		}
	}
	u, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	p := &principal.Principal{
		Kind: principal.KindUser, TenantID: row.TenantID, ID: u.ID, Name: u.Name, Email: u.Email, SessionID: row.ID,
		ReauthAt: row.ReauthAt, ExpiresAt: row.ExpiresAt,
	}
	if support {
		return s.supportPrincipal(ctx, p, *row.SupportGrantID, row.ReadOnly)
	}
	role, err := s.q.GetUserRole(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	perms, err := s.q.UserPermissions(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	slices.Sort(perms)
	p.RoleKey, p.RoleRank, p.Permissions = role.RoleKey, int(role.RoleRank), perms
	p.MFAPending = st.MFARequired(role.RoleKey) && u.TotpConfirmedAt == nil
	return p, nil
}

// supportPrincipal completes the principal of a support session: the
// grant must still be open; read-only sessions get every read permission,
// write sessions those of an admin.
func (s *Service) supportPrincipal(ctx context.Context, p *principal.Principal, grantID uuid.UUID, readOnly bool) (*principal.Principal, error) {
	g, err := s.q.GetSupportGrantForSession(ctx, grantID)
	if isNoRows(err) {
		return nil, errInvalidSession
	}
	if err != nil {
		return nil, err
	}
	if g.EndedAt != nil || !s.now().Before(g.ExpiresAt) {
		return nil, errInvalidSession
	}
	perms, err := s.q.RolePermissionKeys(ctx, "admin")
	if err != nil {
		return nil, err
	}
	if readOnly {
		perms = slices.DeleteFunc(perms, func(k string) bool {
			_, action, _ := strings.Cut(k, ".")
			return action != "read" && action != "read_all"
		})
	}
	slices.Sort(perms)
	p.RoleKey, p.RoleRank, p.Permissions = "admin", roleRank["admin"], perms
	// Support never re-authenticates: sensitive actions stay closed.
	p.ReauthAt = nil
	p.Support = &principal.Support{GrantID: g.ID, ReadOnly: readOnly, Reason: g.Reason, AdminName: g.AdminName, ExpiresAt: g.ExpiresAt}
	return p, nil
}

var errInvalidAPIKey = apperr.Unauthorized("API_KEY_INVALID", "The API key is invalid, expired or revoked")

// AuthenticateAPIKey resolves an API key into a principal whose permissions
// are the key's scopes.
func (s *Service) AuthenticateAPIKey(ctx context.Context, key string) (*principal.Principal, error) {
	if !strings.HasPrefix(key, APIKeyPrefix) || len(key) > 200 {
		return nil, errInvalidAPIKey
	}
	k, err := s.q.GetActiveApiKeyByHash(ctx, crypto.HashToken(key))
	if isNoRows(err) {
		return nil, errInvalidAPIKey
	}
	if err != nil {
		return nil, err
	}
	if err := s.q.TouchApiKey(ctx, k.ID); err != nil {
		return nil, err
	}
	scopes := slices.Clone(k.Scopes)
	slices.Sort(scopes)
	p := &principal.Principal{Kind: principal.KindAPIKey, TenantID: k.TenantID, ID: k.ID, Name: k.Name, Permissions: scopes}
	if k.ExpiresAt != nil {
		p.ExpiresAt = *k.ExpiresAt
	}
	return p, nil
}

// Logout ends the current session.
func (s *Service) Logout(ctx context.Context, p *principal.Principal) error {
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.RevokeSession(ctx, db.RevokeSessionParams{ID: p.SessionID, UserID: p.ID}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.logout", EntityType: "user", EntityID: uuidPtr(p.ID)})
	})
}

// ListSessions returns the user's active sessions in this workspace.
func (s *Service) ListSessions(ctx context.Context, p *principal.Principal) ([]api.SessionInfo, error) {
	rows, err := s.q.ListUserSessions(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]api.SessionInfo, 0, len(rows))
	for _, r := range rows {
		if r.SupportGrantID != nil && r.ID != p.SessionID {
			continue // a platform admin's support sessions are not the member's
		}
		var ip *string
		if r.Ip != nil {
			v := r.Ip.String()
			ip = &v
		}
		out = append(out, api.SessionInfo{
			Id: r.ID, Ip: ip, UserAgent: r.UserAgent, CreatedAt: r.CreatedAt,
			LastSeenAt: r.LastSeenAt, ExpiresAt: r.ExpiresAt, Current: r.ID == p.SessionID,
		})
	}
	return out, nil
}

// RevokeSession ends one of the user's sessions.
func (s *Service) RevokeSession(ctx context.Context, p *principal.Principal, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		n, err := q.RevokeSession(ctx, db.RevokeSessionParams{ID: id, UserID: p.ID})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound("Session not found")
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.session_revoke", EntityType: "user", EntityID: uuidPtr(p.ID), Changes: map[string]any{"session_id": id}})
	})
}

// RevokeOtherSessions ends every session of the user in this workspace but
// the current one.
func (s *Service) RevokeOtherSessions(ctx context.Context, p *principal.Principal) error {
	return s.tx(ctx, func(q *db.Queries) error {
		n, err := q.RevokeUserSessions(ctx, db.RevokeUserSessionsParams{UserID: p.ID, ExceptID: uuidPtr(p.SessionID)})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.session_revoke_others", EntityType: "user", EntityID: uuidPtr(p.ID), Changes: map[string]any{"revoked": n}})
	})
}

// revokeEverywhere ends the user's sessions in every workspace (and the
// admin panel), except one: after a password change or reset, or when an
// account is disabled.
func (s *Service) revokeEverywhere(ctx context.Context, userID uuid.UUID, except *uuid.UUID) error {
	return s.platformTx(ctx, func(q *db.Queries) error {
		if _, err := q.RevokeUserSessionsEverywhere(ctx, db.RevokeUserSessionsEverywhereParams{UserID: userID, ExceptID: except}); err != nil {
			return err
		}
		_, err := q.RevokePlatformSessionsOf(ctx, db.RevokePlatformSessionsOfParams{UserID: userID, ExceptID: except})
		return err
	})
}

// RevokeEverywhere is revokeEverywhere for the platform admin panel.
func (s *Service) RevokeEverywhere(ctx context.Context, userID uuid.UUID) error {
	return s.revokeEverywhere(ctx, userID, nil)
}

// requireReauth fails unless the user confirmed their identity recently.
func (s *Service) requireReauth(p *principal.Principal) error {
	if p.Kind != principal.KindUser || !p.ReauthValid(ReauthWindow, s.now()) {
		return apperr.ErrReauthRequired
	}
	return nil
}
