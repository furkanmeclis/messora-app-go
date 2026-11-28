package billing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// ── Invoice details (the buyer) ────────────────────────────────────────────

// validTCKN checks a Turkish identity number's two check digits.
func validTCKN(v string) bool {
	if len(v) != 11 || v[0] == '0' {
		return false
	}
	d := make([]int, 11)
	for i := range v {
		if v[i] < '0' || v[i] > '9' {
			return false
		}
		d[i] = int(v[i] - '0')
	}
	odd := d[0] + d[2] + d[4] + d[6] + d[8]
	even := d[1] + d[3] + d[5] + d[7]
	if ((odd*7-even)%10+10)%10 != d[9] {
		return false
	}
	sum := 0
	for _, x := range d[:10] {
		sum += x
	}
	return sum%10 == d[10]
}

func digits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// SetProfile stores the context tenant's invoice details.
func (s *Service) SetProfile(ctx context.Context, actor *principal.Principal, in api.BillingProfileInput) (api.BillingProfile, error) {
	p := db.UpsertBillingProfileParams{Kind: string(in.Kind), Title: strings.TrimSpace(in.Title), TaxNumber: strings.TrimSpace(in.TaxNumber),
		Address: strings.TrimSpace(in.Address), Email: strings.TrimSpace(string(in.Email)), Country: "TR", UpdatedBy: actorID(actor)}
	p.TaxOffice, p.City, p.Phone = strings.TrimSpace(deref(in.TaxOffice, "")), strings.TrimSpace(deref(in.City, "")), strings.TrimSpace(deref(in.Phone, ""))
	if in.Country != nil {
		p.Country = *in.Country
	}
	switch in.Kind {
	case api.BillingProfileInputKindCompany:
		if !digits(p.TaxNumber, 10) {
			return api.BillingProfile{}, apperr.Validation("tax_number", "A company's tax number (VKN) has 10 digits.")
		}
		if p.TaxOffice == "" {
			return api.BillingProfile{}, apperr.Validation("tax_office", "Enter the tax office.")
		}
	case api.BillingProfileInputKindIndividual:
		if !validTCKN(p.TaxNumber) {
			return api.BillingProfile{}, apperr.Validation("tax_number", "Enter a valid 11-digit Turkish identity number (TCKN).")
		}
	default:
		return api.BillingProfile{}, apperr.Validation("kind", "Choose company or individual.")
	}
	if len([]rune(p.Title)) < 2 {
		return api.BillingProfile{}, apperr.Validation("title", "Enter the company title or the full name.")
	}
	if len([]rune(p.Address)) < 5 {
		return api.BillingProfile{}, apperr.Validation("address", "Enter the invoice address.")
	}
	if a, err := mail.ParseAddress(p.Email); err != nil || a.Address != p.Email {
		return api.BillingProfile{}, apperr.Validation("email", "Enter the e-mail address invoices go to.")
	}
	var out db.BillingProfile
	err := s.tenantTx(ctx, tenant.ID(ctx), func(ctx context.Context, q *db.Queries, _ *effects) error {
		var err error
		if out, err = q.UpsertBillingProfile(ctx, p); err != nil {
			return err
		}
		return audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.profile_update", EntityType: "billing_profile",
			Changes: map[string]any{"kind": p.Kind, "title": p.Title, "tax_office": p.TaxOffice, "email": p.Email}})
	})
	return profileToAPI(out), err
}

// ── Settings ───────────────────────────────────────────────────────────────

// PlatformSettings returns the seller and the pricing.
func (s *Service) PlatformSettings(ctx context.Context) (api.PlatformBillingSettings, error) {
	seller, pricing, err := ReadSettings(ctx, s.q)
	if err != nil {
		return api.PlatformBillingSettings{}, err
	}
	return api.PlatformBillingSettings{Seller: seller, Pricing: pricing.API}, nil
}

