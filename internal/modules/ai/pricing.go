package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Price is one row of the price table: prices per 1M tokens in Currency.
// Optional rates are nil when not known.
type Price struct {
	ID            uuid.UUID
	Kind          Kind
	Pattern       string // exact model id or a prefix ending in '*'
	Currency      string
	Input         *big.Rat
	CachedInput   *big.Rat
	CacheWrite5m  *big.Rat
	CacheWrite1h  *big.Rat
	Output        *big.Rat
	Rules         Rules
	EffectiveFrom time.Time  // a date (UTC midnight), inclusive
	EffectiveTo   *time.Time // a date, exclusive; nil = open-ended
}

// Rates are per-1M prices written as decimal strings; nil = unchanged (in
// an override) or unknown.
type Rates struct {
	Input        *string `json:"input,omitempty"`
	CachedInput  *string `json:"cached_input,omitempty"`
	CacheWrite5m *string `json:"cache_write_5m,omitempty"`
	CacheWrite1h *string `json:"cache_write_1h,omitempty"`
	Output       *string `json:"output,omitempty"`
}

// Threshold replaces rates when a request's total input (cached and
// cache writes included) exceeds InputTokensOver, e.g. long-context prices
// above 200K tokens. The highest exceeded threshold applies.
type Threshold struct {
	InputTokensOver int64 `json:"input_tokens_over"`
	Rates
}

// Window changes prices during recurring hours, e.g. DeepSeek's peak hours.
// Weekdays are ISO numbers (1 = Monday … 7 = Sunday; empty = every day);
// Start and End are "HH:MM" in TimeZone (default UTC), End exclusive; a
// window may wrap past midnight. Rates (when given) replace the prices,
// then Multiplier (when given) multiplies every rate. The first matching
// window applies.
type Window struct {
	Label      string  `json:"label,omitempty"`
	Weekdays   []int   `json:"weekdays,omitempty"`
	Start      string  `json:"start"`
	End        string  `json:"end"`
	TimeZone   string  `json:"time_zone,omitempty"`
	Multiplier *string `json:"multiplier,omitempty"`
	Rates
}

// Rules are the pricing rules of a row (ai_model_prices.rules).
type Rules struct {
	Thresholds []Threshold `json:"thresholds,omitempty"`
	Windows    []Window    `json:"windows,omitempty"`
}

// ParseRules decodes and validates rules JSON ("{}" or empty = none).
func ParseRules(b []byte) (Rules, error) {
	var r Rules
	if len(b) == 0 {
		return r, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("pricing rules: %w", err)
	}
	return r, r.Validate()
}

// Validate checks every rate, threshold and window.
func (r Rules) Validate() error {
	for i, t := range r.Thresholds {
		if t.InputTokensOver <= 0 {
			return fmt.Errorf("pricing rules: threshold %d: input_tokens_over must be positive", i+1)
		}
		if err := t.Rates.validate(); err != nil {
			return fmt.Errorf("pricing rules: threshold %d: %w", i+1, err)
		}
	}
	for i, w := range r.Windows {
		if err := w.validate(); err != nil {
			return fmt.Errorf("pricing rules: window %d: %w", i+1, err)
		}
	}
	return nil
}

func (rt Rates) validate() error {
	for _, v := range []*string{rt.Input, rt.CachedInput, rt.CacheWrite5m, rt.CacheWrite1h, rt.Output} {
		if v == nil {
			continue
		}
		if _, err := ParseAmount(*v); err != nil {
			return err
		}
	}
	return nil
}

func (w Window) validate() error {
	for _, d := range w.Weekdays {
		if d < 1 || d > 7 {
			return fmt.Errorf("weekday %d is not 1 (Monday) … 7 (Sunday)", d)
		}
	}
	if _, err := clock(w.Start); err != nil {
		return err
	}
	if _, err := clock(w.End); err != nil {
		return err
	}
	if w.Start == w.End {
		return errors.New("start and end are equal")
	}
	if _, err := w.location(); err != nil {
		return err
	}
	if w.Multiplier != nil {
		if _, err := ParseAmount(*w.Multiplier); err != nil {
			return err
		}
	}
	if w.Multiplier == nil && w.Rates == (Rates{}) {
		return errors.New("a window needs a multiplier or rates")
	}
	return w.Rates.validate()
}

func (w Window) location() (*time.Location, error) {
	if w.TimeZone == "" || w.TimeZone == "UTC" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(w.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("unknown time zone %q", w.TimeZone)
	}
	return loc, nil
}

