package billing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Billing e-mails (docs/EMAIL.md): invoice_issued for a new proforma,
// payment_reminder 7 and 3 days before the due date, on the due day and
// once overdue, payment_received, subscription_suspended when the
// workspace turns read-only, credit_low for the message balance. They go to
// the workspace's owners and its billing contact, branded and in each
// recipient's language. Best effort, after the transaction committed.

// BillingPath is the panel's billing page.
const BillingPath = "/settings/billing"

func (s *Service) notify(ctx context.Context, id uuid.UUID, kind templates.Kind, build func(c templates.Common, link func(string) string, loc *time.Location) templates.Data) {
	if s.o.Notify == nil {
		return
	}
	ctx = tenant.WithID(ctx, id)
	contact := ""
	if p, err := s.q.GetBillingProfile(ctx); err == nil {
		contact = p.Email
	}
	s.o.Notify(ctx, identity.BillingNotice{Kind: kind, Contact: contact, Build: build})
}

func money(k int64) templates.Money { return templates.Money{Minor: k, Currency: "TRY"} }

// bankOf is the e-mail's bank transfer block: the proforma's first account
// and its reference.
func bankOf(f db.Proforma) *templates.BankTransfer {
	var p party
	_ = json.Unmarshal(f.Seller, &p)
	if len(p.Banks) == 0 {
		return nil
	}
	b := p.Banks[0]
	return &templates.BankTransfer{BankName: b.BankName, AccountHolder: b.AccountHolder, IBAN: FormatIBAN(b.IBAN), Reference: DisplayReference(f.Reference)}
}

func periodOf(f db.Proforma) string {
	if f.PeriodStart == nil || f.PeriodEnd == nil {
		return ""
	}
	return dateTR(*f.PeriodStart) + " – " + dateTR(f.PeriodEnd.Add(-time.Second))
}

func (s *Service) mailIssued(ctx context.Context, f db.Proforma) {
	s.notify(ctx, f.TenantID, templates.InvoiceIssued, func(c templates.Common, link func(string) string, loc *time.Location) templates.Data {
		return templates.InvoiceIssuedData{Common: c, InvoiceNo: f.Number, Period: periodOf(f), Amount: money(f.TotalKurus - f.PaidKurus),
			DueDate: f.DueAt.In(loc), Bank: bankOf(f), InvoiceURL: link(BillingPath)}
	})
}

func (s *Service) mailReminder(ctx context.Context, id uuid.UUID, f db.Proforma, overdue bool, grace time.Time) {
	s.notify(ctx, id, templates.PaymentReminder, func(c templates.Common, link func(string) string, loc *time.Location) templates.Data {
		d := templates.PaymentReminderData{Common: c, InvoiceNo: f.Number, Amount: money(f.TotalKurus - f.PaidKurus), DueDate: f.DueAt.In(loc),
			Overdue: overdue, Bank: bankOf(f), PayURL: link(BillingPath)}
		if overdue {
			d.GraceEndsAt = grace.In(loc)
		}
		return d
	})
}

func (s *Service) mailPaid(ctx context.Context, id uuid.UUID, f db.Proforma) {
	at := s.now()
	if f.PaidAt != nil {
		at = *f.PaidAt
	}
	s.notify(ctx, id, templates.PaymentReceived, func(c templates.Common, link func(string) string, loc *time.Location) templates.Data {
		return templates.PaymentReceivedData{Common: c, InvoiceNo: f.Number, Amount: money(f.TotalKurus), PaidAt: at.In(loc),
			Method: "Havale/EFT", InvoiceURL: link(BillingPath)}
	})
}

func (s *Service) mailSuspended(ctx context.Context, id uuid.UUID, f *db.Proforma) {
	at := s.now()
	s.notify(ctx, id, templates.SubscriptionSuspended, func(c templates.Common, link func(string) string, loc *time.Location) templates.Data {
		d := templates.SubscriptionSuspendedData{Common: c, SuspendedAt: at.In(loc), BillingURL: link(BillingPath)}
		if f != nil {
			d.InvoiceNo, d.Amount = f.Number, money(f.TotalKurus-f.PaidKurus)
		}
		return d
	})
}

func (s *Service) mailMessageLow(ctx context.Context) {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return
	}
	b, err := balanceMicros(ctx, s.q)
	if err != nil {
		s.o.Logger.WarnContext(ctx, "message balance for the low balance e-mail", "err", err)
		return
	}
	_, pricing, err := s.settings(ctx)
	if err != nil {
		return
	}
	s.notify(ctx, id, templates.CreditLow, func(c templates.Common, link func(string) string, _ *time.Location) templates.Data {
		return templates.CreditLowData{Common: c, CreditType: templates.CreditMessage, Balance: max(MicrosToKurus(b)/100, 0),
			Threshold: max(pricing.LowThreshold/100, 1), TopUpURL: link(BillingPath)}
	})
}
