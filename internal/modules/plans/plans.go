// Package plans is the plan catalogue and what a workspace sees of its
// plan: limits next to their usage, the AI credits, the plan comparison
// and plan change requests; and the platform admin panel's side: editing
// plans, assigning them (with a company's own limits), granting AI
// credits and working the requests (docs/ENTITLEMENTS.md, Phase 17 WP8).
//
// Enforcement is not here: every module checks the plan where the
// resource is created (entitlements.Check) or the feature used.
package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/credits"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Options wires the service.
type Options struct {
	// Plans returns the plan provider in effect (tests replace it).
	Plans func() entitlements.Provider
	// Invalidate drops cached plans after a change (DBProvider); nil
	// with no cache.
	Invalidate    func(ids ...uuid.UUID)
	InvalidateAll func()
	Credits       *credits.Service
	AI            *ai.Service
	Identity      *identity.Service
	Redis         goredis.UniversalClient
	Logger        *slog.Logger
}

// Service implements the plans module.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	o    Options
	now  func() time.Time
	// meBilling adds the billing banners' state to /me (Phase 17 WP9).
	meBilling func(ctx context.Context, id uuid.UUID) MeBilling
}

// MeBilling is the subscription state /me carries for the panel's banners.
type MeBilling struct {
	Status                  string
	ReadOnly                bool
	TrialEndsAt             *time.Time
	GraceEndsAt             *time.Time
	MessageBalanceExhausted bool
}

// SetMeBilling installs billing's /me summary.
func (s *Service) SetMeBilling(fn func(ctx context.Context, id uuid.UUID) MeBilling) { s.meBilling = fn }

// New returns the service.
func New(pool *pgxpool.Pool, o Options) *Service {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Invalidate == nil {
		o.Invalidate = func(...uuid.UUID) {}
	}
	if o.InvalidateAll == nil {
		o.InvalidateAll = func() {}
	}
	return &Service{pool: pool, q: db.New(pool), o: o, now: time.Now}
}

func (s *Service) plans() entitlements.Provider {
	if s.o.Plans == nil {
		return entitlements.Static(entitlements.Default())
	}
	return s.o.Plans()
}

func pctx(ctx context.Context) context.Context { return tenant.Platform(ctx) }

// ── Conversions ────────────────────────────────────────────────────────────

func limitsToAPI(p entitlements.Plan) api.PlanLimits {
	return api.PlanLimits{SessionRequestsPerMinute: int64(p.SessionRequestsPerMinute), ApiKeyRequestsPerMinute: int64(p.APIKeyRequestsPerMinute),
		ApiCallsPerMonth: p.APICallsPerMonth, WebhookEndpoints: int64(p.WebhookEndpoints), WebhookDeliveriesPerMinute: int64(p.WebhookDeliveriesPerMinute),
		Contacts: p.Contacts, Users: int64(p.Users), PhoneNumbers: int64(p.PhoneNumbers), AiAgents: int64(p.AIAgents),
		AutomationFlows: int64(p.AutomationFlows), AiCreditsPerMonth: p.AICreditsPerMonth, StorageBytes: p.StorageBytes,
		CampaignMessagesPerSecond: int64(p.CampaignMessagesPerSecond)}
}

func featuresToAPI(p entitlements.Plan) api.PlanFeatures {
	return api.PlanFeatures{CustomSmtp: p.Has(entitlements.CustomSMTP), ByoAiKeys: p.Has(entitlements.BYOAIKeys),
		CustomDomain: p.Has(entitlements.CustomDomain), PublicApi: p.Has(entitlements.PublicAPI), Webhooks: p.Has(entitlements.Webhooks),
		AiAgents: p.Has(entitlements.AIAgents)}
}

// catalogue builds the plan a catalogue row stands for (over the default).
func catalogue(limits, features []byte) entitlements.Plan {
	p, err := entitlements.FromCatalogue(entitlements.Default(), limits, features)
	if err != nil {
		return entitlements.Plan{Features: map[entitlements.Feature]bool{}}
	}
	return p
}

func price(n pgtype.Numeric) *api.Price {
	if !n.Valid {
		return nil
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return nil
	}
	s := fmt.Sprint(v)
	return &s
}