// clock parses "HH:MM" into minutes after midnight.
func clock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("time %q is not HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// matches reports whether at falls in the window.
func (w Window) matches(at time.Time) bool {
	loc, err := w.location()
	if err != nil {
		return false
	}
	t := at.In(loc)
	start, _ := clock(w.Start)
	end, _ := clock(w.End)
	m := t.Hour()*60 + t.Minute()
	day := isoWeekday(t)
	if start < end {
		return (len(w.Weekdays) == 0 || slices.Contains(w.Weekdays, day)) && m >= start && m < end
	}
	// Wraps past midnight: the part after midnight belongs to the window
	// that started the previous day.
	if m >= start {
		return len(w.Weekdays) == 0 || slices.Contains(w.Weekdays, day)
	}
	if m < end {
		prev := day - 1
		if prev == 0 {
			prev = 7
		}
		return len(w.Weekdays) == 0 || slices.Contains(w.Weekdays, prev)
	}
	return false
}

func isoWeekday(t time.Time) int {
	d := int(t.Weekday())
	if d == 0 {
		return 7
	}
	return d
}

// ParseAmount parses a non-negative decimal ("0.15", "2").
func ParseAmount(s string) (*big.Rat, error) {
	s = strings.TrimSpace(s)
	r, ok := new(big.Rat).SetString(s)
	if !ok || strings.ContainsAny(s, "/eE") {
		return nil, fmt.Errorf("%q is not a decimal number", s)
	}
	if r.Sign() < 0 {
		return nil, fmt.Errorf("%q is negative", s)
	}
	return r, nil
}

func ratOf(s *string) *big.Rat {
	if s == nil {
		return nil
	}
	r, err := ParseAmount(*s)
	if err != nil {
		return nil
	}
	return r
}

// Matches reports whether the row's pattern covers model.
func (p Price) Matches(model string) bool {
	if prefix, ok := strings.CutSuffix(p.Pattern, "*"); ok {
		return strings.HasPrefix(model, prefix)
	}
	return p.Pattern == model
}

// ValidAt reports whether the row applies at t (compared by UTC date).
func (p Price) ValidAt(t time.Time) bool {
	d := dateOf(t)
	if d.Before(p.EffectiveFrom) {
		return false
	}
	return p.EffectiveTo == nil || d.Before(*p.EffectiveTo)
}

func dateOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// SelectPrice picks the row for a request at t: rows of the kind whose
// pattern matches and that are valid at t; an exact id beats a prefix, a
// longer prefix beats a shorter one, then the latest effective_from wins.
func SelectPrice(prices []Price, kind Kind, model string, at time.Time) (Price, bool) {
	var best Price
	found := false
	better := func(a, b Price) bool { // a better than b
		ae, be := !strings.HasSuffix(a.Pattern, "*"), !strings.HasSuffix(b.Pattern, "*")
		if ae != be {
			return ae
		}
		if len(a.Pattern) != len(b.Pattern) {
			return len(a.Pattern) > len(b.Pattern)
		}
		return a.EffectiveFrom.After(b.EffectiveFrom)
	}
	for _, p := range prices {
		if p.Kind != kind || !p.Matches(model) || !p.ValidAt(at) {
			continue
		}
		if !found || better(p, best) {
			best, found = p, true
		}
	}
	return best, found
}

// Cost is the priced result of one request.
type Cost struct {
	Amount   *big.Rat
	Currency string
	// Estimated is set when a rate the usage needed was unknown and the
	// input price stood in for it (e.g. no cache-write price).
	Estimated bool
	// Applied names the rules used ("threshold>200000", "window:peak").
	Applied []string
}

// String renders the amount with 6 decimals (numeric(12,6)).
func (c Cost) String() string { return FormatAmount(c.Amount) }

// FormatAmount rounds half up to 6 decimals.
func FormatAmount(r *big.Rat) string {
	if r == nil {
		return "0.000000"
	}
	return r.FloatString(6)
}

var million = big.NewRat(1_000_000, 1)

// PriceUsage computes the cost of u under p at time at. Pure: no I/O.
func PriceUsage(p Price, u Usage, at time.Time) (Cost, error) {
	if p.Input == nil || p.Output == nil {
		return Cost{}, errors.New("pricing: the row has no input or output price")
	}
	rates := struct{ in, cached, cw5, cw1, out *big.Rat }{p.Input, p.CachedInput, p.CacheWrite5m, p.CacheWrite1h, p.Output}
	var applied []string
	overlay := func(r Rates) {
		if v := ratOf(r.Input); v != nil {
			rates.in = v
		}
		if v := ratOf(r.CachedInput); v != nil {
			rates.cached = v
		}
		if v := ratOf(r.CacheWrite5m); v != nil {
			rates.cw5 = v
		}
		if v := ratOf(r.CacheWrite1h); v != nil {
			rates.cw1 = v
		}
		if v := ratOf(r.Output); v != nil {
			rates.out = v
		}
	}
	// Highest exceeded threshold.
	total := u.TotalInput()
	var th *Threshold
	for i := range p.Rules.Thresholds {
		t := &p.Rules.Thresholds[i]
		if total > t.InputTokensOver && (th == nil || t.InputTokensOver > th.InputTokensOver) {
			th = t
		}
	}
	if th != nil {
		overlay(th.Rates)
		applied = append(applied, fmt.Sprintf("threshold>%d", th.InputTokensOver))
	}
	for _, w := range p.Rules.Windows {
		if !w.matches(at) {
			continue
		}
		overlay(w.Rates)
		if m := ratOf(w.Multiplier); m != nil {
			for _, r := range []**big.Rat{&rates.in, &rates.cached, &rates.cw5, &rates.cw1, &rates.out} {
				if *r != nil {
					*r = new(big.Rat).Mul(*r, m)
				}
			}
		}
		label := w.Label
		if label == "" {
			label = w.Start + "-" + w.End
		}
		applied = append(applied, "window:"+label)
		break
	}
	cost := Cost{Amount: new(big.Rat), Currency: p.Currency, Applied: applied}
	add := func(tokens int64, rate *big.Rat) {
		if tokens <= 0 {
			return
		}
		if rate == nil {
			rate = rates.in
			cost.Estimated = true
		}
		part := new(big.Rat).Mul(big.NewRat(tokens, 1), rate)
		cost.Amount.Add(cost.Amount, part.Quo(part, million))
	}
	add(u.InputTokens, rates.in)
	add(u.CachedInputTokens, rates.cached)
	add(u.CacheWriteTokens, rates.cw5)
	add(u.CacheWrite1hTokens, rates.cw1)
	add(u.OutputTokens, rates.out)
	return cost, nil
}
