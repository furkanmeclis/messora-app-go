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
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

var errInvalidSession = apperr.Unauthorized("SESSION_INVALID", "Your session has ended. Sign in again.")

// createSession issues a session token for u within q's transaction.
func (s *Service) createSession(ctx context.Context, q *db.Queries, u db.User) (api.SessionCreated, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return api.SessionCreated{}, err
	}
	token, err := crypto.NewToken(SessionTokenPrefix)
	if err != nil {
		return api.SessionCreated{}, err
	}
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
	expires := s.now().Add(time.Duration(st.SessionLifetimeDays) * 24 * time.Hour)
	sess, err := q.CreateSession(ctx, db.CreateSessionParams{
		UserID: u.ID, TokenHash: crypto.HashToken(token), Ip: ip, UserAgent: ua, ExpiresAt: expires,
	})
	if err != nil {
		return api.SessionCreated{}, err
	}
	if err := q.RecordLoginSuccess(ctx, u.ID); err != nil {
		return api.SessionCreated{}, err
	}
	view, err := s.userView(ctx, q, u)
	if err != nil {
		return api.SessionCreated{}, err
	}
	actor := &principal.Principal{Kind: principal.KindUser, ID: u.ID, Name: u.Name}
	if err := audit.Record(ctx, q, audit.Entry{
		Action: "user.login", EntityType: "user", EntityID: uuidPtr(u.ID), Actor: actor,
		Changes: map[string]any{"session_id": sess.ID},
	}); err != nil {
		return api.SessionCreated{}, err
	}
	return api.SessionCreated{SessionToken: token, ExpiresAt: sess.ExpiresAt, User: view}, nil
}

// AuthenticateSession resolves a session token into a principal. Idle and
// absolute timeouts are enforced here; disabled users are rejected.
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
	role, err := s.q.GetUserRole(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	perms, err := s.q.UserPermissions(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	slices.Sort(perms)
	return &principal.Principal{
		Kind: principal.KindUser, TenantID: row.TenantID, ID: u.ID, Name: u.Name, SessionID: row.ID,
		RoleKey: role.RoleKey, RoleRank: int(role.RoleRank), Permissions: perms,
		ReauthAt: row.ReauthAt, ExpiresAt: row.ExpiresAt,
		MFAPending: st.MFARequired(role.RoleKey) && u.TotpConfirmedAt == nil,
	}, nil
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

// ListSessions returns the user's active sessions.
func (s *Service) ListSessions(ctx context.Context, p *principal.Principal) ([]api.SessionInfo, error) {
	rows, err := s.q.ListUserSessions(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]api.SessionInfo, 0, len(rows))
	for _, r := range rows {
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

// RevokeOtherSessions ends every session but the current one.
func (s *Service) RevokeOtherSessions(ctx context.Context, p *principal.Principal) error {
	return s.tx(ctx, func(q *db.Queries) error {
		n, err := q.RevokeUserSessions(ctx, db.RevokeUserSessionsParams{UserID: p.ID, ExceptID: uuidPtr(p.SessionID)})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.session_revoke_others", EntityType: "user", EntityID: uuidPtr(p.ID), Changes: map[string]any{"revoked": n}})
	})
}

// requireReauth fails unless the user confirmed their identity recently.
func (s *Service) requireReauth(p *principal.Principal) error {
	if p.Kind != principal.KindUser || !p.ReauthValid(ReauthWindow, s.now()) {
		return apperr.ErrReauthRequired
	}
	return nil
}
