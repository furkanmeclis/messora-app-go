// Package platform is the platform admin panel (admin.<base>): companies
// (tenants), their domains, suspension, accounts across companies, support
// access, platform settings and the audit trail of platform admins.
//
// Every request reaches it on the admin host, which runs as platform work
// (tenant.Platform: the messora_platform database role). Steps that write a
// company's own rows (its audit log, an owner invitation, a support
// session) scope the transaction to that company.
package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// settingReplyTo is the platform's Reply-To address.
const settingReplyTo = identity.SettingReplyTo

// Sender describes the platform's outgoing e-mail (SMTP_* settings).
type Sender struct {
	Configured bool
	From, Host string
}

// Options configures the service.
type Options struct {
	Hosts tenant.Hosts
	// Invalidate drops cached host resolutions (tenant.Resolver).
	Invalidate func(hosts ...string)
	Sender     Sender
	// LookupTXT resolves TXT records (net.DefaultResolver.LookupTXT when nil).
	LookupTXT func(ctx context.Context, name string) ([]string, error)
	Logger    *slog.Logger
}

// Service implements the platform admin panel.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	id   *identity.Service
	o    Options
	now  func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, id *identity.Service, o Options) *Service {
	if o.LookupTXT == nil {
		o.LookupTXT = net.DefaultResolver.LookupTXT
	}
	if o.Invalidate == nil {
		o.Invalidate = func(...string) {}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Service{pool: pool, q: db.New(pool), id: id, o: o, now: time.Now}
}

// SetLookupTXT replaces the DNS lookup of domain verification (tests).
func (s *Service) SetLookupTXT(fn func(ctx context.Context, name string) ([]string, error)) {
	s.o.LookupTXT = fn
}

// ctx makes sure work runs as the platform (it does on the admin host).
func pctx(ctx context.Context) context.Context { return tenant.Platform(ctx) }

func (s *Service) tx(ctx context.Context, fn func(q *db.Queries) error) error {
	return s.id.PlatformTx(ctx, fn)
}

// inTenant runs fn in a platform transaction scoped to tenantID's rows.
func (s *Service) inTenant(ctx context.Context, tenantID uuid.UUID, fn func(q *db.Queries) error) error {
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.SetTenantLocal(pctx(ctx), tenantID.String()); err != nil {
			return err
		}
		return fn(q)
	})
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

var errTenantNotFound = apperr.NotFound("Company not found")

// ── Overview ───────────────────────────────────────────────────────────────

// Overview returns the platform counters.
func (s *Service) Overview(ctx context.Context) (api.PlatformOverview, error) {
	c, err := s.q.PlatformCounts(pctx(ctx))
	if err != nil {
		return api.PlatformOverview{}, err
	}
	return api.PlatformOverview{Tenants: c.Tenants, ActiveTenants: c.ActiveTenants, SuspendedTenants: c.SuspendedTenants,
		TrialTenants: c.TrialTenants, Users: c.Users, Signups7d: c.Signups7d, ActiveSupportSessions: c.ActiveSupportSessions}, nil
}

// ── Tenants ────────────────────────────────────────────────────────────────