func catalogToAPI(r db.Plan, current string) api.CatalogPlan {
	p := catalogue(r.Limits, r.Features)
	return api.CatalogPlan{Key: r.Key, NameTr: r.NameTr, NameEn: r.NameEn, DescriptionTr: r.DescriptionTr, DescriptionEn: r.DescriptionEn,
		SortOrder: r.SortOrder, Limits: limitsToAPI(p), Features: featuresToAPI(p), Currency: r.Currency, PriceMonthly: price(r.PriceMonthly),
		PriceYearly: price(r.PriceYearly), CustomPricing: r.CustomPricing, Current: r.Key == current}
}

func platformPlanToAPI(r db.ListPlansRow) api.PlatformPlan {
	p := catalogue(r.Limits, r.Features)
	return api.PlatformPlan{Key: r.Key, NameTr: r.NameTr, NameEn: r.NameEn, DescriptionTr: r.DescriptionTr, DescriptionEn: r.DescriptionEn,
		SortOrder: r.SortOrder, Limits: limitsToAPI(p), Features: featuresToAPI(p), Currency: r.Currency, PriceMonthly: price(r.PriceMonthly),
		PriceYearly: price(r.PriceYearly), CustomPricing: r.CustomPricing, Active: r.Active, Public: r.Public, TenantCount: r.TenantCount,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func requestToAPI(r db.PlanChangeRequest) api.PlanChangeRequest {
	return api.PlanChangeRequest{Id: r.ID, CurrentPlan: r.CurrentPlan, RequestedPlan: r.RequestedPlan,
		BillingCycle: api.PlanChangeRequestBillingCycle(r.BillingCycle), Message: r.Message, Status: api.PlanChangeRequestStatus(r.Status),
		ResolutionNote: r.ResolutionNote, CreatedAt: r.CreatedAt, ResolvedAt: r.ResolvedAt}
}

func balanceToAPI(b credits.Balance) api.AICreditBalance {
	return api.AICreditBalance{MonthStart: b.MonthStart, ResetsAt: b.ResetsAt, Included: credits.Credits(b.Included),
		Granted: credits.Credits(b.Granted), Used: credits.Credits(b.Used), Remaining: credits.Credits(b.Remaining),
		UsedThisMonth: credits.Credits(b.UsedThisMonth), Unlimited: b.Unlimited, Exhausted: b.Exhausted()}
}

func lotToAPI(l credits.Lot) api.AICreditLot {
	out := api.AICreditLot{Id: l.ID, Source: api.AICreditLotSource(l.Source), Granted: credits.Credits(l.Granted), Used: credits.Credits(l.Used),
		Remaining: credits.Credits(l.Remaining), StartsAt: l.StartsAt, ExpiresAt: l.ExpiresAt, Note: l.Note, CreatedAt: l.CreatedAt}
	if l.PeriodStart.Valid {
		d := l.PeriodStart.Time
		out.PeriodStart = &openapi_types.Date{Time: d}
	}
	return out
}

// ── A workspace's plan and usage ───────────────────────────────────────────

// Overview is ctx's workspace's plan with its usage.
func (s *Service) Overview(ctx context.Context) (api.PlanOverview, error) {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return api.PlanOverview{}, apperr.NotFound("Workspace not found")
	}
	return s.overview(tenant.WithID(ctx, id), id)
}

// overview reads in the tenant's own context (the application role under
// row-level security), also for the platform panel.
func (s *Service) overview(ctx context.Context, id uuid.UUID) (api.PlanOverview, error) {
	plan, info, err := entitlements.DescribeWith(ctx, s.plans(), id)
	if err != nil {
		return api.PlanOverview{}, err
	}
	out := api.PlanOverview{Key: info.Key, NameTr: info.NameTR, NameEn: info.NameEN, SubscriptionStatus: api.SubscriptionStatus(info.SubscriptionStatus),
		TrialEndsAt: info.TrialEndsAt, Overridden: info.Overridden(), Limits: limitsToAPI(plan), Features: featuresToAPI(plan)}
	if info.Key == entitlements.DefaultKey || out.NameTr == "" {
		out.NameTr, out.NameEn = "Varsayılan", "Default"
	}
	if out.SubscriptionStatus == "" {
		t, err := s.q.GetTenant(tenant.System(ctx), id)
		if err != nil {
			return out, err
		}
		out.SubscriptionStatus, out.TrialEndsAt = api.SubscriptionStatus(t.SubscriptionStatus), t.TrialEndsAt
	}
	c, err := s.q.PlanUsageCounts(ctx)
	if err != nil {
		return out, err
	}
	contacts, err := s.q.CountContactsForPlan(ctx)
	if err != nil {
		return out, err
	}
	storage, err := s.q.StorageBytesUsed(ctx)
	if err != nil {
		return out, err
	}
	usage := func(r entitlements.Resource, used int64) api.PlanUsage {
		l := plan.Count(r)
		return api.PlanUsage{Resource: api.PlanResource(r), Used: used, Limit: l, Unlimited: l == entitlements.Unlimited,
			Over: l != entitlements.Unlimited && used > l}
	}
	out.Usage = []api.PlanUsage{
		usage(entitlements.Users, c.Users+c.PendingInvitations),
		usage(entitlements.PhoneNumbers, c.PhoneNumbers),
		usage(entitlements.AIAgentCount, c.AiAgents),
		usage(entitlements.AutomationFlows, c.AutomationFlows),
		usage(entitlements.WebhookEndpoints, c.WebhookEndpoints),
		usage(entitlements.Storage, storage),
		usage(entitlements.Contacts, contacts),
	}
	now := s.now().UTC()
	out.ApiCalls.MonthStart = credits.MonthStart(now)
	out.ApiCalls.Limit, out.ApiCalls.Unlimited = plan.APICallsPerMonth, plan.APICallsPerMonth == entitlements.Unlimited
	if s.o.Redis != nil {
		if n, err := s.o.Redis.Get(ctx, apiguard.QuotaKey(id, now)).Int64(); err == nil {
			out.ApiCalls.Used = n
		} else if !errors.Is(err, goredis.Nil) {
			s.o.Logger.WarnContext(ctx, "read the API call counter", "err", err)
		}
	}
	if s.o.Credits != nil {
		b, err := s.o.Credits.Balance(ctx)
		if err != nil {
			return out, err
		}
		out.AiCredits = balanceToAPI(b)
	}
	if r, err := s.q.GetOpenPlanChangeRequest(ctx); err == nil {
		v := requestToAPI(r)
		out.OpenRequest = &v
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	return out, nil
}

// MePlan is the plan summary of /me (nil when it cannot be read: the panel
// then shows everything and the API still checks).
func (s *Service) MePlan(ctx context.Context, id uuid.UUID) *api.MePlan {
	ctx = tenant.WithID(ctx, id)
	plan, info, err := entitlements.DescribeWith(ctx, s.plans(), id)
	if err != nil {
		s.o.Logger.WarnContext(ctx, "plan of /me", "err", err)
		return nil
	}
	out := &api.MePlan{Key: info.Key, NameTr: info.NameTR, NameEn: info.NameEN, Features: []api.PlanFeature{},
		SubscriptionStatus: api.SubscriptionStatus(info.SubscriptionStatus), TrialEndsAt: info.TrialEndsAt}
	if out.SubscriptionStatus == "" {
		out.SubscriptionStatus = api.SubscriptionStatusActive
	}
	if s.meBilling != nil {
		b := s.meBilling(ctx, id)
		out.SubscriptionStatus, out.ReadOnly, out.TrialEndsAt = api.SubscriptionStatus(b.Status), b.ReadOnly, b.TrialEndsAt
		out.GraceEndsAt, out.MessageBalanceExhausted = b.GraceEndsAt, b.MessageBalanceExhausted
	}
	if out.NameTr == "" {
		out.NameTr, out.NameEn = "Varsayılan", "Default"
	}
	for _, f := range plan.FeatureList() {
		out.Features = append(out.Features, api.PlanFeature(f))
	}
	if s.o.Credits != nil && s.o.AI != nil && s.o.AI.Enabled(ctx) {
		if b, err := s.o.Credits.Balance(ctx); err == nil {
			out.AiCreditsExhausted = b.Exhausted()
		}
	}
	return out
}

// Catalog lists the public, active plans for ctx's workspace.
func (s *Service) Catalog(ctx context.Context) ([]api.CatalogPlan, error) {
	rows, err := s.q.ListPublicPlans(ctx)
	if err != nil {
		return nil, err
	}
	current := ""
	if id, ok := tenant.FromCtx(ctx); ok {
		if _, info, err := entitlements.DescribeWith(ctx, s.plans(), id); err == nil {
			current = info.Key
		}
	}
	out := make([]api.CatalogPlan, len(rows))
	for i, r := range rows {
		out[i] = catalogToAPI(r, current)
	}
	return out, nil
}

func isUnique(err error, constraint string) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505" && (constraint == "" || pe.ConstraintName == constraint)
}

func (s *Service) tx(ctx context.Context, fn func(q *db.Queries) error) error {
	t, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = t.Rollback(ctx) }()
	if err := fn(s.q.WithTx(t)); err != nil {
		return err
	}
	return t.Commit(ctx)
}