// UpdatePlatformSettings replaces the seller and/or the pricing.
func (s *Service) UpdatePlatformSettings(ctx context.Context, req api.UpdatePlatformBillingSettingsRequest) (api.PlatformBillingSettings, error) {
	err := s.platformTx(ctx, func(ctx context.Context, q *db.Queries, _ *effects) error {
		changes := map[string]any{}
		if req.Seller != nil {
			sel, err := ValidateSeller(*req.Seller)
			if err != nil {
				return err
			}
			if err := saveSetting(ctx, q, SellerKey, sel); err != nil {
				return err
			}
			ibans := []string{}
			for _, b := range sel.BankAccounts {
				ibans = append(ibans, b.Iban)
			}
			changes["seller"] = map[string]any{"company_name": sel.CompanyName, "tax_number": sel.TaxNumber, "ibans": ibans}
		}
		if req.Pricing != nil {
			_, cur, err := ReadSettings(ctx, q)
			if err != nil {
				return err
			}
			in := *req.Pricing
			p, err := ParsePricing(in)
			if err != nil {
				return err
			}
			in.FxUpdatedAt = cur.API.FxUpdatedAt
			if cur.FX.String() != p.FX.String() || in.FxUpdatedAt == nil {
				now := s.now()
				in.FxUpdatedAt = &now
			}
			if err := saveSetting(ctx, q, PricingKey, in); err != nil {
				return err
			}
			changes["pricing"] = in
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.billing_settings_update", EntityType: "platform_settings", Changes: changes})
	})
	if err != nil {
		return api.PlatformBillingSettings{}, err
	}
	s.InvalidateSettings()
	return s.PlatformSettings(ctx)
}

// FXSource is where the USD → TL rate comes from. The launch source is
// the platform admin's daily entry (pricing.fx_usd_try); a TCMB (Central
// Bank) fetcher can implement it later and call SetFXRate.
type FXSource interface {
	Rate(ctx context.Context, day time.Time) (Rate, error)
}

// SetFXRate stores a new USD → TL rate (the hook for an automatic source).
func (s *Service) SetFXRate(ctx context.Context, r Rate) error {
	_, pricing, err := ReadSettings(ctx, s.q)
	if err != nil {
		return err
	}
	in := pricing.API
	in.FxUsdTry = r.String()
	_, err = s.UpdatePlatformSettings(ctx, api.UpdatePlatformBillingSettingsRequest{Pricing: &in})
	return err
}

// ── Lists and numbers ──────────────────────────────────────────────────────

func rowMonthly(r db.ListBillingSubscriptionsRow) int64 {
	cycle := deref(r.BillingCycle, CycleMonthly)
	if r.CustomPriceKurus != nil {
		return monthlyValue(*r.CustomPriceKurus, cycle)
	}
	if cycle == CycleYearly {
		if y, err := ParseKurus(r.PlanPriceYearly); err == nil {
			return monthlyValue(y, cycle)
		}
		if m, err := ParseKurus(r.PlanPriceMonthly); err == nil {
			return monthlyValue(m*10, cycle)
		}
		return 0
	}
	m, _ := ParseKurus(r.PlanPriceMonthly)
	return max(m, 0)
}

// paying reports whether a subscription brings recurring revenue: a paid
// period that has not lapsed.
func paying(r db.ListBillingSubscriptionsRow) bool {
	return (r.SubscriptionStatus == StatusActive || r.SubscriptionStatus == StatusPastDue) && r.PeriodEnd != nil
}

func subscriptionRowToAPI(r db.ListBillingSubscriptionsRow) api.PlatformSubscription {
	out := api.PlatformSubscription{Tenant: api.TenantRef{Id: r.ID, Slug: r.Slug, Name: r.Name}, Status: api.SubscriptionStatus(r.SubscriptionStatus),
		PlanKey: r.PlanKey, PlanNameTr: r.PlanNameTr, PlanNameEn: r.PlanNameEn, BillingCycle: cyclePtr(r.BillingCycle), PeriodEnd: r.PeriodEnd,
		GraceEndsAt: r.GraceEndsAt, ReadOnlySince: r.ReadOnlySince, TrialEndsAt: r.TrialEndsAt, StatusReason: deref(r.StatusReason, ""),
		OpenAmount: FormatKurus(r.OpenKurus), AccountCredit: FormatKurus(0), MonthlyValue: FormatKurus(0)}
	if r.AccountCreditKurus != nil {
		out.AccountCredit = FormatKurus(*r.AccountCreditKurus)
	}
	if paying(r) {
		out.MonthlyValue = FormatKurus(rowMonthly(r))
	}
	return out
}

// Subscriptions lists every company's subscription.
func (s *Service) Subscriptions(ctx context.Context, status *string) ([]api.PlatformSubscription, error) {
	rows, err := s.q.ListBillingSubscriptions(tenant.Platform(ctx), db.ListBillingSubscriptionsParams{Status: status, Lim: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]api.PlatformSubscription, len(rows))
	for i, r := range rows {
		out[i] = subscriptionRowToAPI(r)
	}
	return out, nil
}

func monthStart(t time.Time) time.Time {
	t = t.In(istanbul)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// PlatformOverview is the MRR and collections overview.
func (s *Service) PlatformOverview(ctx context.Context) (api.PlatformBillingOverview, error) {
	pctx := tenant.Platform(ctx)
	rows, err := s.q.ListBillingSubscriptions(pctx, db.ListBillingSubscriptionsParams{Lim: 100000})
	if err != nil {
		return api.PlatformBillingOverview{}, err
	}
	var mrr, open int64
	var out api.PlatformBillingOverview
	for _, r := range rows {
		switch r.SubscriptionStatus {
		case StatusTrial:
			out.Counts.Trial++
		case StatusActive:
			out.Counts.Active++
		case StatusPastDue:
			out.Counts.PastDue++
		case StatusReadOnly:
			out.Counts.ReadOnly++
		case StatusCanceled:
			out.Counts.Canceled++
		}
		if paying(r) {
			mrr += rowMonthly(r)
		}
		open += r.OpenKurus
	}
	this := monthStart(s.now())
	last := this.AddDate(0, -1, 0)
	collected := func(from, to time.Time) (int64, error) {
		return s.q.CollectedBetween(pctx, db.CollectedBetweenParams{FromDay: dateOnly(from), ToDay: dateOnly(to)})
	}
	thisMonth, err := collected(this, this.AddDate(0, 1, 0))
	if err != nil {
		return out, err
	}
	lastMonth, err := collected(last, this)
	if err != nil {
		return out, err
	}
	if out.UnmatchedTransfers, err = s.q.CountUnmatchedTransfers(pctx); err != nil {
		return out, err
	}
	if out.PendingInvoices, err = s.q.CountPendingInvoices(pctx); err != nil {
		return out, err
	}
	out.Mrr, out.Arr = FormatKurus(mrr), FormatKurus(mrr*12)
	out.CollectedThisMonth, out.CollectedLastMonth, out.OpenAmount = FormatKurus(thisMonth), FormatKurus(lastMonth), FormatKurus(open)
	return out, nil
}

// PlatformProformas lists proformas of every company.
func (s *Service) PlatformProformas(ctx context.Context, status *string, tenantID *uuid.UUID) ([]api.Proforma, error) {
	rows, err := s.q.ListPlatformProformas(tenant.Platform(ctx), db.ListPlatformProformasParams{Status: status, TenantID: tenantID, Lim: 500})
	if err != nil {
		return nil, err
	}
	out := make([]api.Proforma, len(rows))
	for i, r := range rows {
		v := ProformaToAPI(db.Proforma{ID: r.ID, TenantID: r.TenantID, Number: r.Number, Reference: r.Reference, Kind: r.Kind, Status: r.Status,
			Renewal: r.Renewal, Lines: r.Lines, NetKurus: r.NetKurus, VatRateBp: r.VatRateBp, VatKurus: r.VatKurus, TotalKurus: r.TotalKurus,
			PaidKurus: r.PaidKurus, Currency: r.Currency, PlanKey: r.PlanKey, BillingCycle: r.BillingCycle, PeriodStart: r.PeriodStart,
			PeriodEnd: r.PeriodEnd, AiCredits: r.AiCredits, PackKey: r.PackKey, MessageAmountKurus: r.MessageAmountKurus, Buyer: r.Buyer,
			Seller: r.Seller, DueAt: r.DueAt, PaidAt: r.PaidAt, FulfilledAt: r.FulfilledAt, VoidedAt: r.VoidedAt, VoidReason: r.VoidReason,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
		v.Tenant = &api.TenantRef{Id: r.TenantID, Slug: r.TenantSlug, Name: r.TenantName}
		out[i] = v
	}
	return out, nil
}

// PlatformProforma is any company's proforma with its company.
func (s *Service) PlatformProforma(ctx context.Context, id uuid.UUID) (*db.Proforma, error) {
	f, err := s.q.GetPlatformProforma(tenant.Platform(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ProformaWithTenant renders a proforma with its company (platform views).
func (s *Service) ProformaWithTenant(ctx context.Context, f db.Proforma) api.Proforma {
	v := ProformaToAPI(f)
	if t, err := s.q.GetTenant(tenant.System(ctx), f.TenantID); err == nil {
		v.Tenant = &api.TenantRef{Id: t.ID, Slug: t.Slug, Name: t.Name}
	}
	return v
}

// PlatformInvoices lists invoice records of every company.
func (s *Service) PlatformInvoices(ctx context.Context, status *string, tenantID *uuid.UUID) ([]api.Invoice, error) {
	rows, err := s.q.ListPlatformInvoices(tenant.Platform(ctx), db.ListPlatformInvoicesParams{Status: status, TenantID: tenantID, Lim: 500})
	if err != nil {
		return nil, err
	}
	out := make([]api.Invoice, len(rows))
	for i, r := range rows {
		v := invoiceToAPI(db.Invoice{ID: r.ID, TenantID: r.TenantID, ProformaID: r.ProformaID, Issuer: r.Issuer, Status: r.Status,
			OfficialNumber: r.OfficialNumber, IssuedOn: r.IssuedOn, PdfKey: r.PdfKey, NetKurus: r.NetKurus, VatKurus: r.VatKurus,
			TotalKurus: r.TotalKurus, CreatedAt: r.CreatedAt}, r.ProformaNumber, r.ProformaKind)
		v.Tenant = &api.TenantRef{Id: r.TenantID, Slug: r.TenantSlug, Name: r.TenantName}
		out[i] = v
	}
	return out, nil
}

func (s *Service) platformInvoice(ctx context.Context, id uuid.UUID) (db.Invoice, error) {
	inv, err := s.q.GetPlatformInvoice(tenant.Platform(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return inv, apperr.NotFound("Invoice not found.")
	}
	return inv, err
}

func (s *Service) invoiceView(ctx context.Context, inv db.Invoice) api.Invoice {
	number, kind := "", ""
	if f, err := s.q.GetPlatformProforma(tenant.Platform(ctx), inv.ProformaID); err == nil {
		number, kind = f.Number, f.Kind
	}
	v := invoiceToAPI(inv, number, kind)
	if t, err := s.q.GetTenant(tenant.System(ctx), inv.TenantID); err == nil {
		v.Tenant = &api.TenantRef{Id: t.ID, Slug: t.Slug, Name: t.Name}
	}
	return v
}

// RecordInvoice stores the official invoice number of an invoice record.
func (s *Service) RecordInvoice(ctx context.Context, id uuid.UUID, number string, issuedOn openapi_types.Date) (api.Invoice, error) {
	inv, err := s.platformInvoice(ctx, id)
	if err != nil {
		return api.Invoice{}, err
	}
	number = strings.TrimSpace(number)
	if number == "" {
		return api.Invoice{}, apperr.Validation("official_number", "Enter the official invoice number.")
	}
	err = s.tenantTx(ctx, inv.TenantID, func(ctx context.Context, q *db.Queries, _ *effects) error {
		var err error
		if inv, err = q.SetInvoiceOfficial(ctx, db.SetInvoiceOfficialParams{ID: id, OfficialNumber: &number, IssuedOn: dateOnly(issuedOn.Time),
			IssuedBy: actorID(principal.From(ctx))}); err != nil {
			return err
		}
		ch := map[string]any{"official_number": number, "issued_on": issuedOn.Format(time.DateOnly)}
		if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.invoice_issued", EntityType: "invoice", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "tenant.invoice_record", EntityType: "invoice", EntityID: &id, Tenant: &inv.TenantID, Changes: ch})
	})
	if err != nil {
		return api.Invoice{}, err
	}
	return s.invoiceView(ctx, inv), nil
}

// MaxInvoicePDF bounds an uploaded invoice PDF.
const MaxInvoicePDF = 5 << 20

// UploadInvoicePDF attaches the official invoice PDF.
func (s *Service) UploadInvoicePDF(ctx context.Context, id uuid.UUID, body io.Reader) (api.Invoice, error) {
	if s.o.Store == nil {
		return api.Invoice{}, apperr.Unavailable("STORAGE_UNAVAILABLE", "File storage is not available.")
	}
	inv, err := s.platformInvoice(ctx, id)
	if err != nil {
		return api.Invoice{}, err
	}
	raw, err := io.ReadAll(io.LimitReader(body, MaxInvoicePDF+1))
	if err != nil {
		return api.Invoice{}, err
	}
	if len(raw) > MaxInvoicePDF {
		return api.Invoice{}, apperr.Validation("", "The PDF is larger than 5 MB.")
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-")) {
		return api.Invoice{}, apperr.Validation("", "This is not a PDF file.")
	}
	key, err := storage.Key(inv.TenantID.String(), "invoices", "pdf")
	if err != nil {
		return api.Invoice{}, err
	}
	sctx := tenant.WithID(ctx, inv.TenantID)
	if err := s.o.Store.Put(sctx, key, bytes.NewReader(raw), int64(len(raw)), "application/pdf", storage.PutOptions{}); err != nil {
		return api.Invoice{}, err
	}
	err = s.tenantTx(ctx, inv.TenantID, func(ctx context.Context, q *db.Queries, _ *effects) error {
		var err error
		if inv, err = q.SetInvoicePDF(ctx, db.SetInvoicePDFParams{ID: id, PdfKey: &key}); err != nil {
			return err
		}
		ch := map[string]any{"bytes": len(raw)}
		if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.invoice_pdf", EntityType: "invoice", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "tenant.invoice_pdf", EntityType: "invoice", EntityID: &id, Tenant: &inv.TenantID, Changes: ch})
	})
	if err != nil {
		return api.Invoice{}, err
	}
	return s.invoiceView(ctx, inv), nil
}

// InvoicePDF is the stored PDF's key of the context tenant's invoice
// (platform: any company's, with platform set).
func (s *Service) InvoicePDF(ctx context.Context, id uuid.UUID, platform bool) (string, uuid.UUID, error) {
	var inv db.Invoice
	var err error
	if platform {
		inv, err = s.platformInvoice(ctx, id)
	} else {
		inv, err = s.q.GetInvoice(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			err = apperr.NotFound("Invoice not found.")
		}
	}
	if err != nil {
		return "", uuid.Nil, err
	}
	if inv.PdfKey == nil {
		return "", inv.TenantID, apperr.Conflict("INVOICE_PDF_PENDING", "The official invoice PDF is not attached yet.")
	}
	return *inv.PdfKey, inv.TenantID, nil
}

// Store is the private store of invoice PDFs.
func (s *Service) Store() storage.Store { return s.o.Store }

// ── A company's billing (platform) ─────────────────────────────────────────

// TenantBilling is a company's billing detail.
func (s *Service) TenantBilling(ctx context.Context, id uuid.UUID) (api.PlatformTenantBilling, error) {
	t, err := s.q.GetTenant(tenant.System(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.PlatformTenantBilling{}, apperr.NotFound("Company not found.")
	}
	if err != nil {
		return api.PlatformTenantBilling{}, err
	}
	out := api.PlatformTenantBilling{Tenant: api.TenantRef{Id: t.ID, Slug: t.Slug, Name: t.Name}, Proformas: []api.Proforma{}, Invoices: []api.Invoice{},
		Payments: []api.BillingPayment{}, AccountEntries: []api.BillingAccountEntry{}}
	err = s.tenantTx(ctx, id, func(ctx context.Context, q *db.Queries, _ *effects) error {
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		if out.Subscription, err = s.subscriptionView(ctx, q, id, sub); err != nil {
			return err
		}
		if p, err := q.GetBillingProfile(ctx); err == nil {
			v := profileToAPI(p)
			out.Profile = &v
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		fs, err := q.ListProformas(ctx, 100)
		if err != nil {
			return err
		}
		for _, f := range fs {
			out.Proformas = append(out.Proformas, ProformaToAPI(f))
		}
		invs, err := q.ListInvoices(ctx, 100)
		if err != nil {
			return err
		}
		for _, r := range invs {
			out.Invoices = append(out.Invoices, invoiceToAPI(db.Invoice{ID: r.ID, TenantID: r.TenantID, ProformaID: r.ProformaID, Issuer: r.Issuer,
				Status: r.Status, OfficialNumber: r.OfficialNumber, IssuedOn: r.IssuedOn, PdfKey: r.PdfKey, NetKurus: r.NetKurus, VatKurus: r.VatKurus,
				TotalKurus: r.TotalKurus, CreatedAt: r.CreatedAt}, r.ProformaNumber, r.ProformaKind))
		}
		pays, err := q.ListBillingPayments(ctx, 100)
		if err != nil {
			return err
		}
		for _, p := range pays {
			out.Payments = append(out.Payments, api.BillingPayment{Id: p.ID, ProformaId: p.ProformaID, Source: api.BillingPaymentSource(p.Source),
				Amount: FormatKurus(p.AmountKurus), ReceivedOn: openapi_types.Date{Time: p.ReceivedOn.Time}, CreatedAt: p.CreatedAt})
		}
		entries, err := q.ListAccountEntries(ctx, 100)
		if err != nil {
			return err
		}
		for _, e := range entries {
			out.AccountEntries = append(out.AccountEntries, api.BillingAccountEntry{Id: e.ID, Amount: FormatKurus(e.AmountKurus),
				BalanceAfter: FormatKurus(e.BalanceAfter), Reason: api.BillingAccountEntryReason(e.Reason), Note: e.Note, CreatedAt: e.CreatedAt})
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	// The connection method is the company's (the application role reads
	// it under row-level security; the platform role cannot).
	out.MessageBalance, err = s.messageBalance(tenant.WithID(ctx, id), s.q)
	return out, err
}

// Action applies a platform admin's billing action to a company
// (docs/BILLING.md "Admin runbook"); every action needs a reason and is
// recorded in both audit logs.
func (s *Service) Action(ctx context.Context, admin *principal.Principal, id uuid.UUID, req api.PlatformBillingActionRequest) error {
	if _, err := s.q.GetTenant(tenant.System(ctx), id); errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("Company not found.")
	} else if err != nil {
		return err
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) < 3 {
		return apperr.Validation("reason", "Say why (3–500 characters).")
	}
	return s.tenantTx(ctx, id, func(ctx context.Context, q *db.Queries, fx *effects) error {
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		t, err := q.GetTenantForUpdate(ctx, id)
		if err != nil {
			return err
		}
		now := s.now()
		ch := map[string]any{"action": req.Action, "reason": reason}
		// extend moves the end of the paid period (from now for a
		// company without a live period) and brings the company back to
		// active.
		extend := func(to func(time.Time) time.Time) error {
			base := now
			if sub.PeriodEnd != nil && sub.PeriodEnd.After(now) {
				base = *sub.PeriodEnd
			}
			if sub.PeriodStart == nil || sub.PeriodEnd == nil || !sub.PeriodEnd.After(now) {
				sub.PeriodStart = &now
			}
			end := to(base)
			ch["period_end"] = []any{sub.PeriodEnd, end}
			sub.PeriodEnd, sub.GraceEndsAt, sub.ReadOnlySince, sub.StatusReason = &end, nil, nil, ""
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
			if t.SubscriptionStatus != StatusCanceled {
				return s.setStatus(ctx, q, id, t.SubscriptionStatus, StatusActive, reason, fx)
			}
			return nil
		}
		switch req.Action {
		case api.ExtendDays:
			if req.Days == nil || *req.Days < 1 {
				return apperr.Validation("days", "How many days?")
			}
			if err := extend(func(b time.Time) time.Time { return b.AddDate(0, 0, *req.Days) }); err != nil {
				return err
			}
			ch["days"] = *req.Days
		case api.CompMonths:
			if req.Months == nil || *req.Months < 1 {
				return apperr.Validation("months", "How many months?")
			}
			if err := extend(func(b time.Time) time.Time { return addMonths(b, *req.Months) }); err != nil {
				return err
			}
			ch["months"] = *req.Months
		case api.SetCycle:
			if req.Cycle == nil {
				return apperr.Validation("cycle", "Choose monthly or yearly.")
			}
			ch["billing_cycle"] = []string{sub.BillingCycle, string(*req.Cycle)}
			sub.BillingCycle, sub.PendingCycle = string(*req.Cycle), nil
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
		case api.SetStatus:
			if req.Status == nil {
				return apperr.Validation("status", "Choose active, read_only or canceled.")
			}
			to := string(*req.Status)
			switch to {
			case StatusActive:
				sub.GraceEndsAt, sub.ReadOnlySince, sub.StatusReason = nil, nil, ""
			case StatusReadOnly:
				sub.ReadOnlySince, sub.StatusReason = &now, "admin: "+truncate(reason, 400)
			case StatusCanceled:
				sub.StatusReason = "canceled: " + truncate(reason, 400)
			}
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
			if err := s.setStatus(ctx, q, id, t.SubscriptionStatus, to, reason, fx); err != nil {
				return err
			}
			fx.invalidate = append(fx.invalidate, id)
		case api.SetCustomPrice:
			var k *int64
			if req.Price != nil {
				v, err := ParseKurus(*req.Price)
				if err != nil || v < 0 {
					return apperr.Validation("price", "Enter a TL amount (VAT excluded).")
				}
				k = &v
			}
			ch["custom_price"] = []any{amountPtr(sub.CustomPriceKurus), amountPtr(k)}
			sub.CustomPriceKurus = k
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
		case api.AdjustMessageBalance:
			k, err := signedAmount(req.Amount)
			if err != nil {
				return err
			}
			if err := s.topUp(ctx, q, k*MicrosPerKurus, nil, reason, actorID(admin)); err != nil {
				return err
			}
			ch["amount"] = FormatKurus(k)
		case api.AdjustAccountCredit:
			k, err := signedAmount(req.Amount)
			if err != nil {
				return err
			}
			if err := s.addCredit(ctx, q, &sub, k, "adjustment", nil, nil, reason, actorID(admin)); err != nil {
				return err
			}
			if err := s.applyCredit(ctx, q, &sub, fx); err != nil {
				return err
			}
			ch["amount"] = FormatKurus(k)
		default:
			return apperr.Validation("action", fmt.Sprintf("Unknown action %q.", req.Action))
		}
		if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.admin_" + string(req.Action), EntityType: "subscription", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "tenant.billing_" + string(req.Action), EntityType: "tenant", EntityID: &id, Tenant: &id, Changes: ch})
	})
}

func signedAmount(a *api.Amount) (int64, error) {
	if a == nil {
		return 0, apperr.Validation("amount", "Enter the amount (negative to take away).")
	}
	k, err := ParseKurus(*a)
	if err != nil || k == 0 {
		return 0, apperr.Validation("amount", "Enter a TL amount other than zero.")
	}
	return k, nil
}
