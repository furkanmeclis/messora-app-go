package billing

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

func amountPtr(k *int64) *api.Amount {
	if k == nil {
		return nil
	}
	v := FormatKurus(*k)
	return &v
}

func cyclePtr(c *string) *api.BillingCycle {
	if c == nil {
		return nil
	}
	v := api.BillingCycle(*c)
	return &v
}

func datePtr(d pgtype.Date) *openapi_types.Date {
	if !d.Valid {
		return nil
	}
	return &openapi_types.Date{Time: d.Time}
}

// ProformaToAPI renders a proforma.
func ProformaToAPI(f db.Proforma) api.Proforma {
	var lines []line
	_ = json.Unmarshal(f.Lines, &lines)
	out := api.Proforma{Id: f.ID, Number: f.Number, Reference: f.Reference, ReferenceDisplay: DisplayReference(f.Reference),
		Kind: api.ProformaKind(f.Kind), Status: api.ProformaStatus(f.Status), Renewal: f.Renewal, Lines: make([]api.ProformaLine, len(lines)),
		Net: FormatKurus(f.NetKurus), VatRatePercent: FormatRateBP(f.VatRateBp), Vat: FormatKurus(f.VatKurus), Total: FormatKurus(f.TotalKurus),
		Paid: FormatKurus(f.PaidKurus), Due: FormatKurus(f.TotalKurus - f.PaidKurus), Currency: f.Currency, PlanKey: f.PlanKey,
		BillingCycle: cyclePtr(f.BillingCycle), PeriodStart: f.PeriodStart, PeriodEnd: f.PeriodEnd, AiCredits: f.AiCredits,
		MessageAmount: amountPtr(f.MessageAmountKurus), Buyer: map[string]any{}, Seller: map[string]any{}, DueAt: f.DueAt, PaidAt: f.PaidAt,
		VoidedAt: f.VoidedAt, VoidReason: f.VoidReason, CreatedAt: f.CreatedAt}
	if f.Status == "void" || f.Status == "paid" {
		out.Due = FormatKurus(0)
	}
	for i, l := range lines {
		out.Lines[i] = api.ProformaLine{DescriptionTr: l.DescriptionTR, DescriptionEn: l.DescriptionEN, Quantity: l.Quantity,
			UnitPrice: FormatKurus(l.UnitKurus), Net: FormatKurus(l.NetKurus)}
	}
	_ = json.Unmarshal(f.Buyer, &out.Buyer)
	_ = json.Unmarshal(f.Seller, &out.Seller)
	return out
}

func invoiceToAPI(i db.Invoice, number, kind string) api.Invoice {
	return api.Invoice{Id: i.ID, ProformaId: i.ProformaID, ProformaNumber: number, ProformaKind: kind, Issuer: i.Issuer,
		Status: api.InvoiceStatus(i.Status), OfficialNumber: i.OfficialNumber, IssuedOn: datePtr(i.IssuedOn), HasPdf: i.PdfKey != nil,
		Net: FormatKurus(i.NetKurus), Vat: FormatKurus(i.VatKurus), Total: FormatKurus(i.TotalKurus), CreatedAt: i.CreatedAt}
}

func profileToAPI(p db.BillingProfile) api.BillingProfile {
	return api.BillingProfile{Kind: api.BillingProfileKind(p.Kind), Title: p.Title, TaxOffice: p.TaxOffice, TaxNumber: p.TaxNumber,
		Address: p.Address, City: p.City, Country: p.Country, Email: p.Email, Phone: p.Phone, UpdatedAt: p.UpdatedAt}
}