// RequestChange records ctx's workspace's request for another plan and
// e-mails the platform admins.
func (s *Service) RequestChange(ctx context.Context, actor *principal.Principal, req api.CreatePlanChangeRequest) (api.PlanChangeRequest, error) {
	key := strings.TrimSpace(req.PlanKey)
	want, err := s.q.GetPlan(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!want.Active || !want.Public)) {
		return api.PlanChangeRequest{}, apperr.Validation("plan_key", "This plan cannot be requested")
	}
	if err != nil {
		return api.PlanChangeRequest{}, err
	}
	id := tenant.ID(ctx)
	_, info, err := entitlements.DescribeWith(ctx, s.plans(), id)
	if err != nil {
		return api.PlanChangeRequest{}, err
	}
	if info.Key == key && !info.Overridden() {
		return api.PlanChangeRequest{}, apperr.Validation("plan_key", "The workspace is on this plan already")
	}
	cycle := "monthly"
	if req.BillingCycle != nil {
		cycle = string(*req.BillingCycle)
	}
	msg := ""
	if req.Message != nil {
		msg = strings.TrimSpace(*req.Message)
	}
	var current *string
	if info.Key != entitlements.DefaultKey {
		current = &info.Key
	}
	var by *uuid.UUID
	if actor != nil && actor.Support == nil {
		by = &actor.ID
	}
	var row db.PlanChangeRequest
	err = s.tx(ctx, func(q *db.Queries) error {
		var err error
		row, err = q.InsertPlanChangeRequest(ctx, db.InsertPlanChangeRequestParams{CurrentPlan: current, RequestedPlan: key,
			BillingCycle: cycle, Message: msg, RequestedBy: by})
		if isUnique(err, "plan_change_requests_one_open") {
			return apperr.Conflict("PLAN_REQUEST_OPEN", "A plan change request is open already; Messora will get back to you")
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "plan.change_request", EntityType: "plan_change_request", EntityID: &row.ID,
			Changes: map[string]any{"current_plan": current, "requested_plan": key, "billing_cycle": cycle}})
	})
	if err != nil {
		return api.PlanChangeRequest{}, err
	}
	if s.o.Identity != nil {
		who := ""
		if actor != nil {
			who = actor.Name
			if u, err := s.q.GetUserByID(ctx, actor.ID); err == nil {
				who = strings.TrimSpace(u.Name + " <" + u.Email + ">")
			}
		}
		n := identity.PlanChangeNotice{RequestedPlan: [2]string{want.NameTr, want.NameEn}, BillingCycle: cycle, RequestedBy: who,
			Message: msg, RequestedAt: row.CreatedAt}
		if info.Key != entitlements.DefaultKey {
			n.CurrentPlan = [2]string{info.NameTR, info.NameEN}
		}
		s.o.Identity.NotifyPlanChangeRequest(context.WithoutCancel(ctx), n)
	}
	return requestToAPI(row), nil
}

