// Package metacost prices WhatsApp messages with Meta's per-message rate
// card. Meta publishes no pricing API: operators import the published rate
// card as CSV (docs/WHATSAPP.md, "Rate card"), and every billable message
// is priced from it when its status webhook reports the pricing category.
// Meta's own daily totals (pricing_analytics) are stored separately for
// reconciliation.
package metacost

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Categories a rate card row may price.
var Categories = []string{"marketing", "utility", "authentication", "authentication-international", "service"}

// OtherMarket is the market used for countries no other market lists
// (Meta's rate cards end with an "Other" row).
const OtherMarket = "Other"

// Micros is an amount in millionths of the currency unit (numeric(12,6)).
type Micros int64

var decimalRe = regexp.MustCompile(`^([0-9]{1,6})(?:\.([0-9]{1,6}))?$`)

// ParseMicros parses a non-negative decimal with at most six fractional
// digits ("0.0436", "1", "0.000125").
func ParseMicros(s string) (Micros, error) {
	m := decimalRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("%q is not a price (use a dot and at most 6 decimals, e.g. 0.0436)", s)
	}
	whole, _ := strconv.ParseInt(m[1], 10, 64)
	frac := m[2] + strings.Repeat("0", 6-len(m[2]))
	f, _ := strconv.ParseInt(frac, 10, 64)
	return Micros(whole*1_000_000 + f), nil
}

// String formats the amount with six decimals ("0.043600"), the form
// PostgreSQL's numeric(12,6) uses.
func (m Micros) String() string {
	return fmt.Sprintf("%d.%06d", int64(m)/1_000_000, int64(m)%1_000_000)
}

// Rate is one rate card row.
type Rate struct {
	Currency string
	Market   string
	Category string
	Price    Micros
	// TierLower/TierUpper bound the message's ordinal within the calendar
	// month (inclusive); nil = unbounded. Both nil = untiered.
	TierLower, TierUpper *int64
	EffectiveFrom        time.Time  // a date, UTC midnight
	EffectiveTo          *time.Time // exclusive; nil = still valid
	Source               string
}

// Card is a set of rates and the country → market mapping.
type Card struct {
	Rates   []Rate
	Markets map[string]string // ISO alpha-2 → market
	// Currency restricts the rates to one currency (the account's); empty
	// accepts any.
	Currency string
}

// NormalizeCategory maps Meta's spellings (webhook "authentication-international",
// analytics "AUTHENTICATION_INTERNATIONAL") to the rate card's.
func NormalizeCategory(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	if c == "authentication_international" {
		return "authentication-international"
	}
	return c
}

func (r Rate) covers(at time.Time, ordinal int64) bool {
	if at.Before(r.EffectiveFrom) || (r.EffectiveTo != nil && !at.Before(*r.EffectiveTo)) {
		return false
	}
	if r.TierLower != nil && ordinal < *r.TierLower {
		return false
	}
	return r.TierUpper == nil || ordinal <= *r.TierUpper
}

// RateFor returns the rate for a message to country in category sent at at,
// when monthlyVolume messages of the same market and category were already
// sent this month. The newest rate effective at that time wins; a tiered
// row beats an untiered one from the same date. Countries without their
// own market use the "Other" market.
func (c *Card) RateFor(country, category string, at time.Time, monthlyVolume int64) (Rate, bool) {
	category = NormalizeCategory(category)
	country = strings.ToUpper(country)
	at = at.UTC()
	markets := []string{}
	if m, ok := c.Markets[country]; ok {
		markets = append(markets, m)
	}
	markets = append(markets, OtherMarket)
	for _, market := range markets {
		var best *Rate
		for i := range c.Rates {
			r := &c.Rates[i]
			if r.Market != market || r.Category != category || (c.Currency != "" && r.Currency != c.Currency) ||
				!r.covers(at, monthlyVolume+1) {
				continue
			}
			if best == nil || r.EffectiveFrom.After(best.EffectiveFrom) ||
				(r.EffectiveFrom.Equal(best.EffectiveFrom) && best.TierLower == nil && r.TierLower != nil) ||
				(r.EffectiveFrom.Equal(best.EffectiveFrom) && r.Currency < best.Currency) {
				best = r
			}
		}
		if best != nil {
			return *best, true
		}
	}
	return Rate{}, false
}

// ── CSV import ─────────────────────────────────────────────────────────────

// MaxImportRows bounds one import.
const MaxImportRows = 20000

// ImportError lists the problems of a rejected file, by line.
type ImportError struct{ Problems []string }

func (e *ImportError) Error() string { return strings.Join(e.Problems, "; ") }

var (
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)
	requiredHd = []string{"market", "countries", "category", "price", "currency", "effective_from"}
)

