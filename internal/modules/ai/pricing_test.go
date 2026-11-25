package ai

import (
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func rat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, err := ParseAmount(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func basePrice(t *testing.T) Price {
	return Price{ID: uuid.New(), Kind: KindAnthropic, Pattern: "claude-sonnet-5-5", Currency: "USD",
		Input: rat(t, "2"), CachedInput: rat(t, "0.10"), CacheWrite5m: rat(t, "2.50"), CacheWrite1h: rat(t, "4"), Output: rat(t, "10"),
		EffectiveFrom: day(2026, 11, 25)}
}

func cost(t *testing.T, p Price, u Usage, at time.Time) Cost {
	t.Helper()
	c, err := PriceUsage(p, u, at)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPriceCacheSplit(t *testing.T) {
	p := basePrice(t)
	at := day(2026, 11, 26)
	// 1M of each kind: 2 + 0.10 + 2.50 + 4 + 10 = 18.60
	c := cost(t, p, Usage{InputTokens: 1e6, CachedInputTokens: 1e6, CacheWriteTokens: 1e6, CacheWrite1hTokens: 1e6, OutputTokens: 1e6}, at)
	if c.String() != "18.600000" || c.Currency != "USD" || c.Estimated {
		t.Fatalf("cost = %s %s estimated=%v", c, c.Currency, c.Estimated)
	}
	// Small numbers keep six decimals: 1234 input at 2/1M = 0.002468; 77 output at 10/1M = 0.00077.
	c = cost(t, p, Usage{InputTokens: 1234, OutputTokens: 77}, at)
	if c.String() != "0.003238" {
		t.Fatalf("cost = %s", c)
	}
	// Reasoning is part of output and not billed twice.
	c = cost(t, p, Usage{OutputTokens: 1000, ReasoningTokens: 600}, at)
	if c.String() != "0.010000" {
		t.Fatalf("reasoning billed twice: %s", c)
	}
	// Rounds half up at the sixth decimal: 1 token at 0.5/1M = 0.0000005 → 0.000001.
	p.Input = rat(t, "0.5")
	if c := cost(t, p, Usage{InputTokens: 1}, at); c.String() != "0.000001" {
		t.Fatalf("rounding = %s", c)
	}
	if c := cost(t, p, Usage{}, at); c.String() != "0.000000" {
		t.Fatalf("empty usage = %s", c)
	}
}

func TestPriceMissingRatesUseInputAndAreEstimated(t *testing.T) {
	p := basePrice(t)
	p.CachedInput, p.CacheWrite5m, p.CacheWrite1h = nil, nil, nil
	c := cost(t, p, Usage{InputTokens: 500_000, CachedInputTokens: 500_000}, day(2026, 12, 1))
	if c.String() != "2.000000" || !c.Estimated {
		t.Fatalf("cost = %s estimated=%v", c, c.Estimated)
	}
	// Not estimated when the missing rates are not needed.
	if c := cost(t, p, Usage{InputTokens: 10, OutputTokens: 10}, day(2026, 12, 1)); c.Estimated {
		t.Fatal("estimated without cached tokens")
	}
	if _, err := PriceUsage(Price{Input: rat(t, "1")}, Usage{}, time.Now()); err == nil {
		t.Fatal("a row without output price must be refused")
	}
}

func TestPriceThresholds(t *testing.T) {
	p := basePrice(t)
	p.Rules = Rules{Thresholds: []Threshold{
		{InputTokensOver: 200_000, Rates: Rates{Input: ptr("4"), Output: ptr("15")}},
		{InputTokensOver: 100_000, Rates: Rates{Input: ptr("3")}},
	}}
	at := day(2026, 11, 26)
	cases := []struct {
		name string
		u    Usage
		want string
		rule string
	}{
		{"at the threshold uses the base price", Usage{InputTokens: 100_000}, "0.200000", ""},
		{"over 100K uses the first tier", Usage{InputTokens: 100_001}, "0.300003", "threshold>100000"},
		// Cached and cache-write tokens count toward the threshold.
		{"cached tokens count toward the threshold", Usage{InputTokens: 50_000, CachedInputTokens: 60_000}, "0.156000", "threshold>100000"},
		{"over 200K uses the highest tier, output too", Usage{InputTokens: 250_000, OutputTokens: 1000}, "1.015000", "threshold>200000"},
	}
	for _, c := range cases {
		got := cost(t, p, c.u, at)
		if got.String() != c.want {
			t.Errorf("%s: cost = %s, want %s", c.name, got, c.want)
		}
		if strings.Join(got.Applied, ",") != c.rule {
			t.Errorf("%s: applied = %v, want %q", c.name, got.Applied, c.rule)
		}
	}
}

func deepseekPrice(t *testing.T) Price {
	seed, err := SeedPrices()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range seed {
		if s.Pattern == "deepseek-flash" {
			p := Price{Kind: s.Kind, Pattern: s.Pattern, Currency: s.Currency, Input: rat(t, s.Input), CachedInput: ratOf(s.CachedInput),
				Output: rat(t, s.Output), Rules: s.Rules, EffectiveFrom: s.EffectiveFrom}
			return p
		}
	}
	t.Fatal("deepseek-flash not seeded")
	return Price{}
}

func TestPriceDeepSeekPeakHours(t *testing.T) {
	p := deepseekPrice(t)
	u := Usage{InputTokens: 1e6, CachedInputTokens: 1e6, OutputTokens: 1e6} // off-peak 0.15 + 0.003 + 0.60
	// 2026-11-30 is a Monday.
	mon := func(h, m int) time.Time { return time.Date(2026, 11, 30, h, m, 0, 0, time.UTC) }
	cases := []struct {
		at   time.Time
		want string
	}{
		{mon(0, 59), "0.753000"},
		{mon(1, 0), "1.506000"}, // peak starts at 01:00 (inclusive)
		{mon(3, 59), "1.506000"},
		{mon(4, 0), "0.753000"}, // ends at 04:00 (exclusive)
		{mon(5, 30), "0.753000"},
		{mon(6, 0), "1.506000"},
		{mon(9, 59), "1.506000"},
		{mon(10, 0), "0.753000"},
		{time.Date(2026, 12, 4, 7, 0, 0, 0, time.UTC), "1.506000"}, // Friday
		{time.Date(2026, 12, 5, 7, 0, 0, 0, time.UTC), "0.753000"}, // Saturday
		{time.Date(2026, 12, 6, 2, 0, 0, 0, time.UTC), "0.753000"}, // Sunday
		// The window is in UTC whatever the request's zone: 09:30 in Istanbul is 06:30 UTC.
		{time.Date(2026, 11, 30, 9, 30, 0, 0, time.FixedZone("TRT", 3*3600)), "1.506000"},
	}
	for _, c := range cases {
		if got := cost(t, p, u, c.at); got.String() != c.want {
			t.Errorf("%s: cost = %s, want %s (applied %v)", c.at.Format(time.RFC3339), got, c.want, got.Applied)
		}
	}
}

func TestPriceWindowRatesAndWrap(t *testing.T) {
	p := basePrice(t)
	p.Rules = Rules{Windows: []Window{{Label: "night", Start: "22:00", End: "02:00", Weekdays: []int{5},
		Rates: Rates{Output: ptr("5")}, TimeZone: "Europe/Istanbul"}}}
	u := Usage{OutputTokens: 1e6}
	ist, _ := time.LoadLocation("Europe/Istanbul")
	cases := []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 12, 4, 23, 0, 0, 0, ist), "5.000000"}, // Friday night
		{time.Date(2026, 12, 5, 1, 0, 0, 0, ist), "5.000000"},  // after midnight, window started Friday
		{time.Date(2026, 12, 5, 2, 0, 0, 0, ist), "10.000000"}, // end exclusive
		{time.Date(2026, 12, 5, 23, 0, 0, 0, ist), "10.000000"},
		{time.Date(2026, 12, 4, 1, 0, 0, 0, ist), "10.000000"}, // Friday 01:00 belongs to Thursday's window
	}
	for _, c := range cases {
		if got := cost(t, p, u, c.at); got.String() != c.want {
			t.Errorf("%s: cost = %s, want %s", c.at.Format(time.RFC3339), got, c.want)
		}
	}
	// A threshold and a window combine: the threshold first, then the multiplier.
	p.Rules = Rules{Thresholds: []Threshold{{InputTokensOver: 10, Rates: Rates{Input: ptr("3")}}},
		Windows: []Window{{Start: "00:00", End: "23:59", Multiplier: ptr("2")}}}
	if got := cost(t, p, Usage{InputTokens: 1e6}, day(2026, 12, 1).Add(time.Hour)); got.String() != "6.000000" || len(got.Applied) != 2 {
		t.Fatalf("combined = %s %v", got, got.Applied)
	}
}

