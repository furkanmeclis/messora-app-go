package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

var errInvalidCredentials = apperr.Unauthorized("INVALID_CREDENTIALS", "E-mail or password is incorrect")

func normalizeEmail(field, raw string) (string, error) {
	a, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || a.Name != "" || !strings.Contains(a.Address, ".") {
		return "", apperr.Validation(field, "Enter a valid e-mail address")
	}
	return strings.ToLower(a.Address), nil
}

// ── Setup ──────────────────────────────────────────────────────────────────

const setupTokenKey = "setup.token"

type setupToken struct {
	Hash      []byte    `json:"hash"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SetupRequired reports whether no user exists yet.
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	n, err := s.q.CountUsers(ctx)
	return n == 0, err
}

// EnsureSetupToken creates a one-time setup token when no user exists and no
// valid token is stored. It returns the token only to the replica that
// created it, which logs it; force replaces an existing token (CLI).
func (s *Service) EnsureSetupToken(ctx context.Context, force bool) (string, error) {
	required, err := s.SetupRequired(ctx)
	if err != nil || !required {
		return "", err
	}
	if !force {
		if raw, err := s.q.GetSetting(ctx, setupTokenKey); err == nil {
			var t setupToken
			if json.Unmarshal(raw, &t) == nil && s.now().Before(t.ExpiresAt) {
				return "", nil // a valid token exists; another start already logged it
			}
		}
	}
	token, err := crypto.NewToken(setupTokenPrefix)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(setupToken{Hash: crypto.HashToken(token), ExpiresAt: s.now().Add(setupTokenTTL)})
	if force {
		err = s.q.UpsertSetting(ctx, db.UpsertSettingParams{Key: setupTokenKey, Value: b})
	} else {
		var n int64
		_ = s.q.DeleteSetting(ctx, setupTokenKey) // drop an expired token
		n, err = s.q.InsertSettingIfAbsent(ctx, db.InsertSettingIfAbsentParams{Key: setupTokenKey, Value: b})
		if err == nil && n == 0 {
			return "", nil // a concurrent replica won
		}
	}
	if err != nil {
		return "", err
	}
	return token, nil
}

// SetupURL returns the link that completes setup with token.
func (s *Service) SetupURL(token string) string {
	return s.o.PublicAppURL + "/setup?token=" + token
}

// CompleteSetup creates the first owner. It is serialized by a row lock on
// the setup token so two concurrent requests cannot both create owners.
func (s *Service) CompleteSetup(ctx context.Context, req api.CompleteSetupRequest) (api.SessionCreated, error) {
	email, err := normalizeEmail("email", string(req.Email))
	if err != nil {
		return api.SessionCreated{}, err
	}
	if err := validatePassword("password", req.Password, email); err != nil {
		return api.SessionCreated{}, err
	}
	hash, err := crypto.HashPassword(req.Password, s.o.Argon2)
	if err != nil {
		return api.SessionCreated{}, err
	}
	var out api.SessionCreated
	err = s.tx(ctx, func(q *db.Queries) error {
		invalid := apperr.Forbidden("SETUP_TOKEN_INVALID", "The setup link is invalid or has expired. Run `messora setup token` for a new one.")
		if n, err := q.CountUsers(ctx); err != nil || n > 0 {
			if err == nil {
				err = apperr.Conflict("SETUP_COMPLETE", "Setup has already been completed")
			}
			return err
		}
		raw, err := q.GetSetting(ctx, setupTokenKey)
		if isNoRows(err) {
			return invalid
		}
		if err != nil {
			return err
		}
		var t setupToken
		if err := json.Unmarshal(raw, &t); err != nil || !s.now().Before(t.ExpiresAt) ||
			!constantTimeEqual(t.Hash, crypto.HashToken(req.Token)) {
			return invalid
		}
		// Deleting the token row serializes concurrent completions: the
		// second transaction finds no token and fails.
		if err := q.DeleteSetting(ctx, setupTokenKey); err != nil {
			return err
		}
		st := settings.Defaults()
		tz := st.DefaultTimezone
		u, err := q.CreateUser(ctx, db.CreateUserParams{Email: email, Name: strings.TrimSpace(req.Name), PasswordHash: hash, Locale: "en", Timezone: tz})
		if err != nil {
			return err
		}
		if err := q.AssignRole(ctx, db.AssignRoleParams{UserID: u.ID, RoleKey: "owner"}); err != nil {
			return err
		}
		if req.OrganizationName != nil {
			name := strings.TrimSpace(*req.OrganizationName)
			if _, _, err := s.settings.Update(ctx, q, settings.Patch{OrganizationName: &name}, &u.ID); err != nil {
				return err
			}
		}
		actor := &principal.Principal{Kind: principal.KindUser, ID: u.ID, Name: u.Name}
		if err := audit.Record(ctx, q, audit.Entry{Action: "setup.complete", EntityType: "user", EntityID: uuidPtr(u.ID), Actor: actor, Changes: map[string]any{"email": email}}); err != nil {
			return err
		}
		out, err = s.createSession(ctx, q, u)
		return err
	})
	s.settings.Invalidate() // the organization name may have changed
	return out, err
}

// ── Login ──────────────────────────────────────────────────────────────────

var (
	loginPerIP    = ratelimit.PerMinute(30)
	loginPerEmail = ratelimit.PerMinute(10)
)

// Login verifies credentials. With two-factor authentication enabled it
// returns an MFA challenge token instead of a session.
func (s *Service) Login(ctx context.Context, req api.LoginRequest) (api.LoginResult, error) {
	email, err := normalizeEmail("email", string(req.Email))
	if err != nil {
		return api.LoginResult{}, errInvalidCredentials
	}
	if err := s.rateLimit(ctx, "login:email:"+email, loginPerEmail); err != nil {
		return api.LoginResult{}, err
	}
	if ip := principal.Meta(ctx).IP; ip.IsValid() {
		if err := s.rateLimit(ctx, "login:ip:"+ip.String(), loginPerIP); err != nil {
			return api.LoginResult{}, err
		}
	}

	u, err := s.q.GetUserByEmail(ctx, email)
	if isNoRows(err) {
		crypto.BurnVerify(req.Password) // same timing as a real check
		return api.LoginResult{}, errInvalidCredentials
	}
	if err != nil {
		return api.LoginResult{}, err
	}
	if u.LockedUntil != nil && s.now().Before(*u.LockedUntil) {
		crypto.BurnVerify(req.Password)
		return api.LoginResult{}, apperr.RateLimited("ACCOUNT_LOCKED", "Too many failed attempts. Try again later or reset your password.", u.LockedUntil.Sub(s.now()))
	}
	needsRehash, verr := crypto.VerifyPassword(req.Password, u.PasswordHash, s.o.Argon2)
	if verr != nil {
		return api.LoginResult{}, s.loginFailed(ctx, u, "password")
	}
	if u.Status != "active" {
		return api.LoginResult{}, errInvalidCredentials
	}
	if needsRehash {
		if h, err := crypto.HashPassword(req.Password, s.o.Argon2); err == nil {
			_ = s.q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: u.ID, PasswordHash: h})
		}
	}
	_ = s.limiter.Reset(ctx, "login:email:"+email)

	if u.TotpConfirmedAt != nil {
		token, err := s.newMFAChallenge(ctx, u.ID)
		if err != nil {
			return api.LoginResult{}, err
		}
		return api.LoginResult{Status: api.MfaRequired, MfaToken: &token}, nil
	}
	var sess api.SessionCreated
	if err := s.tx(ctx, func(q *db.Queries) error {
		sess, err = s.createSession(ctx, q, u)
		return err
	}); err != nil {
		return api.LoginResult{}, err
	}
	return api.LoginResult{Status: api.Authenticated, Session: &sess}, nil
}

func (s *Service) loginFailed(ctx context.Context, u db.User, reason string) error {
	var res db.RecordLoginFailureRow
	err := s.tx(ctx, func(q *db.Queries) error {
		var err error
		res, err = q.RecordLoginFailure(ctx, db.RecordLoginFailureParams{ID: u.ID, MaxFailures: maxLoginFailures, LockMinutes: lockMinutes})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{
			Action: "user.login_failed", EntityType: "user", EntityID: uuidPtr(u.ID),
			Changes: map[string]any{"reason": reason, "consecutive_failures": res.FailedLogins, "locked": res.LockedUntil != nil},
		})
	})
	if err != nil {
		return err
	}
	if res.LockedUntil != nil && s.now().Before(*res.LockedUntil) {
		return apperr.RateLimited("ACCOUNT_LOCKED", "Too many failed attempts. Try again later or reset your password.", res.LockedUntil.Sub(s.now()))
	}
	return errInvalidCredentials
}

func (s *Service) rateLimit(ctx context.Context, key string, lim ratelimit.Limit) error {
	r, err := s.limiter.Allow(ctx, key, lim)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return apperr.RateLimited("RATE_LIMITED", "Too many attempts. Please wait and try again.", r.RetryAfter)
	}
	return nil
}

// ── Second factor ──────────────────────────────────────────────────────────

type mfaChallenge struct {
	UserID   uuid.UUID `json:"user_id"`
	Attempts int       `json:"attempts"`
}

func mfaKey(token string) string { return fmt.Sprintf("messora:mfa:%x", crypto.HashToken(token)) }

func (s *Service) newMFAChallenge(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := crypto.NewToken(mfaTokenPrefix)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(mfaChallenge{UserID: userID})
	return token, s.rdb.Set(ctx, mfaKey(token), b, mfaChallengeTTL).Err()
}

var errMFAInvalid = apperr.Unauthorized("MFA_INVALID", "The code is incorrect or expired")

// LoginMFA completes a login that required a second factor.
func (s *Service) LoginMFA(ctx context.Context, req api.LoginMfaRequest) (api.SessionCreated, error) {
	if (req.Code == nil) == (req.RecoveryCode == nil) {
		return api.SessionCreated{}, apperr.Validation("code", "Enter either an authenticator code or a recovery code")
	}
	key := mfaKey(req.MfaToken)
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return api.SessionCreated{}, apperr.Unauthorized("MFA_EXPIRED", "The sign-in attempt expired. Sign in again.")
	}
	var ch mfaChallenge
	if err := json.Unmarshal(raw, &ch); err != nil {
		return api.SessionCreated{}, err
	}
	u, err := s.q.GetUserByID(ctx, ch.UserID)
	if err != nil {
		return api.SessionCreated{}, err
	}
	ok, err := s.checkSecondFactor(ctx, s.q, u, req.Code, req.RecoveryCode)
	if err != nil {
		return api.SessionCreated{}, err
	}
	if !ok {
		ch.Attempts++
		if ch.Attempts >= mfaMaxAttempts {
			s.rdb.Del(ctx, key)
			return api.SessionCreated{}, s.loginFailed(ctx, u, "mfa")
		}
		b, _ := json.Marshal(ch)
		s.rdb.Set(ctx, key, b, goredisKeepTTL)
		return api.SessionCreated{}, errMFAInvalid
	}
	// Single use: a concurrent second request with the same token fails.
	if n, err := s.rdb.Del(ctx, key).Result(); err != nil || n == 0 {
		return api.SessionCreated{}, apperr.Unauthorized("MFA_EXPIRED", "The sign-in attempt expired. Sign in again.")
	}
	var sess api.SessionCreated
	err = s.tx(ctx, func(q *db.Queries) error {
		sess, err = s.createSession(ctx, q, u)
		return err
	})
	return sess, err
}

// checkSecondFactor validates a TOTP code (rejecting reuse) or consumes a
// recovery code.
func (s *Service) checkSecondFactor(ctx context.Context, q *db.Queries, u db.User, code, recovery *string) (bool, error) {
	if u.TotpConfirmedAt == nil || u.TotpSecretEnc == nil {
		return false, nil
	}
	if recovery != nil {
		c := normalizeRecoveryCode(*recovery)
		n, err := q.UseRecoveryCode(ctx, db.UseRecoveryCodeParams{UserID: u.ID, CodeHash: crypto.HashToken(c)})
		if err != nil {
			return false, err
		}
		if n == 1 {
			_ = audit.Record(ctx, q, audit.Entry{Action: "user.recovery_code_used", EntityType: "user", EntityID: uuidPtr(u.ID),
				Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID}})
		}
		return n == 1, nil
	}
	secret, err := s.sealer.Open(u.TotpSecretEnc, totpAD(u.ID))
	if err != nil {
		return false, err
	}
	return s.validateTOTP(ctx, u.ID, string(secret), *code)
}

// ── Re-authentication ──────────────────────────────────────────────────────

// Reauthenticate confirms the user's identity with their password or, when
// two-factor authentication is enabled, alternatively a TOTP code.
func (s *Service) Reauthenticate(ctx context.Context, p *principal.Principal, req api.ReauthRequest) (time.Time, error) {
	if err := s.rateLimit(ctx, "reauth:"+p.ID.String(), ratelimit.PerMinute(10)); err != nil {
		return time.Time{}, err
	}
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return time.Time{}, err
	}
	switch {
	case req.Password != nil:
		if _, err := crypto.VerifyPassword(*req.Password, u.PasswordHash, s.o.Argon2); err != nil {
			return time.Time{}, apperr.Unauthorized("INVALID_CREDENTIALS", "Password is incorrect")
		}
	case req.Code != nil:
		ok, err := s.checkSecondFactor(ctx, s.q, u, req.Code, nil)
		if err != nil {
			return time.Time{}, err
		}
		if !ok {
			return time.Time{}, errMFAInvalid
		}
	default:
		return time.Time{}, apperr.Validation("password", "Enter your password")
	}
	var at *time.Time
	err = s.tx(ctx, func(q *db.Queries) error {
		var err error
		if at, err = q.MarkSessionReauthenticated(ctx, p.SessionID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.reauth", EntityType: "user", EntityID: uuidPtr(p.ID)})
	})
	if err != nil || at == nil {
		return time.Time{}, err
	}
	return at.Add(ReauthWindow), nil
}

// ── Profile and password ───────────────────────────────────────────────────

// Me returns the signed-in user's view.
func (s *Service) Me(ctx context.Context, p *principal.Principal) (api.Me, error) {
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return api.Me{}, err
	}
	view, err := s.userView(ctx, s.q, u)
	if err != nil {
		return api.Me{}, err
	}
	perms := make([]api.Permission, len(p.Permissions))
	copy(perms, p.Permissions)
	me := api.Me{User: view, Permissions: perms, MfaRequired: p.MFAPending}
	me.Session.Id = p.SessionID
	me.Session.ExpiresAt = p.ExpiresAt
	if p.ReauthValid(ReauthWindow, s.now()) {
		until := p.ReauthAt.Add(ReauthWindow)
		me.Session.ReauthValidUntil = &until
	}
	return me, nil
}

// UpdateMe changes profile fields.
func (s *Service) UpdateMe(ctx context.Context, p *principal.Principal, req api.UpdateMeRequest) (api.Me, error) {
	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil {
			return api.Me{}, apperr.Validation("timezone", "Unknown time zone")
		}
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return api.Me{}, apperr.Validation("name", "Enter your name")
		}
		req.Name = &n
	}
	err := s.tx(ctx, func(q *db.Queries) error {
		before, err := q.GetUserByID(ctx, p.ID)
		if err != nil {
			return err
		}
		after, err := q.UpdateProfile(ctx, db.UpdateProfileParams{ID: p.ID, Name: req.Name, Locale: req.Locale, Timezone: req.Timezone})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.profile_update", EntityType: "user", EntityID: uuidPtr(p.ID),
			Changes: diff(map[string][2]any{"name": {before.Name, after.Name}, "locale": {before.Locale, after.Locale}, "timezone": {before.Timezone, after.Timezone}})})
	})
	if err != nil {
		return api.Me{}, err
	}
	return s.Me(ctx, p)
}

// ChangePassword verifies the current password, sets a new one and ends
// every other session.
func (s *Service) ChangePassword(ctx context.Context, p *principal.Principal, req api.ChangePasswordRequest) error {
	if err := s.rateLimit(ctx, "password:"+p.ID.String(), ratelimit.PerMinute(6)); err != nil {
		return err
	}
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return err
	}
	if _, err := crypto.VerifyPassword(req.CurrentPassword, u.PasswordHash, s.o.Argon2); err != nil {
		return apperr.Validation("current_password", "Current password is incorrect")
	}
	if err := validatePassword("new_password", req.NewPassword, u.Email); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(req.NewPassword, s.o.Argon2)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: u.ID, PasswordHash: hash}); err != nil {
			return err
		}
		if _, err := q.RevokeUserSessions(ctx, db.RevokeUserSessionsParams{UserID: u.ID, ExceptID: uuidPtr(p.SessionID)}); err != nil {
			return err
		}
		if err := q.InvalidatePasswordResets(ctx, u.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.password_change", EntityType: "user", EntityID: uuidPtr(u.ID)})
	})
}

// ── Password reset ─────────────────────────────────────────────────────────

// PasswordResetEnabled reports whether e-mail delivery is configured.
func (s *Service) PasswordResetEnabled() bool { return s.mail.Enabled() }

// RequestPasswordReset e-mails a reset link. It never reveals whether the
// address exists; errors after validation are logged, not returned.
func (s *Service) RequestPasswordReset(ctx context.Context, req api.ForgotPasswordRequest) error {
	if !s.mail.Enabled() {
		return apperr.Unavailable("EMAIL_NOT_CONFIGURED", "Password reset by e-mail is not available. Ask an administrator.")
	}
	email, err := normalizeEmail("email", string(req.Email))
	if err != nil {
		return err
	}
	if ip := principal.Meta(ctx).IP; ip.IsValid() {
		if err := s.rateLimit(ctx, "reset:ip:"+ip.String(), ratelimit.PerMinute(5)); err != nil {
			return err
		}
	}
	u, err := s.q.GetUserByEmail(ctx, email)
	if isNoRows(err) || (err == nil && u.Status != "active") {
		return nil
	}
	if err != nil {
		return err
	}
	if n, err := s.q.CountRecentPasswordResets(ctx, u.ID); err != nil || n >= maxResetsPerHour {
		return err // silently drop: at most 3 e-mails per hour per account
	}
	token, err := crypto.NewToken(resetTokenPrefix)
	if err != nil {
		return err
	}
	if err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.CreatePasswordReset(ctx, db.CreatePasswordResetParams{UserID: u.ID, TokenHash: crypto.HashToken(token), ExpiresAt: s.now().Add(passwordResetTTL)}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.password_reset_request", EntityType: "user", EntityID: uuidPtr(u.ID),
			Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID}})
	}); err != nil {
		return err
	}
	st, _ := s.settings.Get(ctx)
	link := s.o.PublicAppURL + "/reset-password?token=" + token
	msg := mailer.Message{
		To:      u.Email,
		Subject: "Reset your " + st.OrganizationName + " password",
		Body: fmt.Sprintf("Hello %s,\n\nSomeone asked to reset the password of your %s account.\n"+
			"Open this link within 30 minutes to choose a new password:\n\n%s\n\n"+
			"If it was not you, ignore this e-mail; your password stays unchanged.\n",
			u.Name, st.OrganizationName, link),
	}
	if err := s.mail.Send(ctx, msg); err != nil {
		s.o.Logger.ErrorContext(ctx, "password reset e-mail failed", "user_id", u.ID, "err", err)
	}
	return nil
}

// ResetPassword sets a new password with a reset token and ends every session.
func (s *Service) ResetPassword(ctx context.Context, req api.ResetPasswordRequest) error {
	invalid := apperr.Forbidden("RESET_TOKEN_INVALID", "This reset link is invalid or has expired. Request a new one.")
	if !strings.HasPrefix(req.Token, resetTokenPrefix) {
		return invalid
	}
	return s.tx(ctx, func(q *db.Queries) error {
		userID, err := q.ConsumePasswordReset(ctx, crypto.HashToken(req.Token))
		if isNoRows(err) {
			return invalid
		}
		if err != nil {
			return err
		}
		u, err := q.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}
		if err := validatePassword("password", req.Password, u.Email); err != nil {
			return err
		}
		hash, err := crypto.HashPassword(req.Password, s.o.Argon2)
		if err != nil {
			return err
		}
		if err := q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: u.ID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.InvalidatePasswordResets(ctx, u.ID); err != nil {
			return err
		}
		if _, err := q.RevokeUserSessions(ctx, db.RevokeUserSessionsParams{UserID: u.ID}); err != nil {
			return err
		}
		if err := q.RecordLoginSuccess(ctx, u.ID); err != nil { // clears a lockout
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.password_reset", EntityType: "user", EntityID: uuidPtr(u.ID),
			Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID}})
	})
}

// diff keeps only changed fields as [old, new].
func diff(fields map[string][2]any) map[string]any {
	out := map[string]any{}
	for k, v := range fields {
		if v[0] != v[1] {
			out[k] = []any{v[0], v[1]}
		}
	}
	return out
}