func (s *Service) tenantURL(host string) *string {
	if host == "" {
		return nil
	}
	u := s.o.Hosts.URL(host)
	return &u
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func tenantView(t db.Tenant) api.PlatformTenant {
	return api.PlatformTenant{Id: t.ID, Slug: t.Slug, Name: t.Name, Status: api.TenantStatus(t.Status),
		SubscriptionStatus: api.SubscriptionStatus(t.SubscriptionStatus), TrialEndsAt: t.TrialEndsAt,
		SuspendedAt: t.SuspendedAt, SuspendedReason: t.SuspendedReason, CreatedAt: t.CreatedAt}
}

// ListTenants searches companies, newest first.
func (s *Service) ListTenants(ctx context.Context, p api.ListPlatformTenantsParams) ([]api.PlatformTenant, string, error) {
	limit := int32(50)
	if p.Limit != nil {
		limit = int32(*p.Limit)
	}
	var after *uuid.UUID
	if p.Cursor != nil && *p.Cursor != "" {
		id, err := uuid.Parse(string(*p.Cursor))
		if err != nil {
			return nil, "", apperr.Validation("cursor", "Invalid cursor")
		}
		after = &id
	}
	var status *string
	if p.Status != nil {
		v := string(*p.Status)
		status = &v
	}
	var q *string
	if p.Q != nil && strings.TrimSpace(*p.Q) != "" {
		v := strings.TrimSpace(*p.Q)
		q = &v
	}
	rows, err := s.q.ListTenants(pctx(ctx), db.ListTenantsParams{Query: q, Status: status, After: after, Lim: limit + 1})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > int(limit) {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID.String()
	}
	out := make([]api.PlatformTenant, len(rows))
	for i, r := range rows {
		t := tenantView(db.Tenant{ID: r.ID, Slug: r.Slug, Name: r.Name, Status: r.Status, SuspendedAt: r.SuspendedAt,
			CreatedAt: r.CreatedAt, SubscriptionStatus: r.SubscriptionStatus, TrialEndsAt: r.TrialEndsAt, SuspendedReason: r.SuspendedReason})
		t.MemberCount, t.PrimaryHost, t.Url = r.MemberCount, strPtr(r.PrimaryHost), s.tenantURL(r.PrimaryHost)
		out[i] = t
	}
	return out, next, nil
}

func (s *Service) domainView(d db.TenantDomain) api.PlatformDomain {
	v := api.PlatformDomain{Id: d.ID, TenantId: d.TenantID, Hostname: d.Hostname, Kind: api.PlatformDomainKind(d.Kind),
		IsPrimary: d.IsPrimary, VerifiedAt: d.VerifiedAt, LastCheckedAt: d.LastCheckedAt, LastCheckError: d.LastCheckError, CreatedAt: d.CreatedAt}
	if d.Kind == "custom" && d.VerificationToken != nil {
		v.Verification = &struct {
			Name  string                             `json:"name"`
			Type  api.PlatformDomainVerificationType `json:"type"`
			Value string                             `json:"value"`
		}{Name: "_messora." + d.Hostname, Type: api.TXT, Value: *d.VerificationToken}
	}
	return v
}

// Tenant returns a company with its domains and owners.
func (s *Service) Tenant(ctx context.Context, id uuid.UUID) (api.PlatformTenantDetail, error) {
	ctx = pctx(ctx)
	t, err := s.q.GetTenant(ctx, id)
	if isNoRows(err) {
		return api.PlatformTenantDetail{}, errTenantNotFound
	}
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	domains, err := s.q.ListTenantDomains(ctx, id)
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	owners, err := s.q.TenantOwners(ctx, id)
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	members, err := s.q.CountTenantMembers(ctx, id)
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	v := tenantView(t)
	out := api.PlatformTenantDetail{Id: v.Id, Slug: v.Slug, Name: v.Name, Status: v.Status, SubscriptionStatus: v.SubscriptionStatus,
		TrialEndsAt: v.TrialEndsAt, SuspendedAt: v.SuspendedAt, SuspendedReason: v.SuspendedReason, CreatedAt: v.CreatedAt,
		MemberCount: members, Domains: make([]api.PlatformDomain, 0, len(domains))}
	out.Owners = make([]struct {
		Email string    `json:"email"`
		Id    uuid.UUID `json:"id"`
		Name  string    `json:"name"`
	}, len(owners))
	for i, o := range owners {
		out.Owners[i].Email, out.Owners[i].Id, out.Owners[i].Name = o.Email, o.ID, o.Name
	}
	for _, d := range domains {
		out.Domains = append(out.Domains, s.domainView(d))
		if d.IsPrimary && d.VerifiedAt != nil {
			out.PrimaryHost, out.Url = strPtr(d.Hostname), s.tenantURL(d.Hostname)
		}
	}
	return out, nil
}

// hostsOf lists a tenant's host names (to drop them from the resolver cache).
func (s *Service) hostsOf(ctx context.Context, id uuid.UUID) []string {
	domains, err := s.q.ListTenantDomains(pctx(ctx), id)
	if err != nil {
		return nil
	}
	out := make([]string, len(domains))
	for i, d := range domains {
		out[i] = d.Hostname
	}
	return out
}

// CreateTenant opens a company and, with an owner address, invites its
// owner.
func (s *Service) CreateTenant(ctx context.Context, admin *principal.Principal, req api.CreatePlatformTenantRequest) (api.PlatformTenantCreated, error) {
	locale := "tr"
	if req.Locale != nil {
		locale = string(*req.Locale)
	}
	trial := 14 * 24 * time.Hour
	if req.TrialDays != nil {
		trial = time.Duration(*req.TrialDays) * 24 * time.Hour
	}
	t, err := s.id.CreateTenant(ctx, identity.NewTenant{Name: req.Name, Slug: req.Slug, Locale: locale, Trial: trial}, admin)
	if err != nil {
		return api.PlatformTenantCreated{}, err
	}
	s.o.Invalidate(s.hostsOf(ctx, t.ID)...)
	var out api.PlatformTenantCreated
	if req.OwnerEmail != nil {
		inv, err := s.InviteOwner(ctx, t.ID, string(*req.OwnerEmail))
		if err != nil {
			return out, err
		}
		out.OwnerInvitation = &inv
	}
	out.Tenant, err = s.Tenant(ctx, t.ID)
	return out, err
}

// InviteOwner invites an owner into a company; the link is returned once.
func (s *Service) InviteOwner(ctx context.Context, tenantID uuid.UUID, email string) (api.OwnerInvitation, error) {
	if _, err := s.q.GetTenant(pctx(ctx), tenantID); isNoRows(err) {
		return api.OwnerInvitation{}, errTenantNotFound
	} else if err != nil {
		return api.OwnerInvitation{}, err
	}
	inv, err := s.id.InviteOwner(ctx, tenantID, email)
	if err != nil {
		return api.OwnerInvitation{}, err
	}
	if err := s.tx(ctx, func(q *db.Queries) error {
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "tenant.owner_invite", EntityType: "invitation", EntityID: &inv.Invitation.Id,
			Tenant: &tenantID, Changes: map[string]any{"email": inv.Invitation.Email}})
	}); err != nil {
		return api.OwnerInvitation{}, err
	}
	return api.OwnerInvitation{Email: inv.Invitation.Email, AcceptUrl: inv.AcceptUrl, EmailSent: inv.EmailSent, ExpiresAt: inv.Invitation.ExpiresAt}, nil
}

