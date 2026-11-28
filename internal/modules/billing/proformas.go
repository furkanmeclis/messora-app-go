package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/billing/invoicing"
	"github.com/furkanmeclis/messora-app-go/internal/modules/credits"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

func ratOf(a, b int64) *big.Rat { return big.NewRat(a, b) }

// line is a proforma line (stored in proformas.lines).
type line struct {
	DescriptionTR string `json:"description_tr"`
	DescriptionEN string `json:"description_en"`
	Quantity      int64  `json:"quantity"`
	UnitKurus     int64  `json:"unit_kurus"`
	NetKurus      int64  `json:"net_kurus"`
}

// party is the buyer or seller snapshot of a document.
type party struct {
	Name      string        `json:"name"`
	Kind      string        `json:"kind,omitempty"`
	TaxOffice string        `json:"tax_office"`
	TaxNumber string        `json:"tax_number"`
	Address   string        `json:"address"`
	City      string        `json:"city,omitempty"`
	Country   string        `json:"country,omitempty"`
	Email     string        `json:"email"`
	Phone     string        `json:"phone,omitempty"`
	Website   string        `json:"website,omitempty"`
	MersisNo  string        `json:"mersis_no,omitempty"`
	Note      string        `json:"note,omitempty"`
	Banks     []bankAccount `json:"bank_accounts,omitempty"`
}

type bankAccount struct {
	BankName      string `json:"bank_name"`
	AccountHolder string `json:"account_holder"`
	IBAN          string `json:"iban"`
}

func dateTR(t time.Time) string { return t.In(istanbul).Format("02.01.2006") }

func cycleWord(cycle string) (string, string) {
	if cycle == CycleYearly {
		return "yıllık", "yearly"
	}
	return "aylık", "monthly"
}

func planLine(p db.Plan, cycle string, price int64, start, end *time.Time) line {
	tr, en := cycleWord(cycle)
	l := line{DescriptionTR: fmt.Sprintf("Messora %s planı · %s", p.NameTr, tr), DescriptionEN: fmt.Sprintf("Messora %s plan · %s", p.NameEn, en),
		Quantity: 1, UnitKurus: price, NetKurus: price}
	if start != nil && end != nil {
		span := " · " + dateTR(*start) + " – " + dateTR(end.Add(-time.Second))
		l.DescriptionTR += span
		l.DescriptionEN += span
	}
	return l
}

func upgradeLine(cur *db.Plan, want db.Plan, cycle string, net int64, start, end time.Time) line {
	from, fromEN := "—", "—"
	if cur != nil {
		from, fromEN = cur.NameTr, cur.NameEn
	}
	span := dateTR(start) + " – " + dateTR(end.Add(-time.Second))
	return line{DescriptionTR: fmt.Sprintf("Plan yükseltme %s → %s · kalan süre %s", from, want.NameTr, span),
		DescriptionEN: fmt.Sprintf("Plan upgrade %s → %s · remaining period %s", fromEN, want.NameEn, span), Quantity: 1, UnitKurus: net, NetKurus: net}
}

func creditsGrant(n int64, exp time.Time, note string, ref *string) credits.Grant {
	return credits.Grant{Credits: n, Source: "purchase", ExpiresAt: &exp, Note: note, Reference: ref}
}

// draft is a proforma to issue.
type draft struct {
	kind                   string
	renewal                bool
	lines                  []line
	net                    int64
	planKey, cycle         *string
	periodStart, periodEnd *time.Time
	aiCredits              *int64
	packKey                *string
	messageAmount          *int64
	due                    *time.Time
	createdBy              *uuid.UUID
}

func buyerOf(p db.BillingProfile) party {
	return party{Name: p.Title, Kind: p.Kind, TaxOffice: p.TaxOffice, TaxNumber: p.TaxNumber, Address: p.Address, City: p.City,
		Country: p.Country, Email: p.Email, Phone: p.Phone}
}

func sellerOf(s api.BillingSeller) party {
	out := party{Name: s.CompanyName, TaxOffice: s.TaxOffice, TaxNumber: s.TaxNumber, Address: s.Address, Email: s.Email, Phone: s.Phone,
		Website: s.Website, Note: s.InvoiceNote}
	if s.MersisNo != nil {
		out.MersisNo = *s.MersisNo
	}
	for _, b := range s.BankAccounts {
		out.Banks = append(out.Banks, bankAccount{BankName: b.BankName, AccountHolder: b.AccountHolder, IBAN: b.Iban})
	}
	return out
}

