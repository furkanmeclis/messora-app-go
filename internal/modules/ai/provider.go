package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Default API endpoints.
const (
	OpenAIBaseURL    = "https://api.openai.com/v1"
	AnthropicBaseURL = "https://api.anthropic.com"
	GeminiBaseURL    = "https://generativelanguage.googleapis.com"
	DeepSeekBaseURL  = "https://api.deepseek.com"
	// anthropicVersion is the API version header for raw calls (model
	// list); the same version anthropic-sdk-go v2.15.0 sends.
	anthropicVersion = "2026-10-21"
)

// ProviderConfig is what a client needs to call one provider connection.
type ProviderConfig struct {
	Kind    Kind
	APIKey  string
	BaseURL string       // "" = the provider's default endpoint
	HTTP    *http.Client // nil = http.DefaultClient
	// Fake scripts the fake provider (nil = its default behaviour).
	Fake FakeScript
}

func (c ProviderConfig) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	switch c.Kind {
	case KindOpenAI:
		return OpenAIBaseURL
	case KindAnthropic:
		return AnthropicBaseURL
	case KindGemini:
		return GeminiBaseURL
	case KindDeepSeek:
		return DeepSeekBaseURL
	}
	return ""
}

func (c ProviderConfig) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// NewClient returns the generation client of a provider connection.
func NewClient(cfg ProviderConfig) (Client, error) {
	switch cfg.Kind {
	case KindOpenAI:
		return newOpenAI(cfg), nil
	case KindAnthropic:
		return newAnthropic(cfg), nil
	case KindGemini:
		return newGemini(cfg), nil
	case KindDeepSeek:
		return newDeepSeek(cfg), nil
	case KindFake:
		return &FakeClient{Script: cfg.Fake}, nil
	}
	return nil, fmt.Errorf("ai: unknown provider kind %q", cfg.Kind)
}

// ValidateBaseURL checks an endpoint override: https, a host, no
// credentials, query or fragment. Only DeepSeek and OpenAI-compatible
// connections accept one.
func ValidateBaseURL(kind Kind, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if kind != KindDeepSeek && kind != KindOpenAI {
		return "", errors.New("only DeepSeek and OpenAI-compatible connections accept a custom endpoint")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("the endpoint must be an https URL without credentials, query or fragment")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// maxListBytes bounds a model list response.
const maxListBytes = 8 << 20

// getJSON performs an authenticated GET and decodes the JSON body into v.
// A non-2xx answer becomes an *Error.
func getJSON(ctx context.Context, cfg ProviderConfig, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.base()+path, nil)
	if err != nil {
		return err
	}
	switch cfg.Kind {
	case KindAnthropic:
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", anthropicVersion)
	case KindGemini:
		req.Header.Set("x-goog-api-key", cfg.APIKey)
	default:
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	req.Header.Set("Accept", "application/json")
	res, err := cfg.httpClient().Do(req)
	if err != nil {
		return &Error{Kind: KindUnavailable, Provider: cfg.Kind, Message: scrub(err.Error(), cfg.APIKey)}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxListBytes))
	if err != nil {
		return &Error{Kind: KindUnavailable, Provider: cfg.Kind, Message: "could not read the response"}
	}
	if res.StatusCode/100 != 2 {
		return httpError(cfg.Kind, res.StatusCode, res.Header, body, cfg.APIKey)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return &Error{Kind: KindUnavailable, Provider: cfg.Kind, Status: res.StatusCode, Message: "unexpected response from the provider"}
	}
	return nil
}

// httpError classifies a raw error response with the provider's error body.
func httpError(kind Kind, status int, h http.Header, body []byte, key string) *Error {
	var b struct {
		Error json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(body, &b)
	var detail struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
		Status  string `json:"status"`
		Details struct {
			ErrorCode string `json:"error_code"`
		} `json:"details"`
	}
	if len(b.Error) > 0 && b.Error[0] == '"' { // {"error": "message"}
		_ = json.Unmarshal(b.Error, &detail.Message)
	} else {
		_ = json.Unmarshal(b.Error, &detail)
	}
	if detail.Message == "" {
		_ = json.Unmarshal(body, &detail) // DeepSeek-style top-level message
	}
	code := ""
	switch c := detail.Code.(type) {
	case string:
		code = c
	case float64:
		code = fmt.Sprint(int(c))
	}
	if detail.Details.ErrorCode != "" {
		code = detail.Details.ErrorCode
	}
	e := &Error{Kind: byStatus(status), Provider: kind, Status: status, Code: code, Message: scrub(shorten(detail.Message), key)}
	refine(e, detail.Type, detail.Status)
	if e.Kind == KindRateLimited {
		e.RetryAfter = retryAfter(h, time.Now())
	}
	return e
}

// refine adjusts the status-based kind with what each provider documents:
//   - OpenAI: a 429 with code insufficient_quota is billing, not a rate limit.
//   - Anthropic: every 429 is rate_limit_error; error_code
//     enforced_spend_limit_reached means a spend limit (do not retry);
//     billing_error is a billing problem; 529 / overloaded_error is overload.
//   - Gemini: an invalid key is a 400 INVALID_ARGUMENT whose message names
//     the key; RESOURCE_EXHAUSTED is a 429.
//   - DeepSeek: 402 is insufficient balance.
func refine(e *Error, typ, status string) {
	switch {
	case e.Code == "insufficient_quota" || e.Code == "enforced_spend_limit_reached" || typ == "billing_error":
		e.Kind = KindQuotaExceeded
	case typ == "overloaded_error":
		e.Kind = KindOverloaded
	case typ == "authentication_error" || typ == "permission_error" || status == "UNAUTHENTICATED" || status == "PERMISSION_DENIED":
		e.Kind = KindAuth
	case e.Provider == KindGemini && e.Status == http.StatusBadRequest &&
		(strings.Contains(strings.ToLower(e.Message), "api key") || e.Code == "API_KEY_INVALID"):
		e.Kind = KindAuth
	}
}

// ListModels reads the provider's model list and maps the fields each one
// reports. Nothing is guessed: what a provider does not report stays nil,
// apart from documented facts noted below.
func ListModels(ctx context.Context, cfg ProviderConfig) ([]ModelInfo, error) {
	switch cfg.Kind {
	case KindOpenAI:
		return listOpenAI(ctx, cfg)
	case KindAnthropic:
		return listAnthropic(ctx, cfg)
	case KindGemini:
		return listGemini(ctx, cfg)
	case KindDeepSeek:
		return listDeepSeek(ctx, cfg)
	case KindFake:
		return fakeModels(cfg)
	}
	return nil, fmt.Errorf("ai: unknown provider kind %q", cfg.Kind)
}

// listOpenAI: GET /v1/models returns only id, created, owned_by and an
// optional shutdown_date — no limits, capabilities or prices.
func listOpenAI(ctx context.Context, cfg ProviderConfig) ([]ModelInfo, error) {
	var res struct {
		Data []struct {
			ID           string `json:"id"`
			ShutdownDate any    `json:"shutdown_date"`
		} `json:"data"`
	}
	if err := getJSON(ctx, cfg, "/models", &res); err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(res.Data))
	for _, m := range res.Data {
		mi := ModelInfo{ID: m.ID, DisplayName: m.ID, Lifecycle: ptr("active")}
		if m.ShutdownDate != nil {
			mi.Lifecycle = ptr("deprecated")
		}
		out = append(out, mi)
	}
	return out, nil
}