// ── Platform: the catalogue ────────────────────────────────────────────────

// ListPlans returns every plan with the number of companies on it.
func (s *Service) ListPlans(ctx context.Context) ([]api.PlatformPlan, error) {
	rows, err := s.q.ListPlans(pctx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]api.PlatformPlan, len(rows))
	for i, r := range rows {
		out[i] = platformPlanToAPI(r)
	}
	return out, nil
}

func (s *Service) platformPlan(ctx context.Context, key string) (api.PlatformPlan, error) {
	rows, err := s.ListPlans(ctx)
	if err != nil {
		return api.PlatformPlan{}, err
	}
	for _, p := range rows {
		if p.Key == key {
			return p, nil
		}
	}
	return api.PlatformPlan{}, errPlanNotFound
}

var errPlanNotFound = apperr.NotFound("Plan not found")

// mergeLimits applies the limits present in in over raw (a JSON object).
func mergeLimits(raw []byte, in *api.PlanLimitsInput) ([]byte, error) {
	m := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	if in != nil {
		b, _ := json.Marshal(in)
		var add map[string]any
		_ = json.Unmarshal(b, &add)
		maps.Copy(m, add)
	}
	return json.Marshal(m)
}

// mergeFeatures applies the features present in in over raw; the result
// names every feature.
func mergeFeatures(raw []byte, in *api.PlanFeaturesInput) ([]byte, error) {
	m := map[string]bool{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	if in != nil {
		b, _ := json.Marshal(in)
		var add map[string]bool
		_ = json.Unmarshal(b, &add)
		maps.Copy(m, add)
	}
	for _, f := range entitlements.AllFeatures {
		if _, ok := m[string(f)]; !ok {
			m[string(f)] = false
		}
	}
	return json.Marshal(m)
}

func validNames(tr, en string) error {
	if strings.TrimSpace(tr) == "" || strings.TrimSpace(en) == "" {
		return apperr.Validation("name_tr", "Name the plan in Turkish and English")
	}
	return nil
}

// CreatePlan adds a plan to the catalogue.
func (s *Service) CreatePlan(ctx context.Context, in api.PlatformPlanInput) (api.PlatformPlan, error) {
	if err := validNames(in.NameTr, in.NameEn); err != nil {
		return api.PlatformPlan{}, err
	}
	limits, err := mergeLimits(nil, &in.Limits)
	if err != nil {
		return api.PlatformPlan{}, err
	}
	features, err := mergeFeatures(nil, &in.Features)
	if err != nil {
		return api.PlatformPlan{}, err
	}
	p := db.CreatePlanParams{Key: in.Key, NameTr: strings.TrimSpace(in.NameTr), NameEn: strings.TrimSpace(in.NameEn),
		DescriptionTr: deref(in.DescriptionTr), DescriptionEn: deref(in.DescriptionEn), SortOrder: derefInt(in.SortOrder), Limits: limits,
		Features: features, Currency: "TRY", PriceMonthly: in.PriceMonthly, PriceYearly: in.PriceYearly, CustomPricing: derefBool(in.CustomPricing, false),
		Active: derefBool(in.Active, true), Public: derefBool(in.Public, true)}
	if in.Currency != nil {
		p.Currency = *in.Currency
	}
	err = s.platformTx(ctx, func(q *db.Queries) error {
		if _, err := q.CreatePlan(pctx(ctx), p); err != nil {
			if isUnique(err, "") {
				return apperr.Conflict("PLAN_EXISTS", "A plan with this key exists already")
			}
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "plan.create", EntityType: "plan",
			Changes: map[string]any{"key": p.Key, "limits": json.RawMessage(limits), "features": json.RawMessage(features)}})
	})
	if err != nil {
		return api.PlatformPlan{}, err
	}
	s.o.InvalidateAll()
	return s.platformPlan(ctx, p.Key)
}

