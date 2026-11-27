package identity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Platform setting keys (platform_settings).
const (
	// SettingSignupEnabled holds a JSON boolean; missing means enabled.
	SettingSignupEnabled = "signup_enabled"
	// SettingReplyTo holds the Reply-To address of the platform's e-mail
	// (a JSON string; missing means none).
	SettingReplyTo = "email.reply_to"
)

var (
	errSignupDisabled = apperr.Forbidden("SIGNUP_DISABLED", "Sign-up is closed at the moment.")
	errSlugTaken      = &apperr.Error{Status: http.StatusConflict, Code: "SLUG_TAKEN", Message: "This address is already taken", Field: "workspace_slug"}
	errSlugReserved   = &apperr.Error{Status: http.StatusUnprocessableEntity, Code: "SLUG_RESERVED", Message: "This address is reserved", Field: "workspace_slug"}
	errEmailTaken     = &apperr.Error{Status: http.StatusConflict, Code: "EMAIL_TAKEN", Message: "An account with this e-mail address already exists. Sign in, or reset your password.", Field: "email"}

	errVerifyInvalid = &apperr.Error{Status: http.StatusBadRequest, Code: "VERIFICATION_TOKEN_INVALID", Message: "This verification link is invalid or was already used."}
	errVerifyExpired = &apperr.Error{Status: http.StatusGone, Code: "VERIFICATION_TOKEN_EXPIRED", Message: "This verification link has expired. Request a new one."}
	errVerified      = apperr.Conflict("EMAIL_ALREADY_VERIFIED", "This e-mail address is already confirmed. Sign in.")
)

// Sign-up budgets per client address.
var (
	signupPerIP = ratelimit.Limit{Rate: 5, Period: 10 * time.Minute, Burst: 5}
	resendPerIP = ratelimit.PerMinute(10)
	// One verification e-mail per address and minute (burst 1).
	resendPerEmail = ratelimit.Limit{Rate: 1, Period: resendVerifyInterval, Burst: 1}
)

