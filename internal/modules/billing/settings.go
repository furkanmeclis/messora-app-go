package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Platform settings of billing (platform_settings): who sells (the legal
// entity is not settled yet, so it is data, not code) and the prices.
const (
	SellerKey  = "billing_seller"
	PricingKey = "billing_pricing"
)

// DefaultPricing is the launch proposal (docs/design/rakip-fiyat-analizi.md):
// 20 % VAT, marketing at Meta × 1.20, other categories Meta + $0.0006, AI
// credit packs 1,000 / 5,000 / 20,000 credits for 590 / 2,490 / 7,990 TL,
// message top-ups from 1,000 TL. The FX rate is a placeholder until the
// platform admin sets the day's rate.
func DefaultPricing() api.BillingPricing {
	return api.BillingPricing{VatRatePercent: "20", FxUsdTry: "42.00", MarketingMarkup: "1.20", OtherMarkupUsd: "0.0006",
		AiCreditPacks: []api.AICreditPack{{Key: "c1000", Credits: 1000, Price: "590.00"}, {Key: "c5000", Credits: 5000, Price: "2490.00"},
			{Key: "c20000", Credits: 20000, Price: "7990.00"}},
		MessageTopups: []api.Amount{"1000.00", "2500.00", "5000.00", "10000.00"}, MinMessageTopup: "1000.00", LowBalanceThreshold: "200.00",
		RenewalLeadDays: 10, DueDays: 7, GraceDays: 7, AiCreditValidityMonths: 12}
}

// DefaultSeller is empty: proformas need the platform admin to set the
// seller and at least one bank account first.
func DefaultSeller() api.BillingSeller {
	return api.BillingSeller{BankAccounts: []api.BankAccount{}}
}

// Pricing is the parsed pricing setting.
type Pricing struct {
	API             api.BillingPricing
	VATRateBP       int64
	FX              Rate
	MarketingMarkup Rate
	OtherMarkupUSD  Rate
	MinTopUp        int64
	LowThreshold    int64
	TopUps          []int64
	Packs           []Pack
}

// Pack is an AI credit pack of the catalogue.
type Pack struct {
	Key     string
	Credits int64
	Price   int64 // kuruş, VAT excluded
}

// Pack returns the pack with key.
func (p Pricing) Pack(key string) (Pack, bool) {
	for _, x := range p.Packs {
		if x.Key == key {
			return x, true
		}
	}
	return Pack{}, false
}

func fieldErr(field, msg string) error { return apperr.Validation(field, msg) }

// ParsePricing validates a pricing setting.
func ParsePricing(in api.BillingPricing) (Pricing, error) {
	p := Pricing{API: in}
	var err error
	if p.VATRateBP, err = ParseRateBP(in.VatRatePercent); err != nil {
		return p, fieldErr("pricing.vat_rate_percent", err.Error())
	}
	if p.FX, err = ParseRate(in.FxUsdTry); err != nil || p.FX.IsZero() {
		return p, fieldErr("pricing.fx_usd_try", "Enter the USD → TL rate, e.g. 42.18.")
	}
	if p.MarketingMarkup, err = ParseRate(in.MarketingMarkup); err != nil || p.MarketingMarkup.IsZero() {
		return p, fieldErr("pricing.marketing_markup", "Enter a factor such as 1.20.")
	}
	if p.OtherMarkupUSD, err = ParseRate(in.OtherMarkupUsd); err != nil {
		return p, fieldErr("pricing.other_markup_usd", "Enter a USD amount such as 0.0006.")
	}
	if p.MinTopUp, err = ParseKurus(in.MinMessageTopup); err != nil || p.MinTopUp <= 0 {
		return p, fieldErr("pricing.min_message_topup", "Enter a TL amount above 0.")
	}
	if p.LowThreshold, err = ParseKurus(in.LowBalanceThreshold); err != nil || p.LowThreshold < 0 {
		return p, fieldErr("pricing.low_balance_threshold", "Enter a TL amount.")
	}
	for _, t := range in.MessageTopups {
		k, err := ParseKurus(t)
		if err != nil || k < p.MinTopUp {
			return p, fieldErr("pricing.message_topups", "Every top-up amount is a TL amount at least the minimum top-up.")
		}
		p.TopUps = append(p.TopUps, k)
	}
	seen := map[string]bool{}
	for _, x := range in.AiCreditPacks {
		k, err := ParseKurus(x.Price)
		if err != nil || k <= 0 || x.Credits <= 0 || seen[x.Key] || !packKey.MatchString(x.Key) {
			return p, fieldErr("pricing.ai_credit_packs", "Every pack has its own key, credits above 0 and a price above 0.")
		}
		seen[x.Key] = true
		p.Packs = append(p.Packs, Pack{Key: x.Key, Credits: x.Credits, Price: k})
	}
	if in.RenewalLeadDays < 7 || in.RenewalLeadDays > 60 {
		return p, fieldErr("pricing.renewal_lead_days", "Between 7 and 60 days (the first reminder goes 7 days before the due date).")
	}
	if in.DueDays < 1 || in.DueDays > 60 || in.GraceDays < 0 || in.GraceDays > 60 || in.AiCreditValidityMonths < 1 || in.AiCreditValidityMonths > 60 {
		return p, fieldErr("pricing", "Due days 1–60, grace days 0–60, AI credit validity 1–60 months.")
	}
	return p, nil
}

var packKey = regexp.MustCompile(`^[a-z0-9_-]{1,30}$`)

// NormalizeIBAN drops spaces and upper-cases an IBAN.
func NormalizeIBAN(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), ""))
}