// TenantPatch is a partial change of a company.
type TenantPatch struct {
	Name *string
	// SetTrial changes trial_ends_at to TrialEndsAt (nil clears it).
	SetTrial     bool
	TrialEndsAt  *time.Time
	Subscription *string
}

// UpdateTenant renames a company or changes its trial.
func (s *Service) UpdateTenant(ctx context.Context, id uuid.UUID, p TenantPatch) (api.PlatformTenantDetail, error) {
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" {
			return api.PlatformTenantDetail{}, apperr.Validation("name", "Enter the company's name")
		}
		p.Name = &n
	}
	err := s.inTenant(ctx, id, func(q *db.Queries) error {
		before, err := q.GetTenantForUpdate(pctx(ctx), id)
		if isNoRows(err) {
			return errTenantNotFound
		}
		if err != nil {
			return err
		}
		after, err := q.UpdateTenant(pctx(ctx), db.UpdateTenantParams{ID: id, Name: p.Name, SetTrial: p.SetTrial, TrialEndsAt: p.TrialEndsAt,
			SubscriptionStatus: p.Subscription})
		if err != nil {
			return err
		}
		changes := map[string]any{}
		if before.Name != after.Name {
			changes["name"] = []string{before.Name, after.Name}
		}
		if p.SetTrial {
			changes["trial_ends_at"] = []any{before.TrialEndsAt, after.TrialEndsAt}
		}
		if before.SubscriptionStatus != after.SubscriptionStatus {
			changes["subscription_status"] = []string{before.SubscriptionStatus, after.SubscriptionStatus}
		}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "tenant.update", EntityType: "tenant", EntityID: &id, Changes: changes}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "tenant.update", EntityType: "tenant", EntityID: &id, Tenant: &id, Changes: changes})
	})
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	return s.Tenant(ctx, id)
}

// Suspend stops a company: every address of it answers 403
// TENANT_SUSPENDED (within the resolver cache's 15 seconds on other
// replicas); its data stays.
func (s *Service) Suspend(ctx context.Context, id uuid.UUID, reason string) (api.PlatformTenantDetail, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 {
		return api.PlatformTenantDetail{}, apperr.Validation("reason", "Say why the company is suspended")
	}
	err := s.inTenant(ctx, id, func(q *db.Queries) error {
		if _, err := q.SuspendTenant(pctx(ctx), db.SuspendTenantParams{ID: id, Reason: &reason}); isNoRows(err) {
			if _, err := q.GetTenant(pctx(ctx), id); isNoRows(err) {
				return errTenantNotFound
			}
			return apperr.Conflict("TENANT_NOT_ACTIVE", "Only an active company can be suspended")
		} else if err != nil {
			return err
		}
		ch := map[string]any{"reason": reason}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "tenant.suspend", EntityType: "tenant", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "tenant.suspend", EntityType: "tenant", EntityID: &id, Tenant: &id, Changes: ch})
	})
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	s.o.Invalidate(s.hostsOf(ctx, id)...)
	return s.Tenant(ctx, id)
}

