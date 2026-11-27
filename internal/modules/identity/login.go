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
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

var (
	errInvalidCredentials = apperr.Unauthorized("INVALID_CREDENTIALS", "E-mail or password is incorrect")
	errNotVerified        = apperr.Forbidden("EMAIL_NOT_VERIFIED", "Confirm your e-mail address first: open the link we sent you, or request a new one.")
	errResetRequired      = apperr.Forbidden("PASSWORD_RESET_REQUIRED", "Choose a new password with \"Forgot password\" to continue.")
)

func normalizeEmail(field, raw string) (string, error) {
	a, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || a.Name != "" || !strings.Contains(a.Address, ".") {
		return "", apperr.Validation(field, "Enter a valid e-mail address")
	}
	return strings.ToLower(a.Address), nil
}

// ── Login ──────────────────────────────────────────────────────────────────

// Login rate limits. Per address they are global: an account is one
// person, so guessing its password on several workspaces shares a budget.
var (
	loginPerIP    = ratelimit.PerMinute(30)
	loginPerEmail = ratelimit.PerMinute(10)
)

// checkPassword verifies the password of the account at email, with the
// rate limits and lockout every sign-in shares (workspaces and the admin
// panel). It returns the account; any failure looks like a wrong password.
func (s *Service) checkPassword(ctx context.Context, rawEmail, password string) (db.User, error) {
	email, err := normalizeEmail("email", rawEmail)
	if err != nil {
		return db.User{}, errInvalidCredentials
	}
	if err := s.rateLimit(ctx, "login:email:"+email, loginPerEmail); err != nil {
		return db.User{}, err
	}
	if ip := principal.Meta(ctx).IP; ip.IsValid() {
		if err := s.rateLimit(ctx, "login:ip:"+ip.String(), loginPerIP); err != nil {
			return db.User{}, err
		}
	}
	u, err := s.q.GetUserByEmail(ctx, email)
	if isNoRows(err) {
		crypto.BurnVerify(password) // same timing as a real check
		return db.User{}, errInvalidCredentials
	}
	if err != nil {
		return db.User{}, err
	}
	if u.LockedUntil != nil && s.now().Before(*u.LockedUntil) {
		crypto.BurnVerify(password)
		return db.User{}, apperr.RateLimited("ACCOUNT_LOCKED", "Too many failed attempts. Try again later or reset your password.", u.LockedUntil.Sub(s.now()))
	}
	needsRehash, verr := crypto.VerifyPassword(password, u.PasswordHash, s.o.Argon2)
	if verr != nil {
		return db.User{}, s.loginFailed(ctx, u, "password")
	}
	if u.Status != "active" {
		return db.User{}, errInvalidCredentials
	}
	if needsRehash {
		if h, err := crypto.HashPassword(password, s.o.Argon2); err == nil {
			_ = s.q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: u.ID, PasswordHash: h})
		}
	}
	_ = s.rdb.Del(ctx, rateKey("login:email:"+email)).Err()
	return u, nil
}

// Login verifies credentials for the request's workspace. With two-factor
// authentication enabled it returns an MFA challenge token (valid only at
// this workspace) instead of a session.
func (s *Service) Login(ctx context.Context, req api.LoginRequest) (api.LoginResult, error) {
	u, err := s.checkPassword(ctx, string(req.Email), req.Password)
	if err != nil {
		return api.LoginResult{}, err
	}
	// An account that is not a member here fails like a wrong password,
	// so a workspace's address reveals nothing about other workspaces.
	// What follows (the password was right) only tells its owner why they
	// cannot get in.
	role, err := s.q.GetUserRole(ctx, u.ID)
	if isNoRows(err) {
		return api.LoginResult{}, errInvalidCredentials
	}
	if err != nil {
		return api.LoginResult{}, err
	}
	if role.MembershipStatus != "active" {
		return api.LoginResult{}, errInvalidCredentials
	}
	if u.EmailVerifiedAt == nil {
		return api.LoginResult{}, errNotVerified
	}
	if u.PasswordResetRequired {
		return api.LoginResult{}, errResetRequired
	}
	if u.TotpConfirmedAt != nil {
		token, err := s.newMFAChallenge(ctx, u.ID, scopeOf(ctx))
		if err != nil {
			return api.LoginResult{}, err
		}
		return api.LoginResult{Status: api.LoginResultStatusMfaRequired, MfaToken: &token}, nil
	}
	sess, err := s.memberSession(ctx, u)
	if err != nil {
		return api.LoginResult{}, err
	}
	return api.LoginResult{Status: api.LoginResultStatusAuthenticated, Session: &sess}, nil
}