// issue creates a proforma for the tenant q is scoped to: the buyer (the
// invoice details) and the seller are snapshots, the reference is the
// company code + its next proforma counter + a check character, and the
// company's account credit is applied to it at once.
func (s *Service) issue(ctx context.Context, q *db.Queries, sub *db.Subscription, fx *effects, d draft) (db.Proforma, error) {
	seller, pricing, err := s.settings(ctx)
	if err != nil {
		return db.Proforma{}, err
	}
	if !Configured(seller) {
		return db.Proforma{}, apperr.Conflict(CodeNotConfigured, "Billing is not set up yet: Messora has not entered its bank accounts.")
	}
	profile, err := q.GetBillingProfile(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Proforma{}, apperr.Conflict(CodeProfileRequired, "Enter the invoice details (company or person, tax number, address) first.")
	}
	if err != nil {
		return db.Proforma{}, err
	}
	seq, err := q.NextProformaSeq(ctx)
	if err != nil {
		return db.Proforma{}, err
	}
	ref, err := Reference(seq.Code, seq.ProformaSeq)
	if err != nil {
		return db.Proforma{}, err
	}
	n, err := q.NextProformaNumber(ctx)
	if err != nil {
		return db.Proforma{}, err
	}
	now := s.now()
	due := now.AddDate(0, 0, pricing.API.DueDays)
	if d.due != nil {
		due = *d.due
	}
	tot := WithVAT(d.net, pricing.VATRateBP)
	lines, _ := json.Marshal(d.lines)
	buyer, _ := json.Marshal(buyerOf(profile))
	sel, _ := json.Marshal(sellerOf(seller))
	f, err := q.InsertProforma(ctx, db.InsertProformaParams{Number: fmt.Sprintf("PF-%d-%06d", now.In(istanbul).Year(), n), Reference: ref,
		Kind: d.kind, Renewal: d.renewal, Lines: lines, NetKurus: tot.Net, VatRateBp: tot.RateBP, VatKurus: tot.VAT, TotalKurus: tot.Total,
		PlanKey: d.planKey, BillingCycle: d.cycle, PeriodStart: d.periodStart, PeriodEnd: d.periodEnd, AiCredits: d.aiCredits,
		PackKey: d.packKey, MessageAmountKurus: d.messageAmount, Buyer: buyer, Seller: sel, DueAt: due, CreatedBy: d.createdBy})
	if err != nil {
		return db.Proforma{}, err
	}
	if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.proforma_issue", EntityType: "proforma", EntityID: &f.ID,
		Changes: map[string]any{"number": f.Number, "kind": f.Kind, "total": FormatKurus(f.TotalKurus), "renewal": f.Renewal}}); err != nil {
		return db.Proforma{}, err
	}
	if _, err := q.InsertBillingEvent(ctx, db.InsertBillingEventParams{Kind: "proforma_issued", Key: f.ID.String(), ProformaID: &f.ID}); err != nil {
		return db.Proforma{}, err
	}
	if err := s.applyCredit(ctx, q, sub, fx); err != nil {
		return db.Proforma{}, err
	}
	if cur, err := q.GetProforma(ctx, f.ID); err == nil {
		f = cur
	}
	if f.Status != "paid" {
		issued := f
		fx.later(func(ctx context.Context) { s.mailIssued(ctx, issued) })
	}
	return f, nil
}