// Unsuspend lifts a suspension.
func (s *Service) Unsuspend(ctx context.Context, id uuid.UUID) (api.PlatformTenantDetail, error) {
	err := s.inTenant(ctx, id, func(q *db.Queries) error {
		if _, err := q.UnsuspendTenant(pctx(ctx), id); isNoRows(err) {
			if _, err := q.GetTenant(pctx(ctx), id); isNoRows(err) {
				return errTenantNotFound
			}
			return apperr.Conflict("TENANT_NOT_SUSPENDED", "The company is not suspended")
		} else if err != nil {
			return err
		}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "tenant.unsuspend", EntityType: "tenant", EntityID: &id}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "tenant.unsuspend", EntityType: "tenant", EntityID: &id, Tenant: &id})
	})
	if err != nil {
		return api.PlatformTenantDetail{}, err
	}
	s.o.Invalidate(s.hostsOf(ctx, id)...)
	return s.Tenant(ctx, id)
}

// ── Domains ────────────────────────────────────────────────────────────────

var errDomainNotFound = apperr.NotFound("Domain not found")

// verifyPrefix starts the TXT value that proves a domain's ownership.
const verifyPrefix = "messora-verify="

// AddDomain registers a custom domain for a company. It answers only once
// its TXT record _messora.<domain> holds the returned token (VerifyDomain).
func (s *Service) AddDomain(ctx context.Context, tenantID uuid.UUID, raw string) (api.PlatformDomain, error) {
	host := tenant.NormalizeHost(raw)
	if host == "" || !strings.Contains(host, ".") || net.ParseIP(host) != nil || strings.HasPrefix(host, ".") {
		return api.PlatformDomain{}, apperr.Validation("hostname", "Enter a domain name such as panel.example.com")
	}
	if base := s.o.Hosts.Base; base != "" && (host == base || strings.HasSuffix(host, "."+base)) {
		return api.PlatformDomain{}, apperr.Validation("hostname", "Addresses under the platform domain are the companies' own sub-domains")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return api.PlatformDomain{}, err
	}
	token := verifyPrefix + hex.EncodeToString(b)
	var d db.TenantDomain
	err := s.inTenant(ctx, tenantID, func(q *db.Queries) error {
		if _, err := q.GetTenant(pctx(ctx), tenantID); isNoRows(err) {
			return errTenantNotFound
		} else if err != nil {
			return err
		}
		var err error
		d, err = q.CreateTenantDomain(pctx(ctx), db.CreateTenantDomainParams{TenantID: tenantID, Hostname: host, Kind: "custom", VerificationToken: &token})
		if err != nil {
			if strings.Contains(err.Error(), "tenant_domains_hostname_key") {
				return apperr.Conflict("DOMAIN_TAKEN", "This domain is already in use")
			}
			return err
		}
		ch := map[string]any{"hostname": host}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "domain.add", EntityType: "tenant_domain", EntityID: &d.ID, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "domain.add", EntityType: "tenant_domain", EntityID: &d.ID, Tenant: &tenantID, Changes: ch})
	})
	if err != nil {
		return api.PlatformDomain{}, err
	}
	return s.domainView(d), nil
}

// VerifyDomain looks the domain's TXT record up and marks it verified when
// it holds the token.
func (s *Service) VerifyDomain(ctx context.Context, id uuid.UUID) (api.PlatformDomain, error) {
	d, err := s.q.GetTenantDomain(pctx(ctx), id)
	if isNoRows(err) {
		return api.PlatformDomain{}, errDomainNotFound
	}
	if err != nil {
		return api.PlatformDomain{}, err
	}
	if d.VerifiedAt != nil || d.VerificationToken == nil {
		return s.domainView(d), nil
	}
	lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	records, lerr := s.o.LookupTXT(lctx, "_messora."+d.Hostname)
	cancel()
	verified := false
	var problem *string
	for _, r := range records {
		if strings.TrimSpace(r) == *d.VerificationToken {
			verified = true
		}
	}
	if !verified {
		msg := "The TXT record _messora." + d.Hostname + " does not hold the token yet"
		if lerr != nil {
			msg = "No TXT record found at _messora." + d.Hostname
		}
		problem = &msg
	}
	err = s.inTenant(ctx, d.TenantID, func(q *db.Queries) error {
		var err error
		if d, err = q.RecordDomainCheck(pctx(ctx), db.RecordDomainCheckParams{ID: id, Error: problem, Verified: verified}); err != nil {
			return err
		}
		if !verified {
			return nil
		}
		ch := map[string]any{"hostname": d.Hostname}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "domain.verify", EntityType: "tenant_domain", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "domain.verify", EntityType: "tenant_domain", EntityID: &id, Tenant: &d.TenantID, Changes: ch})
	})
	if err != nil {
		return api.PlatformDomain{}, err
	}
	s.o.Invalidate(d.Hostname)
	return s.domainView(d), nil
}