func (s *Service) platformTx(ctx context.Context, fn func(q *db.Queries) error) error {
	if s.o.Identity == nil {
		return s.tx(pctx(ctx), fn)
	}
	return s.o.Identity.PlatformTx(ctx, fn)
}

// UpdatePlan changes a plan; every company on it follows within the
// plan cache's 30 seconds (at once on this replica).
func (s *Service) UpdatePlan(ctx context.Context, key string, in api.PlatformPlanPatch) (api.PlatformPlan, error) {
	err := s.platformTx(ctx, func(q *db.Queries) error {
		cur, err := q.GetPlan(pctx(ctx), key)
		if errors.Is(err, pgx.ErrNoRows) {
			return errPlanNotFound
		}
		if err != nil {
			return err
		}
		limits, err := mergeLimits(cur.Limits, in.Limits)
		if err != nil {
			return err
		}
		features, err := mergeFeatures(cur.Features, in.Features)
		if err != nil {
			return err
		}
		p := db.UpdatePlanParams{Key: key, NameTr: cur.NameTr, NameEn: cur.NameEn, DescriptionTr: cur.DescriptionTr, DescriptionEn: cur.DescriptionEn,
			SortOrder: cur.SortOrder, Limits: limits, Features: features, Currency: cur.Currency, PriceMonthly: price(cur.PriceMonthly),
			PriceYearly: price(cur.PriceYearly), CustomPricing: cur.CustomPricing, Active: cur.Active, Public: cur.Public}
		setStr(&p.NameTr, in.NameTr)
		setStr(&p.NameEn, in.NameEn)
		setStr(&p.DescriptionTr, in.DescriptionTr)
		setStr(&p.DescriptionEn, in.DescriptionEn)
		setStr(&p.Currency, in.Currency)
		if in.SortOrder != nil {
			p.SortOrder = *in.SortOrder
		}
		if in.CustomPricing != nil {
			p.CustomPricing = *in.CustomPricing
		}
		if in.Active != nil {
			p.Active = *in.Active
		}
		if in.Public != nil {
			p.Public = *in.Public
		}
		// A price in the request (null clears it) replaces the stored one.
		if in.PriceMonthly != nil {
			p.PriceMonthly = in.PriceMonthly
		}
		if in.PriceYearly != nil {
			p.PriceYearly = in.PriceYearly
		}
		if err := validNames(p.NameTr, p.NameEn); err != nil {
			return err
		}
		if _, err := q.UpdatePlan(pctx(ctx), p); err != nil {
			return err
		}
		b, _ := json.Marshal(in)
		var changes map[string]any
		_ = json.Unmarshal(b, &changes)
		changes["key"] = key
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "plan.update", EntityType: "plan", Changes: changes})
	})
	if err != nil {
		return api.PlatformPlan{}, err
	}
	s.o.InvalidateAll()
	return s.platformPlan(ctx, key)
}

