package ai

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PriceInput is a price row to store (seed, LiteLLM import or admin edit).
// Rates are per 1M tokens.
type PriceInput struct {
	Kind          Kind
	Pattern       string
	Currency      string
	Input         string
	CachedInput   *string
	CacheWrite5m  *string
	CacheWrite1h  *string
	Output        string
	Rules         Rules
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	Source        string // seed | litellm | manual
	SourceURL     string
	Notes         string
}

var (
	patternRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,198}\*?$`)
	currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)
)

// maxRate is the largest per-1M price numeric(12,6) holds.
var maxRate = big.NewRat(999_999, 1)

// Validate checks the row; the error names the field at fault.
func (in *PriceInput) Validate() error {
	in.Pattern = strings.TrimSpace(in.Pattern)
	if in.Currency == "" {
		in.Currency = "USD"
	}
	switch {
	case !slices.Contains(append(slices.Clone(Kinds), KindFake), in.Kind):
		return &FieldError{"provider_kind", "Unknown provider kind."}
	case !patternRe.MatchString(in.Pattern):
		return &FieldError{"model_pattern", "Use a model id, or a prefix ending in * (letters, digits and . _ : / @ -)."}
	case !currencyRe.MatchString(in.Currency):
		return &FieldError{"currency", "Currency must be a three-letter ISO code."}
	case in.EffectiveFrom.IsZero():
		return &FieldError{"effective_from", "Effective date is required."}
	case in.EffectiveTo != nil && !in.EffectiveTo.After(in.EffectiveFrom):
		return &FieldError{"effective_to", "End date must be after the start date."}
	}
	in.EffectiveFrom = dateOf(in.EffectiveFrom)
	if in.EffectiveTo != nil {
		d := dateOf(*in.EffectiveTo)
		in.EffectiveTo = &d
	}
	for _, f := range []struct {
		name string
		v    *string
	}{{"input", &in.Input}, {"output", &in.Output}, {"cached_input", in.CachedInput},
		{"cache_write_5m", in.CacheWrite5m}, {"cache_write_1h", in.CacheWrite1h}} {
		if f.v == nil {
			continue
		}
		r, err := ParseAmount(*f.v)
		if err != nil {
			return &FieldError{f.name, "Enter a non-negative decimal price per 1M tokens."}
		}
		if r.Cmp(maxRate) > 0 {
			return &FieldError{f.name, "Price is too large."}
		}
		*f.v = trimZeros(r.FloatString(6))
	}
	if err := in.Rules.Validate(); err != nil {
		return &FieldError{"rules", err.Error()}
	}
	return nil
}

// FieldError is a validation problem with one field.
type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}

//go:embed seed_prices.json
var seedJSON []byte

// SeedDate is the effective date of the shipped seed prices (the day they
// were read from the providers' pricing pages).
var SeedDate = time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)

// SeedPrices returns the shipped list prices. They are unconfirmed: an
// admin checks them against the source page and confirms or edits them.
func SeedPrices() ([]PriceInput, error) {
	var raw []struct {
		Kind         Kind            `json:"provider_kind"`
		Pattern      string          `json:"model_pattern"`
		Input        string          `json:"input"`
		CachedInput  *string         `json:"cached_input"`
		CacheWrite5m *string         `json:"cache_write_5m"`
		CacheWrite1h *string         `json:"cache_write_1h"`
		Output       string          `json:"output"`
		Rules        json.RawMessage `json:"rules"`
		SourceURL    string          `json:"source_url"`
		Notes        string          `json:"notes"`
	}
	if err := json.Unmarshal(seedJSON, &raw); err != nil {
		return nil, fmt.Errorf("seed prices: %w", err)
	}
	out := make([]PriceInput, 0, len(raw))
	for _, r := range raw {
		rules, err := ParseRules(r.Rules)
		if err != nil {
			return nil, fmt.Errorf("seed prices %s: %w", r.Pattern, err)
		}
		in := PriceInput{Kind: r.Kind, Pattern: r.Pattern, Currency: "USD", Input: r.Input, CachedInput: r.CachedInput,
			CacheWrite5m: r.CacheWrite5m, CacheWrite1h: r.CacheWrite1h, Output: r.Output, Rules: rules,
			EffectiveFrom: SeedDate, Source: "seed", SourceURL: r.SourceURL, Notes: r.Notes}
		if err := in.Validate(); err != nil {
			return nil, fmt.Errorf("seed prices %s: %w", r.Pattern, err)
		}
		out = append(out, in)
	}
	return out, nil
}

// LiteLLMSource is recorded as the source of imported rows.
const LiteLLMSource = "https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json"

// litellmProviders maps LiteLLM's litellm_provider to our kinds and the
// prefix LiteLLM puts before the provider's own model id.
var litellmProviders = map[string]struct {
	kind   Kind
	prefix string
}{
	"openai":    {KindOpenAI, ""},
	"anthropic": {KindAnthropic, ""},
	"gemini":    {KindGemini, "gemini/"},
	"deepseek":  {KindDeepSeek, "deepseek/"},
}

// aboveRe matches LiteLLM's long-context fields, e.g.
// input_cost_per_token_above_200k_tokens or
// cache_read_input_token_cost_above_200k_tokens.
var aboveRe = regexp.MustCompile(`^(input_cost_per_token|output_cost_per_token|cache_read_input_token_cost|cache_creation_input_token_cost)_above_(\d+)k_tokens$`)

// ImportResult summarises a LiteLLM import.
type ImportResult struct {
	Prices  []PriceInput
	Skipped int // entries of other providers or without per-token prices
}

// maxLiteLLMBytes bounds the uploaded file (the full list is ~1.5 MB).
const maxLiteLLMBytes = 16 << 20

// ParseLiteLLM reads LiteLLM's model_prices_and_context_window.json (MIT
// licensed; prices per token) and returns per-1M rows for the providers
// Messora supports, effective from the given date and unconfirmed. The file
// is uploaded by an admin; Messora never fetches it.
func ParseLiteLLM(r io.Reader, effective time.Time) (ImportResult, error) {
	dec := json.NewDecoder(io.LimitReader(r, maxLiteLLMBytes))
	dec.UseNumber()
	var doc map[string]map[string]any
	if err := dec.Decode(&doc); err != nil {
		return ImportResult{}, fmt.Errorf("the file is not LiteLLM's price list (a JSON object of models): %w", err)
	}
	names := make([]string, 0, len(doc))
	for k := range doc {
		names = append(names, k)
	}
	sort.Strings(names)
	var res ImportResult
	for _, name := range names {
		e := doc[name]
		prov, _ := e["litellm_provider"].(string)
		m, ok := litellmProviders[prov]
		if !ok || name == "sample_spec" {
			res.Skipped++
			continue
		}
		if mode, _ := e["mode"].(string); mode != "" && mode != "chat" && mode != "responses" {
			res.Skipped++
			continue
		}
		in, ok1 := perMillion(e["input_cost_per_token"])
		out, ok2 := perMillion(e["output_cost_per_token"])
		if !ok1 || !ok2 {
			res.Skipped++
			continue
		}
		model := strings.TrimPrefix(name, m.prefix)
		if !patternRe.MatchString(model) {
			res.Skipped++
			continue
		}
		p := PriceInput{Kind: m.kind, Pattern: model, Currency: "USD", Input: in, Output: out, EffectiveFrom: effective,
			Source: "litellm", SourceURL: LiteLLMSource}
		if v, ok := perMillion(e["cache_read_input_token_cost"]); ok {
			p.CachedInput = &v
		}
		if v, ok := perMillion(e["cache_creation_input_token_cost"]); ok {
			p.CacheWrite5m = &v
		}
		if v, ok := perMillion(e["cache_creation_input_token_cost_above_1hr"]); ok {
			p.CacheWrite1h = &v
		}
		thresholds := map[int64]*Threshold{}
		for k, v := range e {
			mm := aboveRe.FindStringSubmatch(k)
			if mm == nil {
				continue
			}
			rate, ok := perMillion(v)
			if !ok {
				continue
			}
			n, _ := strconv.ParseInt(mm[2], 10, 64)
			t := thresholds[n*1000]
			if t == nil {
				t = &Threshold{InputTokensOver: n * 1000}
				thresholds[n*1000] = t
			}
			switch mm[1] {
			case "input_cost_per_token":
				t.Input = &rate
			case "output_cost_per_token":
				t.Output = &rate
			case "cache_read_input_token_cost":
				t.CachedInput = &rate
			case "cache_creation_input_token_cost":
				t.CacheWrite5m = &rate
			}
		}
		for _, t := range thresholds {
			p.Rules.Thresholds = append(p.Rules.Thresholds, *t)
		}
		sort.Slice(p.Rules.Thresholds, func(i, j int) bool {
			return p.Rules.Thresholds[i].InputTokensOver < p.Rules.Thresholds[j].InputTokensOver
		})
		if err := p.Validate(); err != nil {
			res.Skipped++
			continue
		}
		res.Prices = append(res.Prices, p)
	}
	if len(res.Prices) == 0 {
		return res, errors.New("the file has no prices for OpenAI, Anthropic, Gemini or DeepSeek models")
	}
	return res, nil
}

// perMillion converts a per-token price (a JSON number) to per 1M tokens.
func perMillion(v any) (string, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return "", false
	}
	r, ok := new(big.Rat).SetString(n.String())
	if !ok || r.Sign() < 0 {
		return "", false
	}
	r.Mul(r, million)
	return trimZeros(r.FloatString(6)), true
}