// SetPrimaryDomain makes a verified domain the company's main address.
func (s *Service) SetPrimaryDomain(ctx context.Context, id uuid.UUID) (api.PlatformDomain, error) {
	d, err := s.q.GetTenantDomain(pctx(ctx), id)
	if isNoRows(err) {
		return api.PlatformDomain{}, errDomainNotFound
	}
	if err != nil {
		return api.PlatformDomain{}, err
	}
	if d.VerifiedAt == nil {
		return api.PlatformDomain{}, apperr.Conflict("DOMAIN_NOT_VERIFIED", "Verify the domain first")
	}
	err = s.inTenant(ctx, d.TenantID, func(q *db.Queries) error {
		if err := q.SetPrimaryDomain(pctx(ctx), db.SetPrimaryDomainParams{ID: id, TenantID: d.TenantID}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "domain.primary", EntityType: "tenant_domain", EntityID: &id, Tenant: &d.TenantID,
			Changes: map[string]any{"hostname": d.Hostname}})
	})
	if err != nil {
		return api.PlatformDomain{}, err
	}
	d.IsPrimary = true
	return s.domainView(d), nil
}

// DeleteDomain removes a custom domain (not the primary one).
func (s *Service) DeleteDomain(ctx context.Context, id uuid.UUID) error {
	d, err := s.q.GetTenantDomain(pctx(ctx), id)
	if isNoRows(err) {
		return errDomainNotFound
	}
	if err != nil {
		return err
	}
	if d.Kind != "custom" {
		return apperr.Conflict("DOMAIN_REQUIRED", "The company's own sub-domain cannot be removed")
	}
	if d.IsPrimary {
		return apperr.Conflict("DOMAIN_PRIMARY", "Make another domain the main address first")
	}
	err = s.inTenant(ctx, d.TenantID, func(q *db.Queries) error {
		if _, err := q.DeleteTenantDomain(pctx(ctx), id); err != nil {
			return err
		}
		ch := map[string]any{"hostname": d.Hostname}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "domain.remove", EntityType: "tenant_domain", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "domain.remove", EntityType: "tenant_domain", EntityID: &id, Tenant: &d.TenantID, Changes: ch})
	})
	if err == nil {
		s.o.Invalidate(d.Hostname)
	}
	return err
}

// ── Accounts ───────────────────────────────────────────────────────────────

func userView(u db.User, memberships int64) api.PlatformUser {
	return api.PlatformUser{Id: u.ID, Email: u.Email, Name: u.Name, Status: api.PlatformUserStatus(u.Status), IsPlatformAdmin: u.IsPlatformAdmin,
		MfaEnabled: u.TotpConfirmedAt != nil, EmailVerified: u.EmailVerifiedAt != nil, PasswordResetRequired: u.PasswordResetRequired,
		MembershipCount: memberships, LockedUntil: u.LockedUntil, LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt}
}

// SearchUsers finds accounts by address or name across every company.
func (s *Service) SearchUsers(ctx context.Context, query *string, cursor *api.Cursor, lim *api.Limit) ([]api.PlatformUser, string, error) {
	limit := int32(50)
	if lim != nil {
		limit = int32(*lim)
	}
	var after *uuid.UUID
	if cursor != nil && *cursor != "" {
		id, err := uuid.Parse(string(*cursor))
		if err != nil {
			return nil, "", apperr.Validation("cursor", "Invalid cursor")
		}
		after = &id
	}
	var q *string
	if query != nil && strings.TrimSpace(*query) != "" {
		v := strings.TrimSpace(*query)
		q = &v
	}
	rows, err := s.q.SearchUsers(pctx(ctx), db.SearchUsersParams{Query: q, After: after, Lim: limit + 1})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > int(limit) {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID.String()
	}
	out := make([]api.PlatformUser, len(rows))
	for i, r := range rows {
		out[i] = userView(db.User{ID: r.ID, Email: r.Email, Name: r.Name, Status: r.Status, TotpConfirmedAt: r.TotpConfirmedAt,
			LockedUntil: r.LockedUntil, LastLoginAt: r.LastLoginAt, CreatedAt: r.CreatedAt, EmailVerifiedAt: r.EmailVerifiedAt,
			IsPlatformAdmin: r.IsPlatformAdmin, PasswordResetRequired: r.PasswordResetRequired}, r.MembershipCount)
	}
	return out, next, nil
}