// SignupEnabled reports the platform setting signup_enabled (default on).
func (s *Service) SignupEnabled(ctx context.Context) (bool, error) {
	raw, err := s.q.GetPlatformSetting(tenant.System(ctx), SettingSignupEnabled)
	if isNoRows(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	var on bool
	if err := json.Unmarshal(raw, &on); err != nil {
		return false, nil
	}
	return on, nil
}

// validateNewSlug applies the slug rules to a new workspace address.
func validateNewSlug(slug string) error {
	switch err := tenant.ValidateSlug(slug); {
	case errors.Is(err, tenant.ErrSlugReserved):
		return errSlugReserved
	case err != nil:
		return apperr.Validation("workspace_slug", "Use 3–30 lowercase letters, digits or single hyphens inside")
	}
	return nil
}

// SlugAvailability says whether slug can be a new workspace's address.
func (s *Service) SlugAvailability(ctx context.Context, raw string) (api.SlugAvailability, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	out := api.SlugAvailability{Slug: slug}
	if err := validateNewSlug(slug); err != nil {
		if errors.Is(err, errSlugReserved) {
			r := api.Reserved
			out.Reason = &r
			return out, nil
		}
		return out, err
	}
	if ip := principal.Meta(ctx).IP; ip.IsValid() {
		if err := s.rateLimit(ctx, "slug:ip:"+ip.String(), ratelimit.PerMinute(60)); err != nil {
			return out, err
		}
	}
	taken, err := s.q.SlugInUse(tenant.System(ctx), db.SlugInUseParams{Slug: slug, Hostname: s.o.Hosts.TenantHost(slug)})
	if err != nil {
		return out, err
	}
	out.Available = !taken
	if taken {
		r := api.Taken
		out.Reason = &r
	}
	return out, nil
}

// NewTenant describes a workspace to open (sign-up or the admin panel).
type NewTenant struct {
	Name, Slug, Locale string
	// Trial is the trial length; 0 opens the workspace with an active
	// subscription.
	Trial time.Duration
}

// createTenant opens a workspace in a platform transaction: the tenant,
// its sub-domain and its first settings. The transaction is left scoped to
// the new tenant (app.tenant_id), so the caller can write its rows.
func (s *Service) createTenant(ctx context.Context, q *db.Queries, nt NewTenant, by *uuid.UUID) (db.Tenant, error) {
	if s.o.Hosts.Base == "" {
		return db.Tenant{}, apperr.Unavailable("NO_BASE_DOMAIN", "Workspaces need TENANT_BASE_DOMAIN to be set")
	}
	if err := validateNewSlug(nt.Slug); err != nil {
		return db.Tenant{}, err
	}
	host := s.o.Hosts.TenantHost(nt.Slug)
	taken, err := q.SlugInUse(ctx, db.SlugInUseParams{Slug: nt.Slug, Hostname: host})
	if err != nil {
		return db.Tenant{}, err
	}
	if taken {
		return db.Tenant{}, errSlugTaken
	}
	params := db.CreateTenantParams{Slug: nt.Slug, Name: nt.Name, SubscriptionStatus: "active"}
	if nt.Trial > 0 {
		end := s.now().Add(nt.Trial)
		params.SubscriptionStatus, params.TrialEndsAt = "trial", &end
	}
	t, err := q.CreateTenant(ctx, params)
	if err != nil {
		if strings.Contains(err.Error(), "tenants_slug_key") {
			return db.Tenant{}, errSlugTaken
		}
		return db.Tenant{}, err
	}
	now := s.now()
	if _, err := q.CreateTenantDomain(ctx, db.CreateTenantDomainParams{TenantID: t.ID, Hostname: host, Kind: "subdomain", VerifiedAt: &now, IsPrimary: true}); err != nil {
		if strings.Contains(err.Error(), "tenant_domains_hostname_key") {
			return db.Tenant{}, errSlugTaken
		}
		return db.Tenant{}, err
	}
	// From here on the transaction writes the new tenant's own rows.
	if err := q.SetTenantLocal(ctx, t.ID.String()); err != nil {
		return db.Tenant{}, err
	}
	locale := templates.Locale(nt.Locale)
	tz := "UTC"
	if locale == "tr" {
		tz = "Europe/Istanbul"
	}
	if _, _, err := s.settings.Update(ctx, q, settings.Patch{OrganizationName: &nt.Name, DefaultLocale: &locale, DefaultTimezone: &tz}, by); err != nil {
		return db.Tenant{}, err
	}
	return t, nil
}

// CreateTenant opens a workspace from the platform admin panel.
func (s *Service) CreateTenant(ctx context.Context, nt NewTenant, admin *principal.Principal) (db.Tenant, error) {
	nt.Slug = strings.ToLower(strings.TrimSpace(nt.Slug))
	nt.Name = strings.TrimSpace(nt.Name)
	var t db.Tenant
	err := s.platformTx(ctx, func(q *db.Queries) error {
		var err error
		if t, err = s.createTenant(tenant.Platform(ctx), q, nt, nil); err != nil {
			return err
		}
		pctx := tenant.Platform(ctx)
		if err := audit.RecordInTenant(pctx, q, audit.Entry{Action: "tenant.create", EntityType: "tenant", EntityID: &t.ID, Actor: admin,
			Changes: map[string]any{"slug": t.Slug, "name": t.Name, "by": "platform"}}); err != nil {
			return err
		}
		return audit.Record(pctx, q, audit.Entry{Action: "tenant.create", EntityType: "tenant", EntityID: &t.ID, Tenant: &t.ID,
			Changes: map[string]any{"slug": t.Slug, "name": t.Name, "subscription_status": t.SubscriptionStatus}})
	})
	return t, err
}

// Signup opens a workspace and its owner's account (unverified) and
// e-mails the verification link. The cross-tenant steps (the registry,
// the global account, the membership) run as the platform role, in one
// transaction with the new tenant's first rows.
func (s *Service) Signup(ctx context.Context, req api.SignupRequest) (api.Signup, error) {
	on, err := s.SignupEnabled(ctx)
	if err != nil {
		return api.Signup{}, err
	}
	if !on || s.o.Hosts.Base == "" {
		return api.Signup{}, errSignupDisabled
	}
	if ip := principal.Meta(ctx).IP; ip.IsValid() {
		if err := s.rateLimit(ctx, "signup:ip:"+ip.String(), signupPerIP); err != nil {
			return api.Signup{}, err
		}
	}
	verr := &validationErrors{}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		verr.add("name", "Enter your name")
	}
	company := strings.TrimSpace(req.CompanyName)
	if company == "" {
		verr.add("company_name", "Enter your company's name")
	}
	email, err := normalizeEmail("email", string(req.Email))
	if err != nil {
		verr.add("email", "Enter a valid e-mail address")
	} else if err := validatePassword("password", req.Password, email); err != nil {
		if e, ok := apperr.As(err); ok {
			verr.add("password", e.Message)
		}
	}
	slug := strings.ToLower(strings.TrimSpace(req.WorkspaceSlug))
	if err := validateNewSlug(slug); err != nil {
		if errors.Is(err, errSlugReserved) && verr.empty() {
			return api.Signup{}, err
		}
		if !errors.Is(err, errSlugReserved) {
			verr.add("workspace_slug", "Use 3–30 lowercase letters, digits or single hyphens inside")
		}
	}
	if !verr.empty() {
		return api.Signup{}, verr.err()
	}
	hash, err := crypto.HashPassword(req.Password, s.o.Argon2)
	if err != nil {
		return api.Signup{}, err
	}
	token, err := crypto.NewToken(verifyTokenPrefix)
	if err != nil {
		return api.Signup{}, err
	}
	locale := templates.Locale(string(req.Locale))
	var (
		t db.Tenant
		u db.User
	)
	pctx := tenant.Platform(ctx)
	err = s.platformTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetUserByEmail(pctx, email); err == nil {
			return errEmailTaken
		} else if !isNoRows(err) {
			return err
		}
		tz := "UTC"
		if locale == "tr" {
			tz = "Europe/Istanbul"
		}
		u, err = q.CreateUser(pctx, db.CreateUserParams{Email: email, Name: name, PasswordHash: hash, Locale: locale, Timezone: tz})
		if err != nil {
			if strings.Contains(err.Error(), "users_email_key") {
				return errEmailTaken
			}
			return err
		}
		if t, err = s.createTenant(pctx, q, NewTenant{Name: company, Slug: slug, Locale: locale, Trial: trialDays * 24 * time.Hour}, &u.ID); err != nil {
			return err
		}
		if err := q.CreateMembershipIn(pctx, db.CreateMembershipInParams{TenantID: t.ID, UserID: u.ID, RoleKey: "owner"}); err != nil {
			return err
		}
		if err := q.CreateEmailVerification(pctx, db.CreateEmailVerificationParams{UserID: u.ID, TenantID: t.ID,
			TokenHash: crypto.HashToken(token), ExpiresAt: s.now().Add(verificationTTL)}); err != nil {
			return err
		}
		actor := &principal.Principal{Kind: principal.KindUser, ID: u.ID, Name: u.Name, Email: u.Email}
		if err := audit.RecordInTenant(pctx, q, audit.Entry{Action: "tenant.signup", EntityType: "tenant", EntityID: &t.ID, Actor: actor,
			Changes: map[string]any{"slug": t.Slug, "name": t.Name, "email": email}}); err != nil {
			return err
		}
		return audit.Record(pctx, q, audit.Entry{Action: "tenant.signup", EntityType: "tenant", EntityID: &t.ID, Tenant: &t.ID, Actor: actor,
			Changes: map[string]any{"slug": t.Slug, "trial_ends_at": t.TrialEndsAt}})
	})
	if err != nil {
		return api.Signup{}, err
	}
	if s.o.InvalidateHosts != nil {
		s.o.InvalidateHosts(s.o.Hosts.TenantHost(t.Slug))
	}
	s.sendVerification(ctx, t.ID, u, token)
	return api.Signup{Email: email, Tenant: api.TenantRef{Id: t.ID, Slug: t.Slug, Name: t.Name}, VerificationRequired: true}, nil
}

