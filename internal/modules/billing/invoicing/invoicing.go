// Package invoicing issues the official invoice of a paid proforma
// (e-Fatura when the buyer is an e-Fatura taxpayer, e-Arşiv otherwise).
//
// The legal entity that invoices is not settled yet (a partner company
// will likely issue them), so issuing is pluggable. The launch issuer is
// Manual: Messora only keeps an invoice record, and a platform admin
// records the official number and uploads the PDF issued elsewhere. An
// integrator issuer (a GİB-licensed özel entegratör such as Paraşüt or
// Sovos) can replace it without touching billing.
package invoicing

import (
	"context"
	"time"
)

// Party is the buyer or the seller as printed on the invoice.
type Party struct {
	Name, TaxOffice, TaxNumber, Address, Email string
}

// Line is an invoice line, amounts in kuruş, VAT excluded.
type Line struct {
	Description string
	Quantity    int64
	UnitKurus   int64
	NetKurus    int64
}

// Request is an invoice to issue for a paid proforma.
type Request struct {
	ProformaNumber string
	Reference      string
	Buyer, Seller  Party
	Lines          []Line
	NetKurus       int64
	VATRateBP      int64
	VATKurus       int64
	TotalKurus     int64
	PaidAt         time.Time
}

// Result is what the issuer did.
type Result struct {
	// Issued: the official invoice exists (Number and IssuedOn set);
	// otherwise the record waits for the platform admin.
	Issued   bool
	Number   string
	IssuedOn time.Time
}

// Issuer issues invoices.
type Issuer interface {
	// Name is stored on the invoice record ("manual").
	Name() string
	// Issue is called in the payment's transaction, after the proforma is
	// paid; an error rolls the payment back, so a remote issuer should
	// queue its work and return a pending Result.
	Issue(ctx context.Context, r Request) (Result, error)
}

// Manual issues nothing: the invoice record waits for the official
// number and PDF from the platform admin panel.
type Manual struct{}

// Name implements Issuer.
func (Manual) Name() string { return "manual" }

// Issue implements Issuer.
func (Manual) Issue(context.Context, Request) (Result, error) { return Result{}, nil }