// User returns an account with its memberships.
func (s *Service) User(ctx context.Context, id uuid.UUID) (api.PlatformUserDetail, error) {
	ctx = pctx(ctx)
	u, err := s.q.GetUserByID(ctx, id)
	if isNoRows(err) {
		return api.PlatformUserDetail{}, apperr.NotFound("Account not found")
	}
	if err != nil {
		return api.PlatformUserDetail{}, err
	}
	ms, err := s.q.ListMembershipsOfUser(ctx, id)
	if err != nil {
		return api.PlatformUserDetail{}, err
	}
	out := api.PlatformUserDetail{User: userView(u, int64(len(ms)))}
	out.Memberships = make([]struct {
		CreatedAt time.Time                               `json:"created_at"`
		Role      string                                  `json:"role"`
		Status    api.PlatformUserDetailMembershipsStatus `json:"status"`
		Tenant    api.TenantRef                           `json:"tenant"`
	}, len(ms))
	for i, m := range ms {
		out.Memberships[i].CreatedAt, out.Memberships[i].Role = m.CreatedAt, m.RoleKey
		out.Memberships[i].Status = api.PlatformUserDetailMembershipsStatus(m.Status)
		out.Memberships[i].Tenant = api.TenantRef{Id: m.TenantID, Slug: m.Slug, Name: m.TenantName}
	}
	return out, nil
}

// SetUserStatus disables (ending every session) or enables an account.
func (s *Service) SetUserStatus(ctx context.Context, admin *principal.Principal, id uuid.UUID, status string) (api.PlatformUserDetail, error) {
	if id == admin.ID && status == "disabled" {
		return api.PlatformUserDetail{}, apperr.Forbidden("SELF_MODIFICATION", "You cannot disable your own account")
	}
	err := s.tx(ctx, func(q *db.Queries) error {
		u, err := q.GetUserForUpdate(pctx(ctx), id)
		if isNoRows(err) {
			return apperr.NotFound("Account not found")
		}
		if err != nil {
			return err
		}
		if err := q.SetUserStatus(pctx(ctx), db.SetUserStatusParams{ID: id, Status: status}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "user." + map[string]string{"disabled": "disable", "active": "enable"}[status],
			EntityType: "user", EntityID: &id, Changes: map[string]any{"email": u.Email, "status": []string{u.Status, status}}})
	})
	if err != nil {
		return api.PlatformUserDetail{}, err
	}
	if status == "disabled" {
		if err := s.id.RevokeEverywhere(ctx, id); err != nil {
			return api.PlatformUserDetail{}, err
		}
	}
	return s.User(ctx, id)
}

// ForcePasswordReset ends every session of the account and requires a new
// password; a reset link goes to the account's first workspace.
func (s *Service) ForcePasswordReset(ctx context.Context, id uuid.UUID) (api.PlatformUserDetail, error) {
	var first *uuid.UUID
	err := s.tx(ctx, func(q *db.Queries) error {
		u, err := q.GetUserForUpdate(pctx(ctx), id)
		if isNoRows(err) {
			return apperr.NotFound("Account not found")
		}
		if err != nil {
			return err
		}
		if err := q.RequirePasswordReset(pctx(ctx), id); err != nil {
			return err
		}
		ms, err := q.ListMembershipsOfUser(pctx(ctx), id)
		if err != nil {
			return err
		}
		for _, m := range ms {
			if m.Status == "active" && m.TenantStatus == tenant.StatusActive {
				tid := m.TenantID
				first = &tid
				break
			}
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "user.force_password_reset", EntityType: "user", EntityID: &id,
			Changes: map[string]any{"email": u.Email}})
	})
	if err != nil {
		return api.PlatformUserDetail{}, err
	}
	if err := s.id.RevokeEverywhere(ctx, id); err != nil {
		return api.PlatformUserDetail{}, err
	}
	if first != nil {
		if _, err := s.id.SendPasswordResetIn(ctx, *first, id); err != nil {
			s.o.Logger.ErrorContext(ctx, "forced password reset e-mail", "user_id", id, "err", err)
		}
	}
	return s.User(ctx, id)
}

