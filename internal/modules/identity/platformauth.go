package identity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Platform admins are global accounts with is_platform_admin. They sign in
// on the admin host only, with sessions of their own (platform_sessions,
// token prefix msp_), which no workspace accepts and which accept no
// workspace session. Two-factor authentication is mandatory: until it is
// set up a platform session can only enrol.

var errInvalidPlatformSession = apperr.Unauthorized("SESSION_INVALID", "Your session has ended. Sign in again.")

func platformAdminView(u db.User) api.PlatformAdmin {
	return api.PlatformAdmin{Id: u.ID, Email: u.Email, Name: u.Name, MfaEnabled: u.TotpConfirmedAt != nil}
}

// createPlatformSession issues a platform session for u (platform role).
func (s *Service) createPlatformSession(ctx context.Context, q *db.Queries, u db.User) (api.PlatformSessionCreated, error) {
	token, err := crypto.NewToken(PlatformTokenPrefix)
	if err != nil {
		return api.PlatformSessionCreated{}, err
	}
	ip, ua := requestOrigin(ctx)
	sess, err := q.CreatePlatformSession(ctx, db.CreatePlatformSessionParams{UserID: u.ID, TokenHash: crypto.HashToken(token),
		Ip: ip, UserAgent: ua, ExpiresAt: s.now().Add(platformSessionTTL)})
	if err != nil {
		return api.PlatformSessionCreated{}, err
	}
	if err := q.RecordLoginSuccess(ctx, u.ID); err != nil {
		return api.PlatformSessionCreated{}, err
	}
	if err := audit.Record(ctx, q, audit.Entry{Action: "platform.login", EntityType: "user", EntityID: &u.ID,
		Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID, Email: u.Email}, Changes: map[string]any{"session_id": sess.ID}}); err != nil {
		return api.PlatformSessionCreated{}, err
	}
	return api.PlatformSessionCreated{SessionToken: token, ExpiresAt: sess.ExpiresAt, Admin: platformAdminView(u)}, nil
}

// PlatformLogin signs a platform admin in to the admin panel.
func (s *Service) PlatformLogin(ctx context.Context, req api.LoginRequest) (api.PlatformLoginResult, error) {
	ctx = tenant.Platform(ctx)
	u, err := s.checkPassword(ctx, string(req.Email), req.Password)
	if err != nil {
		return api.PlatformLoginResult{}, err
	}
	if !u.IsPlatformAdmin {
		return api.PlatformLoginResult{}, errInvalidCredentials
	}
	if u.PasswordResetRequired {
		return api.PlatformLoginResult{}, errResetRequired
	}
	if u.TotpConfirmedAt != nil {
		token, err := s.newMFAChallenge(ctx, u.ID, "platform")
		if err != nil {
			return api.PlatformLoginResult{}, err
		}
		return api.PlatformLoginResult{Status: api.PlatformLoginResultStatusMfaRequired, MfaToken: &token}, nil
	}
	// No second factor yet: the session can only enrol one.
	var sess api.PlatformSessionCreated
	err = s.tx(ctx, func(q *db.Queries) error {
		sess, err = s.createPlatformSession(ctx, q, u)
		return err
	})
	if err != nil {
		return api.PlatformLoginResult{}, err
	}
	return api.PlatformLoginResult{Status: api.PlatformLoginResultStatusAuthenticated, Session: &sess}, nil
}

// PlatformLoginMFA completes a platform sign-in with the second factor.
func (s *Service) PlatformLoginMFA(ctx context.Context, req api.LoginMfaRequest) (api.PlatformSessionCreated, error) {
	ctx = tenant.Platform(ctx)
	u, err := s.passMFA(ctx, "platform", req)
	if err != nil {
		return api.PlatformSessionCreated{}, err
	}
	if !u.IsPlatformAdmin || u.Status != "active" {
		return api.PlatformSessionCreated{}, errInvalidCredentials
	}
	var sess api.PlatformSessionCreated
	err = s.tx(ctx, func(q *db.Queries) error {
		sess, err = s.createPlatformSession(ctx, q, u)
		return err
	})
	return sess, err
}