// subscriptionView renders the context tenant's subscription (q scoped to
// it).
func (s *Service) subscriptionView(ctx context.Context, q *db.Queries, id uuid.UUID, sub db.Subscription) (api.BillingSubscription, error) {
	t, err := q.GetTenant(tenant.System(ctx), id)
	if err != nil {
		return api.BillingSubscription{}, err
	}
	out := api.BillingSubscription{Status: api.SubscriptionStatus(t.SubscriptionStatus), Code: sub.Code, PlanKey: t.PlanKey,
		BillingCycle: api.BillingCycle(sub.BillingCycle), PeriodStart: sub.PeriodStart, PeriodEnd: sub.PeriodEnd, TrialEndsAt: t.TrialEndsAt,
		GraceEndsAt: sub.GraceEndsAt, ReadOnlySince: sub.ReadOnlySince, StatusReason: sub.StatusReason, PendingPlanKey: sub.PendingPlanKey,
		PendingCycle: cyclePtr(sub.PendingCycle), AccountCredit: FormatKurus(sub.AccountCreditKurus), CustomPrice: amountPtr(sub.CustomPriceKurus)}
	out.ReadOnly, _ = s.readOnly(ctx, id, s.now())
	if p, err := planOf(ctx, q, t.PlanKey); err != nil {
		return out, err
	} else if p != nil {
		out.PlanNameTr, out.PlanNameEn = p.NameTr, p.NameEn
		if price, ok := PlanPrice(*p, sub.BillingCycle, sub.CustomPriceKurus); ok {
			out.Price = amountPtr(&price)
		}
	}
	if sub.PeriodEnd != nil && (t.SubscriptionStatus == StatusActive || t.SubscriptionStatus == StatusPastDue) {
		_, pricing, err := s.settings(ctx)
		if err != nil {
			return out, err
		}
		at := sub.PeriodEnd.AddDate(0, 0, -pricing.API.RenewalLeadDays)
		out.NextRenewalAt = &at
	}
	return out, nil
}

// readTx runs fn in a read-only transaction of the tenant (the
// application role under row-level security).
func (s *Service) readTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return fn(s.q.WithTx(tx))
}

// Subscription is the context tenant's subscription.
func (s *Service) Subscription(ctx context.Context) (api.BillingSubscription, error) {
	id := tenant.ID(ctx)
	sub, err := s.Ensure(ctx)
	if err != nil {
		return api.BillingSubscription{}, err
	}
	return s.subscriptionView(ctx, s.q, id, sub)
}

func (s *Service) offer(ctx context.Context, q *db.Queries, currentKey *string) (api.BillingOffer, error) {
	_, pricing, err := s.settings(ctx)
	if err != nil {
		return api.BillingOffer{}, err
	}
	out := api.BillingOffer{VatRatePercent: FormatRateBP(pricing.VATRateBP), AiCreditPacks: pricing.API.AiCreditPacks,
		MessageTopups: pricing.API.MessageTopups, MinMessageTopup: pricing.API.MinMessageTopup, Plans: []api.BillingOfferPlan{}}
	if out.AiCreditPacks == nil {
		out.AiCreditPacks = []api.AICreditPack{}
	}
	if out.MessageTopups == nil {
		out.MessageTopups = []api.Amount{}
	}
	plans, err := q.ListPublicPlans(ctx)
	if err != nil {
		return out, err
	}
	for _, p := range plans {
		item := api.BillingOfferPlan{Key: p.Key, NameTr: p.NameTr, NameEn: p.NameEn, CustomPricing: p.CustomPricing,
			Current: currentKey != nil && *currentKey == p.Key}
		if m, ok := PlanPrice(p, CycleMonthly, nil); ok {
			item.Monthly = amountPtr(&m)
		}
		if y, ok := PlanPrice(p, CycleYearly, nil); ok {
			item.Yearly = amountPtr(&y)
		}
		out.Plans = append(out.Plans, item)
	}
	return out, nil
}