func (s *Service) sendVerification(ctx context.Context, tenantID uuid.UUID, u db.User, token string) {
	s.sendToUser(ctx, tenantID, u, templates.VerifyEmail, func(w workspace) templates.Data {
		return templates.VerifyEmailData{Common: w.common(u.Name), VerifyURL: w.link("/verify-email", "token", token), ExpiresIn: verificationTTL}
	})
}

// VerifyEmail confirms the address of a sign-up and welcomes the owner.
func (s *Service) VerifyEmail(ctx context.Context, token string) (api.VerifyEmailResult, error) {
	if !strings.HasPrefix(token, verifyTokenPrefix) {
		return api.VerifyEmailResult{}, errVerifyInvalid
	}
	var row db.GetEmailVerificationRow
	pctx := tenant.Platform(ctx)
	err := s.platformTx(ctx, func(q *db.Queries) error {
		var err error
		row, err = q.GetEmailVerification(pctx, crypto.HashToken(token))
		switch {
		case isNoRows(err):
			return errVerifyInvalid
		case err != nil:
			return err
		case row.EmailVerifiedAt != nil:
			return errVerified
		case row.UsedAt != nil:
			return errVerifyInvalid
		case !s.now().Before(row.ExpiresAt):
			return errVerifyExpired
		}
		if _, err := q.MarkEmailVerified(pctx, row.UserID); err != nil {
			return err
		}
		if err := q.UseEmailVerification(pctx, row.UserID); err != nil {
			return err
		}
		return audit.Record(pctx, q, audit.Entry{Action: "user.email_verify", EntityType: "user", EntityID: &row.UserID, Tenant: &row.TenantID,
			Actor: &principal.Principal{Kind: principal.KindUser, ID: row.UserID, Email: row.Email}})
	})
	if err != nil {
		return api.VerifyEmailResult{}, err
	}
	w, err := s.workspaceOf(ctx, row.TenantID)
	if err != nil {
		return api.VerifyEmailResult{}, err
	}
	if u, err := s.q.GetUserByID(pctx, row.UserID); err == nil {
		s.sendTo(ctx, w, u.Email, u.Locale, templates.Welcome, templates.WelcomeData{Common: w.common(u.Name),
			DashboardURL: w.link("/"), ConnectWhatsAppURL: w.link("/channels"), ImportContactsURL: w.link("/contacts"),
			NewCampaignURL: w.link("/campaigns/new")})
	}
	out := api.VerifyEmailResult{Email: row.Email, LoginUrl: w.link("/login", "verified", "1")}
	out.Tenant.Slug, out.Tenant.Name = row.Slug, w.Name
	return out, nil
}

