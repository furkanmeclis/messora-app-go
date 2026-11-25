package metacost

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fixture is an invented rate card: the prices are test values, not Meta's.
const fixture = `market,countries,category,price,currency,effective_from,tier_lower,tier_upper
Turkey,TR,marketing,0.0100,USD,2026-01-01,,
Turkey,,marketing,0.0200,USD,2026-07-01,,
Turkey,,utility,0.0050,USD,2026-01-01,1,1000
Turkey,,utility,0.0040,USD,2026-01-01,1001,
Turkey,,service,0.0030,USD,2026-10-01,,
Rest of Western Europe,"AT;BE;CH",marketing,0.0700,USD,2026-01-01,,
Other,,marketing,0.0600,USD,2026-01-01,,
Turkey,,marketing,0.3500,TRY,2026-01-01,,
`

func card(t *testing.T, currency string) *Card {
	t.Helper()
	rates, markets, err := ParseCSV(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	return &Card{Rates: rates, Markets: markets, Currency: currency}
}

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d.Add(12 * time.Hour)
}

func TestRateFor(t *testing.T) {
	c := card(t, "USD")
	cases := []struct {
		name, country, category, at string
		volume                      int64
		want                        string // "" = no rate
	}{
		{"before any rate", "TR", "marketing", "2025-12-31", 0, ""},
		{"first period", "TR", "marketing", "2026-03-01", 0, "0.010000"},
		{"newer rate replaces older", "TR", "marketing", "2026-07-01", 0, "0.020000"},
		{"day before change", "TR", "marketing", "2026-06-30", 0, "0.010000"},
		{"tier 1", "TR", "utility", "2026-05-01", 0, "0.005000"},
		{"last message of tier 1", "TR", "utility", "2026-05-01", 999, "0.005000"},
		{"first message of tier 2", "TR", "utility", "2026-05-01", 1000, "0.004000"},
		{"open-ended tier", "TR", "utility", "2026-05-01", 50_000_000, "0.004000"},
		{"service billed only from October", "TR", "service", "2026-09-30", 0, ""},
		{"service", "TR", "service", "2026-10-02", 0, "0.003000"},
		{"grouped market", "BE", "marketing", "2026-02-01", 0, "0.070000"},
		{"lower-case country", "ch", "marketing", "2026-02-01", 0, "0.070000"},
		{"unlisted country falls back to Other", "BR", "marketing", "2026-02-01", 0, "0.060000"},
		{"no Other rate for category", "BR", "utility", "2026-02-01", 0, ""},
		{"unknown category", "TR", "marketing_lite", "2026-02-01", 0, ""},
		{"empty country uses Other", "", "marketing", "2026-02-01", 0, "0.060000"},
	}
	for _, tc := range cases {
		r, ok := c.RateFor(tc.country, tc.category, day(tc.at), tc.volume)
		got := ""
		if ok {
			got = r.Price.String()
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	// The account's currency selects the rates.
	if r, ok := card(t, "TRY").RateFor("TR", "marketing", day("2026-08-01"), 0); !ok || r.Price.String() != "0.350000" || r.Currency != "TRY" {
		t.Fatalf("TRY rate: %+v %v", r, ok)
	}
	if _, ok := card(t, "EUR").RateFor("TR", "marketing", day("2026-08-01"), 0); ok {
		t.Fatal("rate in another currency used")
	}
}

func TestRateForExplicitEffectiveTo(t *testing.T) {
	to := day("2026-02-01").Truncate(24 * time.Hour)
	c := &Card{Rates: []Rate{{Currency: "USD", Market: "Turkey", Category: "marketing", Price: 1, EffectiveFrom: day("2026-01-01").Truncate(24 * time.Hour), EffectiveTo: &to}},
		Markets: map[string]string{"TR": "Turkey"}}
	if _, ok := c.RateFor("TR", "MARKETING", day("2026-01-31"), 0); !ok {
		t.Fatal("rate before effective_to not used")
	}
	if _, ok := c.RateFor("TR", "marketing", to, 0); ok {
		t.Fatal("effective_to is exclusive")
	}
}

func TestNormalizeCategory(t *testing.T) {
	for in, want := range map[string]string{"AUTHENTICATION_INTERNATIONAL": "authentication-international",
		"authentication-international": "authentication-international", " Marketing ": "marketing", "marketing_lite": "marketing_lite"} {
		if got := NormalizeCategory(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestMicros(t *testing.T) {
	for in, want := range map[string]string{"0.0436": "0.043600", "1": "1.000000", "0.000125": "0.000125", "12.5": "12.500000"} {
		m, err := ParseMicros(in)
		if err != nil || m.String() != want {
			t.Errorf("%q → %q %v, want %q", in, m.String(), err, want)
		}
	}
	for _, in := range []string{"", "-1", "0,05", "0.0000001", "abc", "1e-3"} {
		if _, err := ParseMicros(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestParseCSV(t *testing.T) {
	rates, markets, err := ParseCSV(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 8 || markets["AT"] != "Rest of Western Europe" || markets["TR"] != "Turkey" || len(markets) != 4 {
		t.Fatalf("rates %d markets %v", len(rates), markets)
	}
	// Semicolon-delimited (Excel in many locales), BOM, upper-case header,
	// tier_upper without tier_lower.
	semi := "\xef\xbb\xbfMarket;Countries;Category;Price;Currency;Effective_From;Tier_Upper\nGermany;DE;UTILITY;0.02;eur;2026-01-01;100\n"
	rates, markets, err = ParseCSV(strings.NewReader(semi))
	if err != nil {
		t.Fatal(err)
	}
	r := rates[0]
	if r.Currency != "EUR" || r.Category != "utility" || *r.TierLower != 1 || *r.TierUpper != 100 || markets["DE"] != "Germany" {
		t.Fatalf("semicolon file: %+v %v", r, markets)
	}

	bad := []struct{ csv, want string }{
		{"", "empty"},
		{"market,countries\n", `missing column "category"`},
		{"market,countries,category,price,currency,effective_from\n", "no rate rows"},
		{"market,countries,category,price,currency,effective_from\nX,TR,promo,0.1,USD,2026-01-01\n", `category "promo"`},
		{"market,countries,category,price,currency,effective_from\nX,TR,marketing,0,1,USD,2026-01-01\n", "line 2"},
		{"market,countries,category,price,currency,effective_from\nX,TR,marketing,-1,USD,2026-01-01\n", "not a price"},
		{"market,countries,category,price,currency,effective_from\nX,TR,marketing,0.1,US,2026-01-01\n", "three-letter"},
		{"market,countries,category,price,currency,effective_from\nX,TR,marketing,0.1,USD,01.01.2026\n", "YYYY-MM-DD"},
		{"market,countries,category,price,currency,effective_from\nX,TUR,marketing,0.1,USD,2026-01-01\n", "alpha-2"},
		{"market,countries,category,price,currency,effective_from\nX,TR,marketing,0.1,USD,2026-01-01\nY,TR,utility,0.1,USD,2026-01-01\n", "two markets"},
		{"market,countries,category,price,currency,effective_from\nX,TR,marketing,0.1,USD,2026-01-01\nX,,marketing,0.2,USD,2026-01-01\n", "duplicates line 2"},
		{"market,countries,category,price,currency,effective_from,tier_lower,tier_upper\nX,TR,utility,0.1,USD,2026-01-01,10,5\n", "below tier_lower"},
		{"market,countries,category,price,currency,effective_from,tier_lower\nX,TR,utility,0.1,USD,2026-01-01,zero\n", "positive whole number"},
	}
	for _, tc := range bad {
		_, _, err := ParseCSV(strings.NewReader(tc.csv))
		var ie *ImportError
		if !errors.As(err, &ie) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want %q", tc.csv, err, tc.want)
		}
	}
}