// ── Support access ─────────────────────────────────────────────────────────

func grantView(g db.GetSupportGrantRow, now time.Time) api.SupportGrant {
	v := api.SupportGrant{Id: g.ID, Tenant: api.TenantRef{Id: g.TenantID, Slug: g.Slug, Name: g.TenantName}, Reason: g.Reason,
		ReadOnly: g.ReadOnly, CreatedAt: g.CreatedAt, ExpiresAt: g.ExpiresAt, EndedAt: g.EndedAt,
		Active: g.EndedAt == nil && now.Before(g.ExpiresAt)}
	v.Admin.Id, v.Admin.Name, v.Admin.Email = g.AdminID, g.AdminName, g.AdminEmail
	return v
}

// StartSupport opens a time-boxed support grant for a company and returns
// the link that opens its panel. Read-only unless write is set; the
// company's audit log records it, and its panel shows a banner throughout.
func (s *Service) StartSupport(ctx context.Context, admin *principal.Principal, tenantID uuid.UUID, req api.StartSupportSessionRequest) (api.SupportSessionStarted, error) {
	reason := strings.TrimSpace(req.Reason)
	if len(reason) < 3 {
		return api.SupportSessionStarted{}, apperr.Validation("reason", "Say why you open the company's panel")
	}
	minutes := 30
	if req.Minutes != nil {
		minutes = *req.Minutes
	}
	readOnly := req.Write == nil || !*req.Write
	t, err := s.q.GetTenant(pctx(ctx), tenantID)
	if isNoRows(err) {
		return api.SupportSessionStarted{}, errTenantNotFound
	}
	if err != nil {
		return api.SupportSessionStarted{}, err
	}
	if t.Status != tenant.StatusActive {
		return api.SupportSessionStarted{}, apperr.Conflict("TENANT_NOT_ACTIVE", "The company is suspended: its panel cannot be opened")
	}
	var g db.SupportAccessGrant
	err = s.inTenant(ctx, tenantID, func(q *db.Queries) error {
		var err error
		g, err = q.CreateSupportGrant(pctx(ctx), db.CreateSupportGrantParams{TenantID: tenantID, AdminID: admin.ID, Reason: reason,
			ReadOnly: readOnly, ExpiresAt: s.now().Add(time.Duration(minutes) * time.Minute)})
		if err != nil {
			return err
		}
		ch := map[string]any{"admin": admin.Email, "reason": reason, "read_only": readOnly, "minutes": minutes}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "support.grant", EntityType: "support_grant", EntityID: &g.ID, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "support.grant", EntityType: "support_grant", EntityID: &g.ID, Tenant: &tenantID, Changes: ch})
	})
	if err != nil {
		return api.SupportSessionStarted{}, err
	}
	link, err := s.id.SupportHandoff(ctx, admin.ID, tenantID, g.ID, readOnly)
	if err != nil {
		return api.SupportSessionStarted{}, err
	}
	full, err := s.q.GetSupportGrant(pctx(ctx), g.ID)
	if err != nil {
		return api.SupportSessionStarted{}, err
	}
	return api.SupportSessionStarted{Grant: grantView(full, s.now()), RedirectUrl: link}, nil
}

// ListGrants returns support grants, newest first (all companies, or one).
// On a tenant's host row-level security limits it to that tenant.
func (s *Service) ListGrants(ctx context.Context, tenantID *uuid.UUID) ([]api.SupportGrant, error) {
	rows, err := s.q.ListSupportGrants(ctx, db.ListSupportGrantsParams{TenantID: tenantID, Lim: 100})
	if err != nil {
		return nil, err
	}
	out := make([]api.SupportGrant, len(rows))
	for i, r := range rows {
		out[i] = grantView(db.GetSupportGrantRow(r), s.now())
	}
	return out, nil
}

// EndSupport closes a grant and its session at once.
func (s *Service) EndSupport(ctx context.Context, id uuid.UUID) error {
	g, err := s.q.GetSupportGrant(pctx(ctx), id)
	if isNoRows(err) {
		return apperr.NotFound("Support session not found")
	}
	if err != nil {
		return err
	}
	return s.inTenant(ctx, g.TenantID, func(q *db.Queries) error {
		if _, err := q.EndSupportGrant(pctx(ctx), id); err != nil && !isNoRows(err) {
			return err
		}
		if _, err := q.RevokeSupportSessions(pctx(ctx), &id); err != nil {
			return err
		}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "support.end", EntityType: "support_grant", EntityID: &id}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "support.end", EntityType: "support_grant", EntityID: &id, Tenant: &g.TenantID})
	})
}