// ResendVerification sends a new verification link to an account that
// still waits for one. It answers the same whether or not the address is
// known; the per-address limit (one a minute) applies either way.
func (s *Service) ResendVerification(ctx context.Context, raw string) error {
	email, err := normalizeEmail("email", raw)
	if err != nil {
		return err
	}
	if ip := principal.Meta(ctx).IP; ip.IsValid() {
		if err := s.rateLimit(ctx, "resend:ip:"+ip.String(), resendPerIP); err != nil {
			return err
		}
	}
	if err := s.rateLimit(ctx, "resend:email:"+email, resendPerEmail); err != nil {
		return err
	}
	pctx := tenant.Platform(ctx)
	u, err := s.q.GetUserByEmail(pctx, email)
	if isNoRows(err) || (err == nil && (u.EmailVerifiedAt != nil || u.Status != "active")) {
		return nil
	}
	if err != nil {
		return err
	}
	token, err := crypto.NewToken(verifyTokenPrefix)
	if err != nil {
		return err
	}
	var tenantID uuid.UUID
	err = s.platformTx(ctx, func(q *db.Queries) error {
		var err error
		if tenantID, err = q.LatestVerificationTenant(pctx, u.ID); err != nil {
			return err
		}
		if err := q.UseEmailVerification(pctx, u.ID); err != nil { // older links stop working
			return err
		}
		return q.CreateEmailVerification(pctx, db.CreateEmailVerificationParams{UserID: u.ID, TenantID: tenantID,
			TokenHash: crypto.HashToken(token), ExpiresAt: s.now().Add(verificationTTL)})
	})
	if isNoRows(err) {
		return nil // not a sign-up (an invited account is verified by its link)
	}
	if err != nil {
		return err
	}
	s.sendVerification(ctx, tenantID, u, token)
	return nil
}

// validationErrors collects field problems into one 422.
type validationErrors struct{ httpx.ValidationErrors }

func (v *validationErrors) add(field, msg string) { v.Add(field, msg) }
func (v *validationErrors) empty() bool           { return len(v.Details) == 0 }

func (v *validationErrors) err() error {
	if len(v.Details) == 1 {
		return apperr.Validation(v.Details[0].Field, v.Details[0].Msg)
	}
	return &v.ValidationErrors
}