// ValidIBAN checks an IBAN's shape and its ISO 13616 mod-97 checksum (a
// Turkish IBAN has 26 characters).
func ValidIBAN(iban string) bool {
	iban = NormalizeIBAN(iban)
	if len(iban) < 15 || len(iban) > 34 || !ibanShape.MatchString(iban) {
		return false
	}
	if strings.HasPrefix(iban, "TR") && len(iban) != 26 {
		return false
	}
	re := iban[4:] + iban[:4]
	var b strings.Builder
	for _, r := range re {
		if r >= 'A' && r <= 'Z' {
			fmt.Fprintf(&b, "%d", r-'A'+10)
		} else {
			b.WriteRune(r)
		}
	}
	n, ok := new(big.Int).SetString(b.String(), 10)
	return ok && new(big.Int).Mod(n, big.NewInt(97)).Int64() == 1
}

var ibanShape = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[0-9A-Z]+$`)

// FormatIBAN groups an IBAN in blocks of four.
func FormatIBAN(iban string) string {
	iban = NormalizeIBAN(iban)
	var parts []string
	for i := 0; i < len(iban); i += 4 {
		parts = append(parts, iban[i:min(i+4, len(iban))])
	}
	return strings.Join(parts, " ")
}

// ValidateSeller checks and normalizes the seller identity.
func ValidateSeller(in api.BillingSeller) (api.BillingSeller, error) {
	in.CompanyName, in.TaxOffice, in.TaxNumber = strings.TrimSpace(in.CompanyName), strings.TrimSpace(in.TaxOffice), strings.TrimSpace(in.TaxNumber)
	in.Address, in.Email, in.Phone, in.Website = strings.TrimSpace(in.Address), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Website)
	in.InvoiceNote = strings.TrimSpace(in.InvoiceNote)
	if in.BankAccounts == nil {
		in.BankAccounts = []api.BankAccount{}
	}
	for i, b := range in.BankAccounts {
		b.Iban = NormalizeIBAN(b.Iban)
		if !ValidIBAN(b.Iban) {
			return in, fieldErr(fmt.Sprintf("seller.bank_accounts[%d].iban", i), "This IBAN is not valid (check the digits).")
		}
		b.BankName, b.AccountHolder = strings.TrimSpace(b.BankName), strings.TrimSpace(b.AccountHolder)
		if b.BankName == "" || b.AccountHolder == "" {
			return in, fieldErr(fmt.Sprintf("seller.bank_accounts[%d]", i), "Name the bank and the account holder.")
		}
		in.BankAccounts[i] = b
	}
	if len(in.BankAccounts) > 0 && in.CompanyName == "" {
		return in, fieldErr("seller.company_name", "Name the company that receives the payments.")
	}
	return in, nil
}

// Configured reports whether proformas can be issued: a seller with at
// least one bank account.
func Configured(s api.BillingSeller) bool { return s.CompanyName != "" && len(s.BankAccounts) > 0 }

// settingsTTL is how long the settings are used before they are read again
// (a change in the panel applies on this replica at once).
const settingsTTL = 30 * time.Second

type settingsCache struct {
	mu      sync.Mutex
	seller  api.BillingSeller
	pricing Pricing
	expires time.Time
}

func readSetting[T any](ctx context.Context, q db.Querier, key string, def T) (T, error) {
	raw, err := q.GetPlatformSetting(tenant.System(ctx), key)
	if errors.Is(err, pgx.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	v := def
	if err := json.Unmarshal(raw, &v); err != nil {
		return def, nil
	}
	return v, nil
}

// ReadSettings reads the seller and the pricing (defaults when unset; an
// invalid stored pricing falls back to the defaults).
func ReadSettings(ctx context.Context, q db.Querier) (api.BillingSeller, Pricing, error) {
	seller, err := readSetting(ctx, q, SellerKey, DefaultSeller())
	if err != nil {
		return seller, Pricing{}, err
	}
	if seller.BankAccounts == nil {
		seller.BankAccounts = []api.BankAccount{}
	}
	raw, err := readSetting(ctx, q, PricingKey, DefaultPricing())
	if err != nil {
		return seller, Pricing{}, err
	}
	p, perr := ParsePricing(raw)
	if perr != nil {
		p, _ = ParsePricing(DefaultPricing())
	}
	return seller, p, nil
}

// settings returns the cached settings.
func (s *Service) settings(ctx context.Context) (api.BillingSeller, Pricing, error) {
	now := s.now()
	c := &s.cache
	c.mu.Lock()
	if now.Before(c.expires) {
		sel, p := c.seller, c.pricing
		c.mu.Unlock()
		return sel, p, nil
	}
	c.mu.Unlock()
	sel, p, err := ReadSettings(ctx, s.q)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.expires.IsZero() {
			return c.seller, c.pricing, nil
		}
		return sel, p, err
	}
	c.mu.Lock()
	c.seller, c.pricing, c.expires = sel, p, now.Add(settingsTTL)
	c.mu.Unlock()
	return sel, p, nil
}

// InvalidateSettings drops the cached settings.
func (s *Service) InvalidateSettings() {
	s.cache.mu.Lock()
	s.cache.expires = time.Time{}
	s.cache.mu.Unlock()
}

func saveSetting(ctx context.Context, q db.Querier, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return q.UpsertPlatformSetting(ctx, db.UpsertPlatformSettingParams{Key: key, Value: raw})
}