func TestSelectPriceEffectiveDatesAndPatterns(t *testing.T) {
	mk := func(pattern string, from time.Time, to *time.Time, in string) Price {
		return Price{ID: uuid.New(), Kind: KindGemini, Pattern: pattern, Currency: "USD", Input: rat(t, in), Output: rat(t, "1"),
			EffectiveFrom: from, EffectiveTo: to}
	}
	dec1 := day(2026, 12, 1)
	prices := []Price{
		mk("gemini-4-hydrogen", day(2026, 11, 25), nil, "1.50"),
		mk("gemini-4-hydrogen", dec1, nil, "1.80"), // announced increase
		mk("gemini-4-*", day(2026, 1, 1), nil, "9"),
		mk("gemini-*", day(2026, 1, 1), nil, "99"),
		mk("gemini-3-old", day(2026, 1, 1), &dec1, "0.5"),
		{ID: uuid.New(), Kind: KindOpenAI, Pattern: "gemini-4-hydrogen", Input: rat(t, "7"), Output: rat(t, "7"), EffectiveFrom: day(2026, 1, 1)},
	}
	cases := []struct {
		model string
		at    time.Time
		want  string // input price, "" = none
	}{
		{"gemini-4-hydrogen", time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC), "1.50"},
		{"gemini-4-hydrogen", dec1, "1.80"},
		{"gemini-4-hydrogen", day(2027, 3, 1), "1.80"},
		{"gemini-4-hydrogen", day(2026, 11, 24), "9"}, // before the exact row exists, the prefix applies
		{"gemini-4-helium", dec1, "9"},                // the longer prefix wins
		{"gemini-5", dec1, "99"},
		{"gemini-3-old", day(2026, 11, 30), "0.5"},
		{"gemini-3-old", dec1, "99"}, // effective_to is exclusive
		{"claude-x", dec1, ""},
		{"gemini-4-hydrogen", day(2025, 1, 1), ""},
	}
	for _, c := range cases {
		p, ok := SelectPrice(prices, KindGemini, c.model, c.at)
		got := ""
		if ok {
			got = trimZeros(p.Input.FloatString(6))
		}
		if got != trimZeros(c.want) {
			t.Errorf("%s @ %s: input = %q, want %q", c.model, c.at.Format(time.DateOnly), got, c.want)
		}
	}
	// Dates compare in UTC: 2026-12-01 01:00 in Istanbul is still 2026-11-30 UTC.
	if p, _ := SelectPrice(prices, KindGemini, "gemini-4-hydrogen", time.Date(2026, 12, 1, 1, 0, 0, 0, time.FixedZone("TRT", 3*3600))); p.Input.FloatString(2) != "1.50" {
		t.Fatalf("zone handling: %s", p.Input.FloatString(2))
	}
}