func (s *Service) loginFailed(ctx context.Context, u db.User, reason string) error {
	var res db.RecordLoginFailureRow
	err := s.tx(ctx, func(q *db.Queries) error {
		var err error
		res, err = q.RecordLoginFailure(ctx, db.RecordLoginFailureParams{ID: u.ID, MaxFailures: maxLoginFailures, LockMinutes: lockMinutes})
		if err != nil {
			return err
		}
		if tenant.ID(ctx) == uuid.Nil && !tenant.IsPlatform(ctx) {
			return nil // no audit log to write to
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

// rateKey is the Redis key of an identity rate limit. Every one of them
// is person- or address-global (tenant.GlobalRedisKey): an account signs in
// to several workspaces, so guessing its password, resetting it or probing
// sign-up shares one budget whatever address is used.
func rateKey(key string) string { return tenant.GlobalRedisKey("rl:" + key) }

func (s *Service) rateLimit(ctx context.Context, key string, lim ratelimit.Limit) error {
	r, err := s.limiter.AllowRaw(ctx, rateKey(key), lim)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return apperr.RateLimited("RATE_LIMITED", "Too many attempts. Please wait and try again.", r.RetryAfter)
	}
	return nil
}

// ── Second factor ──────────────────────────────────────────────────────────

// mfaChallenge is a pending sign-in between the password and the code. It
// is bound to where it started: one workspace, or the admin panel.
type mfaChallenge struct {
	UserID   uuid.UUID `json:"user_id"`
	Scope    string    `json:"scope"`
	Attempts int       `json:"attempts"`
}

// scopeOf names where a sign-in happens: "tenant:<id>" or "platform".
func scopeOf(ctx context.Context) string {
	if id, ok := tenant.FromCtx(ctx); ok {
		return "tenant:" + id.String()
	}
	return "platform"
}

// mfaKey is the Redis key of a pending sign-in: under the workspace's
// prefix (tenant.RedisKey) for a workspace sign-in, platform-global for the
// admin panel. A challenge is never found from another scope.
func mfaKey(scope, token string) string {
	hash := fmt.Sprintf("%x", crypto.HashToken(token))
	if id, ok := strings.CutPrefix(scope, "tenant:"); ok {
		if tid, err := uuid.Parse(id); err == nil {
			return tenant.RedisPrefix(tid) + "mfa:" + hash
		}
	}
	return tenant.GlobalRedisKey("mfa:platform:" + hash)
}

func (s *Service) newMFAChallenge(ctx context.Context, userID uuid.UUID, scope string) (string, error) {
	token, err := crypto.NewToken(mfaTokenPrefix)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(mfaChallenge{UserID: userID, Scope: scope})
	return token, s.rdb.Set(ctx, mfaKey(scope, token), b, mfaChallengeTTL).Err()
}

var (
	errMFAInvalid = apperr.Unauthorized("MFA_INVALID", "The code is incorrect or expired")
	errMFAExpired = apperr.Unauthorized("MFA_EXPIRED", "The sign-in attempt expired. Sign in again.")
)

// passMFA checks the second factor of a pending sign-in of scope and
// consumes the challenge. It returns the account.
func (s *Service) passMFA(ctx context.Context, scope string, req api.LoginMfaRequest) (db.User, error) {
	if (req.Code == nil) == (req.RecoveryCode == nil) {
		return db.User{}, apperr.Validation("code", "Enter either an authenticator code or a recovery code")
	}
	key := mfaKey(scope, req.MfaToken)
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return db.User{}, errMFAExpired
	}
	var ch mfaChallenge
	if err := json.Unmarshal(raw, &ch); err != nil || ch.Scope != scope {
		return db.User{}, errMFAExpired
	}
	u, err := s.q.GetUserByID(ctx, ch.UserID)
	if err != nil {
		return db.User{}, err
	}
	ok, err := s.checkSecondFactor(ctx, s.q, u, req.Code, req.RecoveryCode)
	if err != nil {
		return db.User{}, err
	}
	if !ok {
		ch.Attempts++
		if ch.Attempts >= mfaMaxAttempts {
			s.rdb.Del(ctx, key)
			return db.User{}, s.loginFailed(ctx, u, "mfa")
		}
		b, _ := json.Marshal(ch)
		s.rdb.Set(ctx, key, b, goredisKeepTTL)
		return db.User{}, errMFAInvalid
	}
	// Single use: a concurrent second request with the same token fails.
	if n, err := s.rdb.Del(ctx, key).Result(); err != nil || n == 0 {
		return db.User{}, errMFAExpired
	}
	return u, nil
}

// LoginMFA completes a workspace sign-in that required a second factor.
func (s *Service) LoginMFA(ctx context.Context, req api.LoginMfaRequest) (api.SessionCreated, error) {
	u, err := s.passMFA(ctx, scopeOf(ctx), req)
	if err != nil {
		return api.SessionCreated{}, err
	}
	return s.memberSession(ctx, u)
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
		if n == 1 && (tenant.ID(ctx) != uuid.Nil || tenant.IsPlatform(ctx)) {
			_ = audit.Record(ctx, q, audit.Entry{Action: "user.recovery_code_used", EntityType: "user", EntityID: uuidPtr(u.ID),
				Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID, Email: u.Email}})
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
	if p.Support != nil {
		return time.Time{}, apperr.Forbidden("SUPPORT_SESSION", "Support sessions cannot confirm sensitive actions")
	}
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

// Me returns the signed-in user's view in this workspace.
func (s *Service) Me(ctx context.Context, p *principal.Principal) (api.Me, error) {
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return api.Me{}, err
	}
	var view api.User
	if p.Support != nil {
		view = toAPIUser(u, p.RoleKey, "Messora Support")
	} else if view, err = s.userView(ctx, s.q, u); err != nil {
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
	if t, err := s.q.GetTenant(ctx, p.TenantID); err == nil {
		me.Tenant = &struct {
			Id   uuid.UUID `json:"id"`
			Name string    `json:"name"`
			Slug string    `json:"slug"`
		}{Id: t.ID, Name: t.Name, Slug: t.Slug}
	}
	if sp := p.Support; sp != nil {
		me.Support = &api.SupportInfo{GrantId: sp.GrantID, AdminName: sp.AdminName, Reason: sp.Reason, ReadOnly: sp.ReadOnly, ExpiresAt: sp.ExpiresAt}
	}
	return me, nil
}

// UpdateMe changes profile fields (they belong to the person: every
// workspace shows the same name and language).
func (s *Service) UpdateMe(ctx context.Context, p *principal.Principal, req api.UpdateMeRequest) (api.Me, error) {
	if p.Support != nil {
		return api.Me{}, errSupportForbidden
	}
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

var errSupportForbidden = apperr.Forbidden("SUPPORT_SESSION", "Support sessions cannot change the account of the person signed in")

// ChangePassword verifies the current password, sets a new one and ends
// every other session in every workspace.
func (s *Service) ChangePassword(ctx context.Context, p *principal.Principal, req api.ChangePasswordRequest) error {
	if p.Support != nil {
		return errSupportForbidden
	}
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
	if err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.UpdatePasswordHash(ctx, db.UpdatePasswordHashParams{ID: u.ID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.InvalidatePasswordResets(ctx, u.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.password_change", EntityType: "user", EntityID: uuidPtr(u.ID)})
	}); err != nil {
		return err
	}
	if err := s.revokeEverywhere(ctx, u.ID, uuidPtr(p.SessionID)); err != nil {
		return err
	}
	s.notifyPasswordChanged(ctx, u)
	return nil
}

// notifyPasswordChanged sends the password_changed security notice.
func (s *Service) notifyPasswordChanged(ctx context.Context, u db.User) {
	ip, ua := requestOrigin(ctx)
	s.sendToUser(ctx, tenant.ID(ctx), u, templates.PasswordChanged, func(w workspace) templates.Data {
		d := templates.PasswordChangedData{Common: w.common(u.Name), ChangedAt: s.now(), SecureURL: w.link("/forgot-password")}
		if ip != nil {
			d.IP = ip.String()
		}
		if ua != nil {
			d.Device = deviceOf(*ua)
		}
		return d
	})
}

// ── Password reset ─────────────────────────────────────────────────────────

// PasswordResetEnabled reports whether e-mail delivery is configured.
func (s *Service) PasswordResetEnabled(ctx context.Context) bool { return s.mail.Enabled(ctx) }

// RequestPasswordReset e-mails a reset link for this workspace. It never
// reveals whether the address exists or belongs to the workspace; errors
// after validation are logged, not returned.
func (s *Service) RequestPasswordReset(ctx context.Context, req api.ForgotPasswordRequest) error {
	if !s.mail.Enabled(ctx) {
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
	if role, err := s.q.GetUserRole(ctx, u.ID); err != nil || role.MembershipStatus != "active" {
		if isNoRows(err) {
			return nil // not a member here: nothing to reset at this address
		}
		return err
	}
	_, err = s.sendPasswordReset(ctx, u)
	return err
}

// sendPasswordReset creates a reset token in the connection's workspace and
// e-mails its link (at most three per account and hour).
func (s *Service) sendPasswordReset(ctx context.Context, u db.User) (bool, error) {
	if n, err := s.q.CountRecentPasswordResets(ctx, u.ID); err != nil || n >= maxResetsPerHour {
		return false, err // silently drop: at most 3 e-mails per hour per account
	}
	token, err := crypto.NewToken(resetTokenPrefix)
	if err != nil {
		return false, err
	}
	if err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.CreatePasswordReset(ctx, db.CreatePasswordResetParams{UserID: u.ID, TokenHash: crypto.HashToken(token), ExpiresAt: s.now().Add(passwordResetTTL)}); err != nil {
			return err
		}
		return audit.RecordInTenant(ctx, q, audit.Entry{Action: "user.password_reset_request", EntityType: "user", EntityID: uuidPtr(u.ID),
			Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID}})
	}); err != nil {
		return false, err
	}
	ip, _ := requestOrigin(ctx)
	return s.sendToUser(ctx, tenant.ID(ctx), u, templates.PasswordReset, func(w workspace) templates.Data {
		d := templates.PasswordResetData{Common: w.common(u.Name), ResetURL: w.link("/reset-password", "token", token), ExpiresIn: passwordResetTTL}
		if ip != nil && !tenant.IsPlatform(ctx) {
			d.IP = ip.String()
		}
		return d
	}), nil
}

// SendPasswordResetIn e-mails a reset link for workspace tenantID (the
// platform admin panel's "force password reset").
func (s *Service) SendPasswordResetIn(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) (bool, error) {
	ctx = tenant.WithID(ctx, tenantID)
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return s.sendPasswordReset(ctx, u)
}

// ResetPassword sets a new password with a reset token and ends every
// session in every workspace.
func (s *Service) ResetPassword(ctx context.Context, req api.ResetPasswordRequest) error {
	invalid := apperr.Forbidden("RESET_TOKEN_INVALID", "This reset link is invalid or has expired. Request a new one.")
	if !strings.HasPrefix(req.Token, resetTokenPrefix) {
		return invalid
	}
	var u db.User
	err := s.tx(ctx, func(q *db.Queries) error {
		userID, err := q.ConsumePasswordReset(ctx, crypto.HashToken(req.Token))
		if isNoRows(err) {
			return invalid
		}
		if err != nil {
			return err
		}
		if u, err = q.GetUserByID(ctx, userID); err != nil {
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
		if err := q.RecordLoginSuccess(ctx, u.ID); err != nil { // clears a lockout
			return err
		}
		// Following the link proves the address too (an unverified
		// sign-up that forgot its password).
		if _, err := q.MarkEmailVerified(ctx, u.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.password_reset", EntityType: "user", EntityID: uuidPtr(u.ID),
			Actor: &principal.Principal{Kind: principal.KindUser, ID: u.ID}})
	})
	if err != nil {
		return err
	}
	if err := s.revokeEverywhere(ctx, u.ID, nil); err != nil {
		return err
	}
	s.notifyPasswordChanged(ctx, u)
	return nil
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