// ── Platform: a company's plan and credits ─────────────────────────────────

// TenantPlan is a company's plan, usage and credit lots for the panel.
func (s *Service) TenantPlan(ctx context.Context, id uuid.UUID) (api.PlatformTenantPlan, error) {
	t, err := s.q.GetTenant(pctx(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.PlatformTenantPlan{}, apperr.NotFound("Company not found")
	}
	if err != nil {
		return api.PlatformTenantPlan{}, err
	}
	tctx := tenant.WithID(ctx, id)
	ov, err := s.overview(tctx, id)
	if err != nil {
		return api.PlatformTenantPlan{}, err
	}
	out := api.PlatformTenantPlan{PlanKey: t.PlanKey, Overview: ov, Overrides: map[string]any{}, Lots: []api.AICreditLot{}}
	_ = json.Unmarshal(t.PlanOverrides, &out.Overrides)
	if s.o.Credits != nil {
		lots, err := s.o.Credits.Lots(tctx, nil, 100)
		if err != nil {
			return out, err
		}
		for _, l := range lots {
			out.Lots = append(out.Lots, lotToAPI(l))
		}
	}
	return out, nil
}

// validOverrides checks a company's own limits: only the keys of
// PlanLimitsInput (non-negative integers) and a "features" object of
// known features.
func validOverrides(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	known := map[string]bool{}
	b, _ := json.Marshal(api.PlanLimitsInput{AiAgents: ptr(int64(0)), AiCreditsPerMonth: ptr(int64(0)), ApiCallsPerMonth: ptr(int64(0)),
		ApiKeyRequestsPerMinute: ptr(int64(0)), AutomationFlows: ptr(int64(0)), CampaignMessagesPerSecond: ptr(int64(0)), Contacts: ptr(int64(0)),
		PhoneNumbers: ptr(int64(0)), SessionRequestsPerMinute: ptr(int64(0)), StorageBytes: ptr(int64(0)), Users: ptr(int64(0)),
		WebhookDeliveriesPerMinute: ptr(int64(0)), WebhookEndpoints: ptr(int64(0))})
	var all map[string]any
	_ = json.Unmarshal(b, &all)
	for k := range all {
		known[k] = true
	}
	for k, v := range m {
		if k == "features" {
			f, ok := v.(map[string]any)
			if !ok {
				return nil, apperr.Validation("overrides", `"features" is an object of feature: true/false`)
			}
			for fk, fv := range f {
				if _, ok := fv.(bool); !ok || !featureKnown(fk) {
					return nil, apperr.Validation("overrides", "Unknown feature or value: "+fk)
				}
			}
			continue
		}
		if !known[k] {
			return nil, apperr.Validation("overrides", "Unknown limit: "+k)
		}
		n, ok := v.(float64)
		if !ok || n < 0 || n != float64(int64(n)) {
			return nil, apperr.Validation("overrides", "A limit is a whole number ≥ 0 (0 = no limit): "+k)
		}
	}
	return json.Marshal(m)
}

func featureKnown(k string) bool {
	for _, f := range entitlements.AllFeatures {
		if string(f) == k {
			return true
		}
	}
	return false
}

// SetTenantPlan assigns a plan and the company's own limits. Nothing is
// deleted: what is over a lower plan's limits keeps working, only creating
// more is refused (docs/ENTITLEMENTS.md "Downgrades").
func (s *Service) SetTenantPlan(ctx context.Context, id uuid.UUID, req api.SetTenantPlanRequest) (api.PlatformTenantPlan, error) {
	var overrides map[string]any
	if req.Overrides != nil {
		overrides = *req.Overrides
	}
	raw, err := validOverrides(overrides)
	if err != nil {
		return api.PlatformTenantPlan{}, err
	}
	err = s.platformTx(ctx, func(q *db.Queries) error {
		return s.assign(ctx, q, id, req.PlanKey, raw, "")
	})
	if err != nil {
		return api.PlatformTenantPlan{}, err
	}
	s.o.Invalidate(id)
	return s.TenantPlan(ctx, id)
}

// assign sets a company's plan in a platform transaction q and writes both
// audit logs.
func (s *Service) assign(ctx context.Context, q *db.Queries, id uuid.UUID, key string, overrides []byte, via string) error {
	p, err := q.GetPlan(pctx(ctx), key)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Validation("plan_key", "Unknown plan")
	}
	if err != nil {
		return err
	}
	if !p.Active {
		return apperr.Validation("plan_key", "This plan is switched off; switch it on first")
	}
	before, err := q.GetTenantForUpdate(pctx(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("Company not found")
	}
	if err != nil {
		return err
	}
	if overrides == nil {
		overrides = before.PlanOverrides
	}
	if _, err := q.SetTenantPlan(pctx(ctx), db.SetTenantPlanParams{ID: id, PlanKey: &key, PlanOverrides: overrides}); err != nil {
		return err
	}
	changes := map[string]any{"plan": []any{before.PlanKey, key}, "overrides": json.RawMessage(overrides)}
	if via != "" {
		changes["via"] = via
	}
	if err := q.SetTenantLocal(pctx(ctx), id.String()); err != nil {
		return err
	}
	if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "plan.change", EntityType: "tenant", EntityID: &id, Changes: changes}); err != nil {
		return err
	}
	return audit.Record(pctx(ctx), q, audit.Entry{Action: "tenant.plan_change", EntityType: "tenant", EntityID: &id, Tenant: &id, Changes: changes})
}