// Purchase creates a proforma for an AI credit pack or a message balance
// top-up.
func (s *Service) Purchase(ctx context.Context, actor *principal.Principal, req api.CreateBillingProformaRequest) (db.Proforma, error) {
	id := tenant.ID(ctx)
	var out db.Proforma
	err := s.tenantTx(ctx, id, func(ctx context.Context, q *db.Queries, fx *effects) error {
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		_, pricing, err := s.settings(ctx)
		if err != nil {
			return err
		}
		d := draft{kind: string(req.Kind), createdBy: actorID(actor)}
		switch req.Kind {
		case api.CreateBillingProformaRequestKindAiCredits:
			pack, ok := pricing.Pack(deref(req.PackKey, ""))
			if !ok {
				return apperr.Validation("pack_key", "Choose one of the AI credit packs.")
			}
			d.aiCredits, d.packKey, d.net = &pack.Credits, &pack.Key, pack.Price
			d.lines = []line{{DescriptionTR: fmt.Sprintf("AI kredisi paketi · %d kredi (%d ay geçerli)", pack.Credits, pricing.API.AiCreditValidityMonths),
				DescriptionEN: fmt.Sprintf("AI credit pack · %d credits (valid %d months)", pack.Credits, pricing.API.AiCreditValidityMonths),
				Quantity:      1, UnitKurus: pack.Price, NetKurus: pack.Price}}
		case api.CreateBillingProformaRequestKindMessageBalance:
			if req.Amount == nil {
				return apperr.Validation("amount", "Enter the top-up amount.")
			}
			k, err := ParseKurus(*req.Amount)
			if err != nil || k < pricing.MinTopUp || k > 100_000_000_00 {
				return apperr.Validation("amount", fmt.Sprintf("The minimum top-up is %s TL (VAT excluded).", FormatKurus(pricing.MinTopUp)))
			}
			d.messageAmount, d.net = &k, k
			d.lines = []line{{DescriptionTR: "Mesaj bakiyesi yüklemesi (WhatsApp / Meta mesaj ücretleri)",
				DescriptionEN: "Message balance top-up (WhatsApp / Meta message fees)", Quantity: 1, UnitKurus: k, NetKurus: k}}
		default:
			return apperr.Validation("kind", "Choose ai_credits or message_balance.")
		}
		out, err = s.issue(ctx, q, &sub, fx, d)
		return err
	})
	return out, err
}

// Cancel voids the context tenant's unpaid proforma (not a renewal).
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (db.Proforma, error) {
	tid := tenant.ID(ctx)
	var out db.Proforma
	err := s.tenantTx(ctx, tid, func(ctx context.Context, q *db.Queries, _ *effects) error {
		f, err := q.GetProformaForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Proforma not found.")
		}
		if err != nil {
			return err
		}
		if f.Renewal {
			return apperr.Conflict("PROFORMA_NOT_OPEN", "A renewal cannot be cancelled; change the plan or ask Messora.")
		}
		out, err = q.VoidProforma(ctx, db.VoidProformaParams{ID: id, Reason: "cancelled by the workspace"})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("PROFORMA_NOT_OPEN", "Only an open proforma with nothing paid can be cancelled.")
		}
		if err != nil {
			return err
		}
		return audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.proforma_cancel", EntityType: "proforma", EntityID: &id,
			Changes: map[string]any{"number": f.Number}})
	})
	return out, err
}

// Void voids any company's unpaid proforma (platform admin).
func (s *Service) Void(ctx context.Context, id uuid.UUID, reason string) (db.Proforma, error) {
	f, err := s.q.GetPlatformProforma(tenant.Platform(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Proforma{}, apperr.NotFound("Proforma not found.")
	}
	if err != nil {
		return db.Proforma{}, err
	}
	var out db.Proforma
	err = s.tenantTx(ctx, f.TenantID, func(ctx context.Context, q *db.Queries, _ *effects) error {
		out, err = q.VoidProforma(ctx, db.VoidProformaParams{ID: id, Reason: strings.TrimSpace(reason)})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("PROFORMA_NOT_OPEN", "Only an open proforma with nothing paid can be voided.")
		}
		if err != nil {
			return err
		}
		ch := map[string]any{"number": f.Number, "reason": reason}
		if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.proforma_void", EntityType: "proforma", EntityID: &id, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "tenant.proforma_void", EntityType: "proforma", EntityID: &id, Tenant: &f.TenantID, Changes: ch})
	})
	return out, err
}

// ── Payments ───────────────────────────────────────────────────────────────

// pay applies amount (kuruş) of a transfer or of the account credit to an
// open proforma and fulfils it when it is paid in full. It returns what
// the proforma did not need.
func (s *Service) pay(ctx context.Context, q *db.Queries, sub *db.Subscription, f db.Proforma, amount int64, source string,
	transfer *uuid.UUID, receivedOn time.Time, by *uuid.UUID, fx *effects) (int64, error) {
	if f.Status != "open" && f.Status != "partially_paid" {
		return amount, nil
	}
	take := min(amount, f.TotalKurus-f.PaidKurus)
	if take <= 0 {
		return amount, nil
	}
	paid, err := q.AddProformaPayment(ctx, db.AddProformaPaymentParams{ID: f.ID, Amount: take})
	if err != nil {
		return 0, err
	}
	if _, err := q.InsertBillingPayment(ctx, db.InsertBillingPaymentParams{ProformaID: f.ID, Source: source, TransferID: transfer,
		AmountKurus: take, ReceivedOn: pgDate(receivedOn), CreatedBy: by}); err != nil {
		return 0, err
	}
	if err := s.fulfil(ctx, q, sub, paid, fx); err != nil {
		return 0, err
	}
	return amount - take, nil
}

