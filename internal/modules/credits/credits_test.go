package credits

import (
	"testing"
	"time"
)

func TestChargeConvertsCostIntoMillicredits(t *testing.T) {
	d := DefaultSettings() // 1 credit = 0.001 USD, markup 1, unpriced 5
	for _, c := range []struct {
		cost string
		ok   bool
		want int64
	}{
		{"0.00014", true, 140},       // 0.14 credits
		{"0.0000001", true, 1},       // rounded up to one millicredit
		{"0.001", true, 1000},        // exactly one credit
		{"1.2345678", true, 1234568}, // ceil
		{"", true, 5000},             // no price: the flat charge
		{"", false, 0},               // a failed call without usage
		{"0", true, 0},               // free (e.g. no tokens)
		{"-1", true, 0},              // never negative
		{"not a number", true, 0},    // ignored
		{"0.00014", false, 140},      // a failed call with usage still pays its tokens
	} {
		if got := d.Charge(c.cost, c.ok); got != c.want {
			t.Errorf("Charge(%q, %v) = %d, want %d", c.cost, c.ok, got, c.want)
		}
	}
	m := Settings{USDPerCredit: "0.002", Markup: "1.5", UnpricedCallCredits: "0.5"}
	if got := m.Charge("0.001", true); got != 750 { // 0.001 × 1.5 ÷ 0.002 = 0.75 credits
		t.Errorf("markup: %d", got)
	}
	if got := m.Charge("", true); got != 500 {
		t.Errorf("unpriced: %d", got)
	}
	// Invalid settings fall back to the defaults.
	if got := (Settings{USDPerCredit: "0", Markup: "x"}).Charge("0.001", true); got != 1000 {
		t.Errorf("invalid settings: %d", got)
	}
}

func TestSettingsValidate(t *testing.T) {
	if err := DefaultSettings().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Settings{
		{USDPerCredit: "0", Markup: "1", UnpricedCallCredits: "1"},
		{USDPerCredit: "0.001", Markup: "-1", UnpricedCallCredits: "1"},
		{USDPerCredit: "0.001", Markup: "1", UnpricedCallCredits: "abc"},
		{USDPerCredit: "99999999999", Markup: "1", UnpricedCallCredits: "1"},
	} {
		if err := s.Validate(); err == nil {
			t.Errorf("%+v accepted", s)
		}
	}
	// A free unpriced call is allowed (0).
	if err := (Settings{USDPerCredit: "0.001", Markup: "1", UnpricedCallCredits: "0"}).Validate(); err != nil {
		t.Errorf("zero unpriced: %v", err)
	}
}

func TestBalanceAndMonths(t *testing.T) {
	at := time.Date(2026, 11, 30, 23, 59, 0, 0, time.FixedZone("TRT", 3*3600)) // 20:59 UTC
	if m := MonthStart(at); !m.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("month: %v", m)
	}
	if !(Balance{Remaining: 0}).Exhausted() || (Balance{Remaining: 1}).Exhausted() || (Balance{Unlimited: true}).Exhausted() {
		t.Fatal("Exhausted")
	}
	if Credits(1500) != 1.5 {
		t.Fatal("Credits")
	}
}