// ParseCSV reads the rate card format documented in docs/WHATSAPP.md:
//
//	market,countries,category,price,currency,effective_from[,tier_lower,tier_upper]
//
// countries lists ISO alpha-2 codes separated by spaces, ";" or "|" (a
// market's countries need to be listed on one of its rows only). The file
// may use "," or ";" as the delimiter. It returns the rates and the country
// → market mapping, or *ImportError with every problem found.
func ParseCSV(r io.Reader) ([]Rate, map[string]string, error) {
	data, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return nil, nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	first, _, _ := bytes.Cut(data, []byte("\n"))
	cr := csv.NewReader(bytes.NewReader(data))
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		cr.Comma = ';'
	}
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return nil, nil, &ImportError{Problems: []string{"the file is empty or not CSV"}}
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var problems []string
	for _, h := range requiredHd {
		if _, ok := col[h]; !ok {
			problems = append(problems, fmt.Sprintf("missing column %q", h))
		}
	}
	if len(problems) > 0 {
		return nil, nil, &ImportError{Problems: problems}
	}
	get := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	markets := map[string]string{}
	var rates []Rate
	seen := map[string]int{}
	line := 1
	for {
		rec, err := cr.Read()
		line++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("line %d: %v", line, err))
			break
		}
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue // blank line
		}
		if len(rates) >= MaxImportRows {
			problems = append(problems, fmt.Sprintf("more than %d rows", MaxImportRows))
			break
		}
		bad := func(format string, a ...any) {
			problems = append(problems, fmt.Sprintf("line %d: ", line)+fmt.Sprintf(format, a...))
		}
		rt := Rate{Market: get(rec, "market"), Category: NormalizeCategory(get(rec, "category")),
			Currency: strings.ToUpper(get(rec, "currency")), Source: get(rec, "source")}
		if rt.Market == "" || len(rt.Market) > 120 {
			bad("market is required (at most 120 characters)")
		}
		if !slices.Contains(Categories, rt.Category) {
			bad("category %q is not one of %s", get(rec, "category"), strings.Join(Categories, ", "))
		}
		if !currencyRe.MatchString(rt.Currency) {
			bad("currency %q is not a three-letter code such as USD", get(rec, "currency"))
		}
		if p, err := ParseMicros(get(rec, "price")); err != nil {
			bad("%v", err)
		} else {
			rt.Price = p
		}
		if d, err := time.Parse(time.DateOnly, get(rec, "effective_from")); err != nil {
			bad("effective_from %q is not a date (YYYY-MM-DD)", get(rec, "effective_from"))
		} else {
			rt.EffectiveFrom = d
		}
		for _, f := range []struct {
			name string
			dst  **int64
		}{{"tier_lower", &rt.TierLower}, {"tier_upper", &rt.TierUpper}} {
			v := get(rec, f.name)
			if v == "" {
				continue
			}
			n, err := strconv.ParseInt(strings.ReplaceAll(v, "_", ""), 10, 64)
			if err != nil || n < 1 {
				bad("%s %q is not a positive whole number", f.name, v)
				continue
			}
			*f.dst = &n
		}
		if rt.TierUpper != nil && rt.TierLower == nil {
			one := int64(1)
			rt.TierLower = &one
		}
		if rt.TierLower != nil && rt.TierUpper != nil && *rt.TierUpper < *rt.TierLower {
			bad("tier_upper is below tier_lower")
		}
		for _, c := range strings.FieldsFunc(strings.ToUpper(get(rec, "countries")), func(r rune) bool {
			return r == ' ' || r == ';' || r == '|' || r == ','
		}) {
			if !countryRe.MatchString(c) {
				bad("country %q is not an ISO 3166 alpha-2 code", c)
				continue
			}
			if m, ok := markets[c]; ok && m != rt.Market {
				bad("country %s is listed in two markets (%s, %s)", c, m, rt.Market)
				continue
			}
			markets[c] = rt.Market
		}
		key := fmt.Sprintf("%s|%s|%s|%v|%s", rt.Currency, rt.Market, rt.Category, deref(rt.TierLower), rt.EffectiveFrom.Format(time.DateOnly))
		if prev, ok := seen[key]; ok {
			bad("duplicates line %d (same currency, market, category, tier and date)", prev)
		}
		seen[key] = line
		rates = append(rates, rt)
		if len(problems) > 50 {
			break
		}
	}
	if len(problems) == 0 && len(rates) == 0 {
		problems = append(problems, "the file has no rate rows")
	}
	if len(problems) > 0 {
		if len(problems) > 20 {
			problems = append(problems[:20], fmt.Sprintf("… and %d more", len(problems)-20))
		}
		return nil, nil, &ImportError{Problems: problems}
	}
	sort.SliceStable(rates, func(i, j int) bool { return rates[i].EffectiveFrom.Before(rates[j].EffectiveFrom) })
	return rates, markets, nil
}

func deref(p *int64) any {
	if p == nil {
		return "-"
	}
	return *p
}