// Overview is the billing page of the context tenant.
func (s *Service) Overview(ctx context.Context) (api.BillingOverview, error) {
	id := tenant.ID(ctx)
	sub, err := s.Ensure(ctx)
	if err != nil {
		return api.BillingOverview{}, err
	}
	var out api.BillingOverview
	err = s.readTx(ctx, func(q *db.Queries) error {
		var err error
		if out.Subscription, err = s.subscriptionView(ctx, q, id, sub); err != nil {
			return err
		}
		if p, err := q.GetBillingProfile(ctx); err == nil {
			v := profileToAPI(p)
			out.Profile = &v
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		seller, _, err := s.settings(ctx)
		if err != nil {
			return err
		}
		out.Seller = seller
		if out.Offer, err = s.offer(ctx, q, out.Subscription.PlanKey); err != nil {
			return err
		}
		if s.o.Credits != nil {
			b, err := s.o.Credits.BalanceWith(ctx, q)
			if err != nil {
				return err
			}
			out.AiCredits = api.AICreditBalance{MonthStart: b.MonthStart, ResetsAt: b.ResetsAt, Included: float64(b.Included) / 1000,
				Granted: float64(b.Granted) / 1000, Used: float64(b.Used) / 1000, Remaining: float64(b.Remaining) / 1000,
				UsedThisMonth: float64(b.UsedThisMonth) / 1000, Unlimited: b.Unlimited, Exhausted: b.Exhausted()}
		}
		if out.MessageBalance, err = s.messageBalance(ctx, q); err != nil {
			return err
		}
		rows, err := q.ListProformas(ctx, 50)
		if err != nil {
			return err
		}
		out.OpenProformas = []api.Proforma{}
		for _, f := range rows {
			if f.Status == "open" || f.Status == "partially_paid" {
				out.OpenProformas = append(out.OpenProformas, ProformaToAPI(f))
			}
		}
		return nil
	})
	return out, err
}

// Proformas lists the context tenant's proformas.
func (s *Service) Proformas(ctx context.Context) ([]api.Proforma, error) {
	rows, err := s.q.ListProformas(ctx, 200)
	if err != nil {
		return nil, err
	}
	out := make([]api.Proforma, len(rows))
	for i, r := range rows {
		out[i] = ProformaToAPI(r)
	}
	return out, nil
}

// Proforma is one of the context tenant's proformas (nil: not found).
func (s *Service) Proforma(ctx context.Context, id uuid.UUID) (*db.Proforma, error) {
	f, err := s.q.GetProforma(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// Invoices lists the context tenant's invoice records.
func (s *Service) Invoices(ctx context.Context) ([]api.Invoice, error) {
	rows, err := s.q.ListInvoices(ctx, 200)
	if err != nil {
		return nil, err
	}
	out := make([]api.Invoice, len(rows))
	for i, r := range rows {
		out[i] = invoiceToAPI(db.Invoice{ID: r.ID, TenantID: r.TenantID, ProformaID: r.ProformaID, Issuer: r.Issuer, Status: r.Status,
			OfficialNumber: r.OfficialNumber, IssuedOn: r.IssuedOn, PdfKey: r.PdfKey, NetKurus: r.NetKurus, VatKurus: r.VatKurus,
			TotalKurus: r.TotalKurus, CreatedAt: r.CreatedAt}, r.ProformaNumber, r.ProformaKind)
	}
	return out, nil
}

// MeBilling is what /me tells the panel for its banners.
type MeBilling struct {
	Status                  string
	ReadOnly                bool
	TrialEndsAt             *time.Time
	GraceEndsAt             *time.Time
	MessageBalanceExhausted bool
}

// Me is the banner summary of tenant id.
func (s *Service) Me(ctx context.Context, id uuid.UUID) MeBilling {
	ctx = tenant.WithID(ctx, id)
	out := MeBilling{Status: StatusActive}
	t, err := s.q.GetTenant(tenant.System(ctx), id)
	if err != nil {
		return out
	}
	out.Status, out.TrialEndsAt = t.SubscriptionStatus, t.TrialEndsAt
	out.ReadOnly = s.ReadOnlyTenant(ctx, id)
	if sub, err := s.q.GetSubscription(ctx); err == nil {
		out.GraceEndsAt = sub.GraceEndsAt
	}
	out.MessageBalanceExhausted = s.MessageBalanceExhausted(ctx)
	return out
}