// AuthenticatePlatformSession resolves a platform session token. Only
// active platform admins pass; idle (2 h) and absolute (12 h) limits apply.
func (s *Service) AuthenticatePlatformSession(ctx context.Context, token string) (*principal.Principal, error) {
	if !strings.HasPrefix(token, PlatformTokenPrefix) {
		return nil, errInvalidPlatformSession
	}
	ctx = tenant.Platform(ctx)
	row, err := s.q.GetActivePlatformSession(ctx, crypto.HashToken(token))
	if isNoRows(err) {
		return nil, errInvalidPlatformSession
	}
	if err != nil {
		return nil, err
	}
	if row.UserStatus != "active" || !row.IsPlatformAdmin {
		return nil, errInvalidPlatformSession
	}
	now := s.now()
	if now.Sub(row.LastSeenAt) > platformSessionIdle {
		_, _ = s.q.RevokePlatformSession(ctx, row.ID)
		return nil, errInvalidPlatformSession
	}
	if now.Sub(row.LastSeenAt) > sessionTouchPeriod {
		if err := s.q.TouchPlatformSession(ctx, row.ID); err != nil {
			return nil, err
		}
	}
	return &principal.Principal{
		Kind: principal.KindUser, Platform: true, ID: row.UserID, Name: row.UserName, Email: row.Email,
		SessionID: row.ID, ReauthAt: row.ReauthAt, ExpiresAt: row.ExpiresAt,
		MFAPending: row.TotpConfirmedAt == nil,
	}, nil
}

// PlatformLogout ends the platform session.
func (s *Service) PlatformLogout(ctx context.Context, p *principal.Principal) error {
	ctx = tenant.Platform(ctx)
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.RevokePlatformSession(ctx, p.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.logout", EntityType: "user", EntityID: &p.ID})
	})
}

// PlatformMe describes the signed-in platform admin.
func (s *Service) PlatformMe(ctx context.Context, p *principal.Principal) (api.PlatformMe, error) {
	u, err := s.q.GetUserByID(tenant.Platform(ctx), p.ID)
	if err != nil {
		return api.PlatformMe{}, err
	}
	me := api.PlatformMe{Admin: platformAdminView(u), MfaRequired: u.TotpConfirmedAt == nil}
	me.Session.Id, me.Session.ExpiresAt = p.SessionID, p.ExpiresAt
	return me, nil
}

// ── Bootstrap: the first platform admin ────────────────────────────────────

// settingBootstrap holds the hash of the one-time token that creates the
// first platform admin (platform_settings).
const settingBootstrap = "platform.bootstrap"