// applyCredit applies the account credit to the open proformas, the one
// due first first.
func (s *Service) applyCredit(ctx context.Context, q *db.Queries, sub *db.Subscription, fx *effects) error {
	if sub.AccountCreditKurus <= 0 {
		return nil
	}
	open, err := q.LockOpenProformas(ctx)
	if err != nil {
		return err
	}
	credit := sub.AccountCreditKurus
	for _, f := range open {
		if credit <= 0 {
			break
		}
		left, err := s.pay(ctx, q, sub, f, credit, "account_credit", nil, s.now(), nil, fx)
		if err != nil {
			return err
		}
		if used := credit - left; used > 0 {
			credit = left
			if _, err := q.InsertAccountEntry(ctx, db.InsertAccountEntryParams{AmountKurus: -used, BalanceAfter: credit, Reason: "applied",
				ProformaID: &f.ID, Note: "Applied to " + f.Number}); err != nil {
				return err
			}
		}
	}
	// fulfil may have saved the subscription meanwhile: keep its fields.
	cur, err := q.GetSubscriptionForUpdate(ctx)
	if err != nil {
		return err
	}
	cur.AccountCreditKurus = credit
	*sub = cur
	return s.save(ctx, q, sub)
}

// addCredit keeps money the proformas did not need as account credit.
func (s *Service) addCredit(ctx context.Context, q *db.Queries, sub *db.Subscription, amount int64, reason string, transfer, proforma *uuid.UUID,
	note string, by *uuid.UUID) error {
	if amount == 0 {
		return nil
	}
	cur, err := q.GetSubscriptionForUpdate(ctx)
	if err != nil {
		return err
	}
	*sub = cur
	if sub.AccountCreditKurus+amount < 0 {
		return apperr.Validation("amount", "The account credit cannot go below zero.")
	}
	sub.AccountCreditKurus += amount
	if _, err := q.InsertAccountEntry(ctx, db.InsertAccountEntryParams{AmountKurus: amount, BalanceAfter: sub.AccountCreditKurus, Reason: reason,
		TransferID: transfer, ProformaID: proforma, Note: note, CreatedBy: by}); err != nil {
		return err
	}
	return s.save(ctx, q, sub)
}

func pgDate(t time.Time) pgtype.Date {
	t = t.In(istanbul)
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

// ── Invoices ───────────────────────────────────────────────────────────────

func partyOf(raw []byte) invoicing.Party {
	var p party
	_ = json.Unmarshal(raw, &p)
	return invoicing.Party{Name: p.Name, TaxOffice: p.TaxOffice, TaxNumber: p.TaxNumber, Address: p.Address, Email: p.Email}
}

// invoice creates the invoice record of a paid proforma through the
// issuer (manual: pending until the platform admin records the official
// invoice).
func (s *Service) invoice(ctx context.Context, q *db.Queries, f db.Proforma) error {
	var lines []line
	_ = json.Unmarshal(f.Lines, &lines)
	req := invoicing.Request{ProformaNumber: f.Number, Reference: f.Reference, Buyer: partyOf(f.Buyer), Seller: partyOf(f.Seller),
		NetKurus: f.NetKurus, VATRateBP: f.VatRateBp, VATKurus: f.VatKurus, TotalKurus: f.TotalKurus, PaidAt: s.now()}
	for _, l := range lines {
		req.Lines = append(req.Lines, invoicing.Line{Description: l.DescriptionTR, Quantity: l.Quantity, UnitKurus: l.UnitKurus, NetKurus: l.NetKurus})
	}
	res, err := s.o.Issuer.Issue(ctx, req)
	if err != nil {
		return fmt.Errorf("billing: issue the invoice: %w", err)
	}
	p := db.InsertInvoiceParams{ProformaID: f.ID, Issuer: s.o.Issuer.Name(), Status: "pending", NetKurus: f.NetKurus, VatKurus: f.VatKurus,
		TotalKurus: f.TotalKurus, Buyer: f.Buyer, Seller: f.Seller}
	if res.Issued && res.Number != "" {
		p.Status, p.OfficialNumber, p.IssuedOn = "issued", &res.Number, pgDate(res.IssuedOn)
	}
	if _, err := q.InsertInvoice(ctx, p); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return nil
}