// ── Settings and audit ─────────────────────────────────────────────────────

// Settings returns the platform settings.
func (s *Service) Settings(ctx context.Context) (api.PlatformSettings, error) {
	on, err := s.id.SignupEnabled(ctx)
	if err != nil {
		return api.PlatformSettings{}, err
	}
	out := api.PlatformSettings{SignupEnabled: on, BaseDomain: s.o.Hosts.Base, AdminUrl: strPtr(s.o.Hosts.AdminURL()), PlatformUrl: strPtr(s.o.Hosts.PlatformURL())}
	out.Email.Configured, out.Email.From, out.Email.Host = s.o.Sender.Configured, s.o.Sender.From, s.o.Sender.Host
	if raw, err := s.q.GetPlatformSetting(pctx(ctx), settingReplyTo); err == nil {
		var v string
		if json.Unmarshal(raw, &v) == nil && v != "" {
			out.Email.ReplyTo = &v
		}
	}
	return out, nil
}

// SettingsPatch is a partial change of the platform settings.
type SettingsPatch struct {
	SignupEnabled *bool
	SetReplyTo    bool
	ReplyTo       *string
}

// UpdateSettings changes platform settings.
func (s *Service) UpdateSettings(ctx context.Context, p SettingsPatch) (api.PlatformSettings, error) {
	err := s.tx(ctx, func(q *db.Queries) error {
		changes := map[string]any{}
		if p.SignupEnabled != nil {
			b, _ := json.Marshal(*p.SignupEnabled)
			if err := q.UpsertPlatformSetting(pctx(ctx), db.UpsertPlatformSettingParams{Key: identity.SettingSignupEnabled, Value: b}); err != nil {
				return err
			}
			changes["signup_enabled"] = *p.SignupEnabled
		}
		if p.SetReplyTo {
			if p.ReplyTo == nil || *p.ReplyTo == "" {
				if err := q.DeletePlatformSetting(pctx(ctx), settingReplyTo); err != nil {
					return err
				}
				changes["reply_to"] = nil
			} else {
				b, _ := json.Marshal(*p.ReplyTo)
				if err := q.UpsertPlatformSetting(pctx(ctx), db.UpsertPlatformSettingParams{Key: settingReplyTo, Value: b}); err != nil {
					return err
				}
				changes["reply_to"] = *p.ReplyTo
			}
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "platform.settings_update", EntityType: "platform_settings", Changes: changes})
	})
	if err != nil {
		return api.PlatformSettings{}, err
	}
	return s.Settings(ctx)
}

// AuditLogs lists platform audit entries, newest first.
func (s *Service) AuditLogs(ctx context.Context, tenantID *uuid.UUID, cursor *api.Cursor, lim *api.Limit) ([]api.PlatformAuditLog, string, error) {
	limit := int32(50)
	if lim != nil {
		limit = int32(*lim)
	}
	var after *uuid.UUID
	if cursor != nil && *cursor != "" {
		id, err := uuid.Parse(string(*cursor))
		if err != nil {
			return nil, "", apperr.Validation("cursor", "Invalid cursor")
		}
		after = &id
	}
	rows, err := s.q.ListPlatformAuditLogs(pctx(ctx), db.ListPlatformAuditLogsParams{TenantID: tenantID, After: after, Lim: limit + 1})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > int(limit) {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID.String()
	}
	out := make([]api.PlatformAuditLog, len(rows))
	for i, r := range rows {
		e := api.PlatformAuditLog{Id: r.ID, OccurredAt: r.OccurredAt, ActorId: r.ActorID, ActorEmail: r.ActorEmail, Action: r.Action,
			EntityType: r.EntityType, EntityId: r.EntityID, Changes: map[string]any{}}
		_ = json.Unmarshal(r.Changes, &e.Changes)
		if r.Ip != nil {
			ip := r.Ip.String()
			e.Ip = &ip
		}
		if r.TenantID != nil && r.TenantSlug != nil {
			e.Tenant = &struct {
				Id   uuid.UUID `json:"id"`
				Slug string    `json:"slug"`
			}{Id: *r.TenantID, Slug: *r.TenantSlug}
		}
		out[i] = e
	}
	return out, next, nil
}