type supported struct {
	Supported bool `json:"supported"`
}

// listAnthropic: GET /v1/models (paged with after_id, limit ≤ 1000) gives
// max_input_tokens, max_tokens and capabilities. Tool use has no flag
// because every Claude model supports it
// (https://docs.claude.com/en/docs/agents-and-tools/tool-use/overview).
func listAnthropic(ctx context.Context, cfg ProviderConfig) ([]ModelInfo, error) {
	var out []ModelInfo
	after := ""
	for page := 0; page < 20; page++ {
		var res struct {
			Data []struct {
				ID             string `json:"id"`
				DisplayName    string `json:"display_name"`
				MaxInputTokens *int64 `json:"max_input_tokens"`
				MaxTokens      *int64 `json:"max_tokens"`
				Lifecycle      string `json:"lifecycle"`
				Capabilities   *struct {
					ImageInput        *supported `json:"image_input"`
					StructuredOutputs *supported `json:"structured_outputs"`
					Thinking          *supported `json:"thinking"`
				} `json:"capabilities"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		q := "/v1/models?limit=1000"
		if after != "" {
			q += "&after_id=" + url.QueryEscape(after)
		}
		if err := getJSON(ctx, cfg, q, &res); err != nil {
			return nil, err
		}
		for _, m := range res.Data {
			mi := ModelInfo{ID: m.ID, DisplayName: m.DisplayName, ContextWindow: m.MaxInputTokens, MaxOutput: m.MaxTokens,
				Capabilities: Capabilities{Tools: ptr(true)}}
			if m.Lifecycle != "" {
				mi.Lifecycle = ptr(m.Lifecycle)
			}
			if c := m.Capabilities; c != nil {
				mi.Capabilities.Vision = flag(c.ImageInput)
				mi.Capabilities.StructuredOutput = flag(c.StructuredOutputs)
				mi.Capabilities.Thinking = flag(c.Thinking)
			}
			out = append(out, mi)
		}
		if !res.HasMore || res.LastID == "" {
			break
		}
		after = res.LastID
	}
	return out, nil
}

func flag(s *supported) *bool {
	if s == nil {
		return nil
	}
	return ptr(s.Supported)
}

// listGemini: GET /v1beta/models gives inputTokenLimit, outputTokenLimit,
// supportedGenerationMethods and thinking. Only models that support
// generateContent are kept.
func listGemini(ctx context.Context, cfg ProviderConfig) ([]ModelInfo, error) {
	var out []ModelInfo
	token := ""
	for page := 0; page < 20; page++ {
		var res struct {
			Models []struct {
				Name             string   `json:"name"`
				DisplayName      string   `json:"displayName"`
				InputTokenLimit  *int64   `json:"inputTokenLimit"`
				OutputTokenLimit *int64   `json:"outputTokenLimit"`
				Methods          []string `json:"supportedGenerationMethods"`
				Thinking         *bool    `json:"thinking"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := "/v1beta/models?pageSize=1000"
		if token != "" {
			q += "&pageToken=" + url.QueryEscape(token)
		}
		if err := getJSON(ctx, cfg, q, &res); err != nil {
			return nil, err
		}
		for _, m := range res.Models {
			if !slices.Contains(m.Methods, "generateContent") {
				continue
			}
			out = append(out, ModelInfo{ID: strings.TrimPrefix(m.Name, "models/"), DisplayName: m.DisplayName,
				ContextWindow: m.InputTokenLimit, MaxOutput: m.OutputTokenLimit, Capabilities: Capabilities{Thinking: m.Thinking}})
		}
		if res.NextPageToken == "" {
			break
		}
		token = res.NextPageToken
	}
	return out, nil
}

// listDeepSeek: GET /models gives context_window, max_output_tokens and
// input_modalities (vision = "image" among them, when reported).
func listDeepSeek(ctx context.Context, cfg ProviderConfig) ([]ModelInfo, error) {
	var res struct {
		Data []struct {
			ID              string   `json:"id"`
			ContextWindow   *int64   `json:"context_window"`
			MaxOutputTokens *int64   `json:"max_output_tokens"`
			InputModalities []string `json:"input_modalities"`
		} `json:"data"`
	}
	if err := getJSON(ctx, cfg, "/models", &res); err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(res.Data))
	for _, m := range res.Data {
		mi := ModelInfo{ID: m.ID, DisplayName: m.ID, ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutputTokens}
		if m.InputModalities != nil {
			mi.Capabilities.Vision = ptr(slices.Contains(m.InputModalities, "image"))
		}
		out = append(out, mi)
	}
	return out, nil
}

// CheckKey validates a key with the cheapest free call: the model list
// (OpenAI, Anthropic, Gemini) or the balance (DeepSeek). For Gemini a
// successful list does not guarantee that generation is enabled.
func CheckKey(ctx context.Context, cfg ProviderConfig) (KeyStatus, string) {
	var err error
	switch cfg.Kind {
	case KindDeepSeek:
		var res struct {
			IsAvailable  bool `json:"is_available"`
			BalanceInfos []struct {
				Currency     string `json:"currency"`
				TotalBalance string `json:"total_balance"`
			} `json:"balance_infos"`
		}
		if err = getJSON(ctx, cfg, "/user/balance", &res); err == nil {
			var parts []string
			for _, b := range res.BalanceInfos {
				parts = append(parts, b.TotalBalance+" "+b.Currency)
			}
			detail := "Balance: " + strings.Join(parts, ", ")
			if len(parts) == 0 {
				detail = ""
			}
			if !res.IsAvailable {
				return StatusNoBalance, strings.TrimSpace("The account has no usable balance. " + detail)
			}
			return StatusOK, detail
		}
	case KindFake:
		switch {
		case strings.Contains(cfg.APIKey, "invalid"):
			return StatusInvalid, "The fake provider rejects keys containing \"invalid\"."
		case strings.Contains(cfg.APIKey, "nobalance"):
			return StatusNoBalance, "The fake provider reports no balance for keys containing \"nobalance\"."
		}
		return StatusOK, "Fake provider (tests and development only)."
	default:
		var models []ModelInfo
		if models, err = ListModels(ctx, cfg); err == nil {
			detail := fmt.Sprintf("%d models available.", len(models))
			if cfg.Kind == KindGemini {
				detail += " Gemini lists models for any valid key; generation may still need billing to be enabled."
			}
			return StatusOK, detail
		}
	}
	var e *Error
	if errors.As(err, &e) {
		switch e.Kind {
		case KindAuth:
			return StatusInvalid, firstNonEmpty(e.Message, "The provider rejected the key.")
		case KindQuotaExceeded:
			return StatusNoBalance, firstNonEmpty(e.Message, "The account has no credit or reached its spend limit.")
		}
		return StatusError, firstNonEmpty(e.Message, string(e.Kind))
	}
	return StatusError, scrub(err.Error(), cfg.APIKey)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