func TestRulesValidation(t *testing.T) {
	bad := []string{
		`{"thresholds":[{"input_tokens_over":0,"input":"1"}]}`,
		`{"thresholds":[{"input_tokens_over":10,"input":"-1"}]}`,
		`{"windows":[{"start":"25:00","end":"02:00","multiplier":"2"}]}`,
		`{"windows":[{"start":"01:00","end":"01:00","multiplier":"2"}]}`,
		`{"windows":[{"start":"01:00","end":"02:00"}]}`,
		`{"windows":[{"start":"01:00","end":"02:00","multiplier":"2","weekdays":[0]}]}`,
		`{"windows":[{"start":"01:00","end":"02:00","multiplier":"2","time_zone":"Mars/Base"}]}`,
		`{"unknown":true}`,
		`{"thresholds":[{"input_tokens_over":10,"input":"1e3"}]}`,
	}
	for _, b := range bad {
		if _, err := ParseRules([]byte(b)); err == nil {
			t.Errorf("accepted %s", b)
		}
	}
	if r, err := ParseRules([]byte(`{}`)); err != nil || len(r.Windows) != 0 {
		t.Fatal(err)
	}
}

func TestPriceInputValidate(t *testing.T) {
	ok := PriceInput{Kind: KindOpenAI, Pattern: "gpt-6*", Input: "2.500000", Output: "10", EffectiveFrom: day(2026, 11, 25)}
	if err := ok.Validate(); err != nil || ok.Input != "2.5" || ok.Currency != "USD" {
		t.Fatalf("%v %+v", err, ok)
	}
	cases := map[string]PriceInput{
		"provider_kind":  {Kind: "mistral", Pattern: "m", Input: "1", Output: "1", EffectiveFrom: day(2026, 1, 1)},
		"model_pattern":  {Kind: KindOpenAI, Pattern: "has space", Input: "1", Output: "1", EffectiveFrom: day(2026, 1, 1)},
		"input":          {Kind: KindOpenAI, Pattern: "m", Input: "abc", Output: "1", EffectiveFrom: day(2026, 1, 1)},
		"effective_to":   {Kind: KindOpenAI, Pattern: "m", Input: "1", Output: "1", EffectiveFrom: day(2026, 1, 1), EffectiveTo: ptr(day(2026, 1, 1))},
		"effective_from": {Kind: KindOpenAI, Pattern: "m", Input: "1", Output: "1"},
		"currency":       {Kind: KindOpenAI, Pattern: "m", Input: "1", Output: "1", EffectiveFrom: day(2026, 1, 1), Currency: "usd"},
		"cached_input":   {Kind: KindOpenAI, Pattern: "m", Input: "1", Output: "1", EffectiveFrom: day(2026, 1, 1), CachedInput: ptr("9999999")},
	}
	for field, in := range cases {
		err := in.Validate()
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: got %v", field, err)
		}
	}
}