// Grant adds AI credits to a company (credits.Grant), recorded in both
// audit logs.
func (s *Service) Grant(ctx context.Context, admin *principal.Principal, id uuid.UUID, req api.GrantAICreditsRequest) (api.AICreditLot, error) {
	if s.o.Credits == nil {
		return api.AICreditLot{}, apperr.Unavailable("CREDITS_UNAVAILABLE", "AI credits are not available")
	}
	if _, err := s.q.GetTenant(pctx(ctx), id); errors.Is(err, pgx.ErrNoRows) {
		return api.AICreditLot{}, apperr.NotFound("Company not found")
	} else if err != nil {
		return api.AICreditLot{}, err
	}
	var lot db.AiCreditLot
	err := s.platformTx(ctx, func(q *db.Queries) error {
		if err := q.SetTenantLocal(pctx(ctx), id.String()); err != nil {
			return err
		}
		var err error
		lot, err = s.o.Credits.Grant(pctx(ctx), q, credits.Grant{Credits: req.Credits, Source: "grant", ExpiresAt: req.ExpiresAt, Note: req.Note})
		var fe *credits.FieldError
		if errors.As(err, &fe) {
			return apperr.Validation(fe.Field, fe.Message)
		}
		if err != nil {
			return err
		}
		ch := map[string]any{"credits": req.Credits, "expires_at": req.ExpiresAt, "note": req.Note}
		if err := audit.RecordInTenant(pctx(ctx), q, audit.Entry{Action: "ai.credits_grant", EntityType: "ai_credit_lot", EntityID: &lot.ID, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "tenant.ai_credits_grant", EntityType: "ai_credit_lot", EntityID: &lot.ID, Tenant: &id, Changes: ch})
	})
	if err != nil {
		return api.AICreditLot{}, err
	}
	return lotToAPI(credits.Lot{AiCreditLot: lot, Remaining: lot.Granted - lot.Used}), nil
}

