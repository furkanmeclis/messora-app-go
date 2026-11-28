// Package payments abstracts how a company pays a proforma. The launch
// method is bank transfer (havale/EFT): the proforma shows Messora's IBANs
// and a payment reference, a platform admin confirms the incoming
// transfer. Card payments (PayTR or iyzico Abonelik) come later behind the
// same interface; Card is a disabled stub until then.
package payments

import (
	"context"
	"errors"
)

// Methods.
const (
	MethodBankTransfer = "bank_transfer"
	MethodCard         = "card"
)

// Account is a bank account to pay into.
type Account struct{ BankName, AccountHolder, IBAN string }

// Instructions tell the company how to pay one proforma.
type Instructions struct {
	Method string
	// Accounts and Reference for a bank transfer: the reference goes into
	// the transfer's description.
	Accounts  []Account
	Reference string
	// RedirectURL for a card payment page.
	RedirectURL string
}

// Due is the proforma to pay.
type Due struct {
	ProformaID string
	Reference  string
	TotalKurus int64
}

// Provider is a payment method.
type Provider interface {
	Method() string
	// Enabled reports whether companies can use the method now.
	Enabled() bool
	// Instructions describe how to pay due.
	Instructions(ctx context.Context, due Due) (Instructions, error)
}

// ErrUnavailable is returned by a method that is not enabled.
var ErrUnavailable = errors.New("payments: this payment method is not available")

// BankTransfer pays into the seller's accounts with the proforma's
// reference; the platform admin confirms the money (billing matches it).
type BankTransfer struct {
	// Accounts returns the seller's bank accounts (platform setting).
	Accounts func(ctx context.Context) ([]Account, error)
}

// Method implements Provider.
func (BankTransfer) Method() string { return MethodBankTransfer }

// Enabled implements Provider.
func (BankTransfer) Enabled() bool { return true }

// Instructions implements Provider.
func (b BankTransfer) Instructions(ctx context.Context, due Due) (Instructions, error) {
	var acc []Account
	if b.Accounts != nil {
		var err error
		if acc, err = b.Accounts(ctx); err != nil {
			return Instructions{}, err
		}
	}
	if len(acc) == 0 {
		return Instructions{}, ErrUnavailable
	}
	return Instructions{Method: MethodBankTransfer, Accounts: acc, Reference: due.Reference}, nil
}

// Card is the card payment stub (PayTR / iyzico later): never enabled.
type Card struct{}

// Method implements Provider.
func (Card) Method() string { return MethodCard }

// Enabled implements Provider.
func (Card) Enabled() bool { return false }

// Instructions implements Provider.
func (Card) Instructions(context.Context, Due) (Instructions, error) {
	return Instructions{}, ErrUnavailable
}
