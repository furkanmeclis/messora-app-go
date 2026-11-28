package billing

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Money math (docs/BILLING.md "Money"). Amounts are integer kuruş
// (1 TL = 100 kuruş); the message balance is micro-lira (1 TL = 1,000,000)
// because one message costs a fraction of a kuruş. Every division rounds
// half up (away from zero for the positive amounts billing uses), the way
// Turkish invoices round: 0.5 kuruş becomes 1 kuruş. VAT is computed once
// on a proforma's net total, never per line.

// MicrosPerKurus converts kuruş to micro-lira.
const MicrosPerKurus = 10_000

// ErrAmount is an amount that is not a TL decimal with at most two
// decimals.
var ErrAmount = errors.New("billing: an amount is TL with at most two decimals, e.g. 1290 or 1290.50")

// roundHalfUp rounds r to the nearest integer, halves away from zero.
func roundHalfUp(r *big.Rat) int64 {
	num := new(big.Int).Set(r.Num())
	den := r.Denom()
	neg := num.Sign() < 0
	if neg {
		num.Neg(num)
	}
	// ⌊(2·num + den) / (2·den)⌋
	n := new(big.Int).Mul(num, big.NewInt(2))
	n.Add(n, den)
	d := new(big.Int).Mul(den, big.NewInt(2))
	q := new(big.Int).Quo(n, d)
	if neg {
		q.Neg(q)
	}
	if !q.IsInt64() {
		if neg {
			return -(1<<62 - 1)
		}
		return 1<<62 - 1
	}
	return q.Int64()
}

// ParseKurus parses a TL decimal ("1290", "1290.5", "1290.50", "-12.30";
// a comma is accepted as the decimal separator) into kuruş.
func ParseKurus(s string) (int64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, ErrAmount
	}
	if i := strings.IndexByte(s, '.'); i >= 0 && len(s)-i-1 > 2 {
		return 0, ErrAmount
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, ErrAmount
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() || !r.Num().IsInt64() || r.Num().Int64() > 1e15 || r.Num().Int64() < -1e15 {
		return 0, ErrAmount
	}
	return r.Num().Int64(), nil
}

// FormatKurus renders kuruş as a TL decimal with two decimals ("3588.00").
func FormatKurus(k int64) string {
	sign := ""
	if k < 0 {
		sign, k = "-", -k
	}
	return fmt.Sprintf("%s%d.%02d", sign, k/100, k%100)
}

// FormatMicros renders micro-lira with six decimals ("1842.600000").
func FormatMicros(m int64) string {
	sign := ""
	if m < 0 {
		sign, m = "-", -m
	}
	return fmt.Sprintf("%s%d.%06d", sign, m/1_000_000, m%1_000_000)
}

// MicrosToKurus rounds micro-lira to kuruş (half up), for display.
func MicrosToKurus(m int64) int64 { return roundHalfUp(big.NewRat(m, MicrosPerKurus)) }

// VAT is the VAT of net kuruş at rate basis points (2000 = 20 %), rounded
// half up to a kuruş.
func VAT(net int64, rateBP int64) int64 {
	return roundHalfUp(new(big.Rat).SetFrac(big.NewInt(net*int64(rateBP)), big.NewInt(10_000)))
}

// Totals is a proforma's net, VAT and total.
type Totals struct {
	Net, VAT, Total int64
	RateBP          int64
}

// WithVAT adds VAT at rateBP to net.
func WithVAT(net int64, rateBP int64) Totals {
	v := VAT(net, rateBP)
	return Totals{Net: net, VAT: v, Total: net + v, RateBP: rateBP}
}

// FormatRateBP renders basis points as a percentage ("20").
func FormatRateBP(bp int64) string {
	if bp%100 == 0 {
		return fmt.Sprintf("%d", bp/100)
	}
	return strings.TrimRight(fmt.Sprintf("%d.%02d", bp/100, bp%100), "0")
}

// ParseRateBP parses a percentage ("20", "20.5") into basis points.
func ParseRateBP(s string) (int64, error) {
	k, err := ParseKurus(s) // two decimals: the same shape
	if err != nil || k < 0 || k > 10_000 {
		return 0, fmt.Errorf("billing: a VAT rate is a percentage between 0 and 100")
	}
	return k, nil
}

// Prorate is the part of diff (kuruş per period) owed for what is left of
// the period: diff × remaining days ÷ period days, half up. Days are whole
// days: the remaining part of today counts as a full day, so an upgrade on
// the last day still costs a day; the period's length is its own (28–31
// days for a month, 365/366 for a year).
func Prorate(diff int64, now, start, end time.Time) int64 {
	if diff <= 0 || !end.After(now) || !end.After(start) {
		return 0
	}
	if now.Before(start) {
		now = start
	}
	total := wholeDays(end.Sub(start))
	left := min(wholeDays(end.Sub(now)), total)
	return roundHalfUp(new(big.Rat).SetFrac(big.NewInt(diff*left), big.NewInt(total)))
}

// wholeDays rounds a duration up to whole days (at least 1).
func wholeDays(d time.Duration) int64 {
	days := int64(d / (24 * time.Hour))
	if d%(24*time.Hour) > 0 {
		days++
	}
	return max(days, 1)
}

// Rate is a decimal factor such as an FX rate or a markup.
type Rate struct{ r *big.Rat }

// ParseRate parses a non-negative decimal ("42.18", "1.20", "0.0006").
func ParseRate(s string) (Rate, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(strings.ReplaceAll(s, ",", ".")))
	if !ok || r.Sign() < 0 || r.Cmp(big.NewRat(1_000_000, 1)) > 0 {
		return Rate{}, fmt.Errorf("billing: %q is not a decimal number", s)
	}
	return Rate{r: r}, nil
}

func (r Rate) rat() *big.Rat {
	if r.r == nil {
		return new(big.Rat)
	}
	return r.r
}

// IsZero reports a zero (or unset) rate.
func (r Rate) IsZero() bool { return r.rat().Sign() == 0 }

// String renders the rate with up to six decimals.
func (r Rate) String() string {
	return strings.TrimRight(strings.TrimRight(r.rat().FloatString(6), "0"), ".")
}

// MessageCharge is the TL charge of one message Meta billed at usdMicros
// (Meta's price in micro-USD): marketing costs price × marketingMarkup,
// every other category price + otherMarkupUSD; the USD amount is converted
// at fx (TL per USD) and rounded half up to a micro-lira.
func MessageCharge(usdMicros int64, category string, fx, marketingMarkup, otherMarkupUSD Rate) int64 {
	if usdMicros <= 0 {
		return 0
	}
	usd := big.NewRat(usdMicros, 1) // micro-USD
	if strings.EqualFold(category, "marketing") || strings.EqualFold(category, "marketing_lite") {
		usd.Mul(usd, marketingMarkup.rat())
	} else {
		usd.Add(usd, new(big.Rat).Mul(otherMarkupUSD.rat(), big.NewRat(1_000_000, 1)))
	}
	return roundHalfUp(usd.Mul(usd, fx.rat())) // micro-USD × TL/USD = micro-TL
}