// ── Platform: requests ─────────────────────────────────────────────────────

func platformRequestToAPI(r db.ListPlanChangeRequestsRow) api.PlatformPlanRequest {
	base := requestToAPI(db.PlanChangeRequest{ID: r.ID, TenantID: r.TenantID, CurrentPlan: r.CurrentPlan, RequestedPlan: r.RequestedPlan,
		BillingCycle: r.BillingCycle, Message: r.Message, RequestedBy: r.RequestedBy, Status: r.Status, ResolutionNote: r.ResolutionNote,
		ResolvedBy: r.ResolvedBy, ResolvedAt: r.ResolvedAt, CreatedAt: r.CreatedAt})
	out := api.PlatformPlanRequest{Id: base.Id, CurrentPlan: base.CurrentPlan, RequestedPlan: base.RequestedPlan,
		BillingCycle: api.PlatformPlanRequestBillingCycle(base.BillingCycle), Message: base.Message, Status: api.PlatformPlanRequestStatus(base.Status),
		ResolutionNote: base.ResolutionNote, CreatedAt: base.CreatedAt, ResolvedAt: base.ResolvedAt,
		Tenant: api.TenantRef{Id: r.TenantID, Slug: r.TenantSlug, Name: r.TenantName}}
	if r.RequestedByEmail != nil {
		out.RequestedBy = &struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}{Email: *r.RequestedByEmail, Name: deref(r.RequestedByName)}
	}
	return out
}

// Requests lists every company's plan change requests (optionally one
// status), newest first.
func (s *Service) Requests(ctx context.Context, status *string) ([]api.PlatformPlanRequest, error) {
	rows, err := s.q.ListPlanChangeRequests(pctx(ctx), db.ListPlanChangeRequestsParams{Status: status, Lim: 200})
	if err != nil {
		return nil, err
	}
	out := make([]api.PlatformPlanRequest, len(rows))
	for i, r := range rows {
		out[i] = platformRequestToAPI(r)
	}
	return out, nil
}

// Resolve closes a request; with done and apply the requested plan is
// assigned in the same transaction.
func (s *Service) Resolve(ctx context.Context, admin *principal.Principal, id uuid.UUID, req api.ResolvePlanRequest) (api.PlatformPlanRequest, error) {
	note := strings.TrimSpace(deref(req.Note))
	apply := req.Apply != nil && *req.Apply && req.Status == api.ResolvePlanRequestStatusDone
	var tenantID uuid.UUID
	err := s.platformTx(ctx, func(q *db.Queries) error {
		cur, err := q.GetPlanChangeRequest(pctx(ctx), id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Request not found")
		}
		if err != nil {
			return err
		}
		tenantID = cur.TenantID
		var by *uuid.UUID
		if admin != nil {
			by = &admin.ID
		}
		r, err := q.ResolvePlanChangeRequest(pctx(ctx), db.ResolvePlanChangeRequestParams{ID: id, Status: string(req.Status), ResolutionNote: note, ResolvedBy: by})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("PLAN_REQUEST_CLOSED", "The request is closed already")
		}
		if err != nil {
			return err
		}
		if apply {
			if err := s.assign(ctx, q, r.TenantID, r.RequestedPlan, nil, "plan_change_request"); err != nil {
				return err
			}
		}
		return audit.Record(pctx(ctx), q, audit.Entry{Action: "platform.plan_request_resolve", EntityType: "plan_change_request", EntityID: &id,
			Tenant: &r.TenantID, Changes: map[string]any{"status": req.Status, "applied": apply, "note": note}})
	})
	if err != nil {
		return api.PlatformPlanRequest{}, err
	}
	if apply {
		s.o.Invalidate(tenantID)
	}
	rows, err := s.Requests(ctx, nil)
	if err != nil {
		return api.PlatformPlanRequest{}, err
	}
	for _, r := range rows {
		if r.Id == id {
			return r, nil
		}
	}
	return api.PlatformPlanRequest{}, apperr.NotFound("Request not found")
}

// ── helpers ────────────────────────────────────────────────────────────────

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefBool(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func setStr(dst *string, v *string) {
	if v != nil {
		*dst = strings.TrimSpace(*v)
	}
}

func ptr[T any](v T) *T { return &v }