type bootstrapToken struct {
	Hash      []byte    `json:"hash"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IssueBootstrapToken stores a new one-time token for the first platform
// admin while there is none (replacing an earlier token) and returns it;
// "" when a platform admin exists. q must be able to write
// platform_settings (the owner, during `messora migrate`).
func IssueBootstrapToken(ctx context.Context, q *db.Queries, now time.Time) (string, error) {
	n, err := q.CountPlatformAdmins(ctx)
	if err != nil || n > 0 {
		if n > 0 {
			_ = q.DeletePlatformSetting(ctx, settingBootstrap)
		}
		return "", err
	}
	token, err := crypto.NewToken(bootstrapPrefix)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(bootstrapToken{Hash: crypto.HashToken(token), ExpiresAt: now.Add(bootstrapTokenTTL)})
	if err := q.UpsertPlatformSetting(ctx, db.UpsertPlatformSettingParams{Key: settingBootstrap, Value: b}); err != nil {
		return "", err
	}
	return token, nil
}

// BootstrapURL is the admin panel link that redeems token.
func (s *Service) BootstrapURL(token string) string {
	return s.o.Hosts.AdminURL() + "/bootstrap?token=" + token
}

// BootstrapRequired reports whether no platform admin exists yet.
func (s *Service) BootstrapRequired(ctx context.Context) (bool, error) {
	n, err := s.q.CountPlatformAdmins(tenant.Platform(ctx))
	return n == 0, err
}

var errBootstrapInvalid = apperr.Forbidden("BOOTSTRAP_TOKEN_INVALID", "This link is invalid or has expired. Run `messora migrate` (or `messora admin create`) for a new one.")

// Bootstrap creates the first platform admin with the token printed by
// `messora migrate` and signs them in (two-factor enrolment follows). An
// existing account with the address is promoted when its password matches.
func (s *Service) Bootstrap(ctx context.Context, req api.PlatformBootstrapRequest) (api.PlatformSessionCreated, error) {
	ctx = tenant.Platform(ctx)
	email, err := normalizeEmail("email", string(req.Email))
	if err != nil {
		return api.PlatformSessionCreated{}, err
	}
	if err := validatePassword("password", req.Password, email); err != nil {
		return api.PlatformSessionCreated{}, err
	}
	hash, err := crypto.HashPassword(req.Password, s.o.Argon2)
	if err != nil {
		return api.PlatformSessionCreated{}, err
	}
	var out api.PlatformSessionCreated
	err = s.tx(ctx, func(q *db.Queries) error {
		if n, err := q.CountPlatformAdmins(ctx); err != nil || n > 0 {
			if err == nil {
				err = apperr.Conflict("BOOTSTRAP_COMPLETE", "The platform already has an admin")
			}
			return err
		}
		raw, err := q.GetPlatformSetting(ctx, settingBootstrap)
		if isNoRows(err) {
			return errBootstrapInvalid
		}
		if err != nil {
			return err
		}
		var t bootstrapToken
		if err := json.Unmarshal(raw, &t); err != nil || !s.now().Before(t.ExpiresAt) || !constantTimeEqual(t.Hash, crypto.HashToken(req.Token)) {
			return errBootstrapInvalid
		}
		if err := q.DeletePlatformSetting(ctx, settingBootstrap); err != nil {
			return err
		}
		u, err := q.GetUserByEmail(ctx, email)
		switch {
		case isNoRows(err):
			now := s.now()
			if u, err = q.CreateUser(ctx, db.CreateUserParams{Email: email, Name: strings.TrimSpace(req.Name), PasswordHash: hash,
				Locale: "en", Timezone: "UTC", EmailVerifiedAt: &now}); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if _, err := crypto.VerifyPassword(req.Password, u.PasswordHash, s.o.Argon2); err != nil {
				return apperr.Conflict("EMAIL_TAKEN", "This address has an account: enter its password to make it the platform admin")
			}
		}
		if err := q.SetPlatformAdmin(ctx, db.SetPlatformAdminParams{ID: u.ID, IsPlatformAdmin: true}); err != nil {
			return err
		}
		u.IsPlatformAdmin = true
		if err := audit.Record(ctx, q, audit.Entry{Action: "platform.bootstrap", EntityType: "user", EntityID: &u.ID,
			Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID, Email: u.Email}}); err != nil {
			return err
		}
		out, err = s.createPlatformSession(ctx, q, u)
		return err
	})
	return out, err
}

// CreatePlatformAdmin makes the account at email a platform admin
// (`messora admin create`): a new, verified account when there is none
// (password required), otherwise the existing one, whose password changes
// when one is given.
func (s *Service) CreatePlatformAdmin(ctx context.Context, email, name, password string) (uuid.UUID, bool, error) {
	ctx = tenant.Platform(ctx)
	email, err := normalizeEmail("email", email)
	if err != nil {
		return uuid.Nil, false, err
	}
	var hash string
	if password != "" {
		if err := validatePassword("password", password, email); err != nil {
			return uuid.Nil, false, err
		}
		if hash, err = crypto.HashPassword(password, s.o.Argon2); err != nil {
			return uuid.Nil, false, err
		}
	}
	var (
		id      uuid.UUID
		created bool
	)
	err = s.tx(ctx, func(q *db.Queries) error {
		u, err := q.GetUserByEmail(ctx, email)
		switch {
		case isNoRows(err):
			if hash == "" {
				return errors.New("a password is required for a new account")
			}
			if strings.TrimSpace(name) == "" {
				name = strings.SplitN(email, "@", 2)[0]
			}
			now := s.now()
			if u, err = q.CreateUser(ctx, db.CreateUserParams{Email: email, Name: strings.TrimSpace(name), PasswordHash: hash,
				Locale: "en", Timezone: "UTC", EmailVerifiedAt: &now}); err != nil {
				return err
			}
			created = true
		case err != nil:
			return err
		case hash != "":
			if err := q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: u.ID, PasswordHash: hash}); err != nil {
				return err
			}
		}
		id = u.ID
		if err := q.SetPlatformAdmin(ctx, db.SetPlatformAdminParams{ID: u.ID, IsPlatformAdmin: true}); err != nil {
			return err
		}
		if err := q.DeletePlatformSetting(ctx, settingBootstrap); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.admin_create", EntityType: "user", EntityID: &u.ID,
			Changes: map[string]any{"email": email, "created": created, "via": "cli"}})
	})
	return id, created, err
}
