package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseAnthropicUsage(t *testing.T) {
	r, err := parseAnthropic(fixture(t, "anthropic_message.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	want := Usage{InputTokens: 120, CachedInputTokens: 3000, CacheWriteTokens: 1000, CacheWrite1hTokens: 500, OutputTokens: 80, ReasoningTokens: 30}
	if r.Usage != want {
		t.Fatalf("usage = %+v, want %+v", r.Usage, want)
	}
	if r.Usage.TotalInput() != 120+1500+3000 {
		t.Fatalf("total input = %d", r.Usage.TotalInput())
	}
	if r.Model != "claude-sonnet-5-5" || r.StopReason != StopEnd || string(r.Structured) != `{"reply":"Merhaba!","confidence":0.92}` {
		t.Fatalf("response = %+v", r)
	}
	// Without the 5m/1h split every cache write counts as the 5-minute kind.
	raw := strings.Replace(string(fixture(t, "anthropic_message.json")), `"cache_creation": {"ephemeral_5m_input_tokens": 1000, "ephemeral_1h_input_tokens": 500},`, "", 1)
	r, _ = parseAnthropic([]byte(raw), false)
	if r.Usage.CacheWriteTokens != 1500 || r.Usage.CacheWrite1hTokens != 0 {
		t.Fatalf("unsplit cache write = %+v", r.Usage)
	}

	r, err = parseAnthropic(fixture(t, "anthropic_tool_use.json"), false)
	if err != nil || r.StopReason != StopToolUse || len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "order_status" ||
		string(r.ToolCalls[0].Arguments) != `{"order_id": "A-17"}` || r.Text != "Checking your order." {
		t.Fatalf("tool use = %+v %v", r, err)
	}

	refusal := `{"model":"claude-opus-5-5","content":[],"stop_reason":"refusal","usage":{"input_tokens":5,"output_tokens":1}}`
	r, err = parseAnthropic([]byte(refusal), false)
	if !errors.Is(err, ErrRefusal) || r.Usage.InputTokens != 5 {
		t.Fatalf("refusal = %v %+v", err, r.Usage)
	}
	if _, err := parseAnthropic([]byte(`{"content":[{"type":"text","text":"not json"}],"stop_reason":"end_turn"}`), true); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("invalid structured output = %v", err)
	}
}

func TestParseOpenAIUsage(t *testing.T) {
	r, err := parseOpenAIResponse(fixture(t, "openai_response.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	// input_tokens (5000) includes cached reads (3000) and cache writes (1000).
	want := Usage{InputTokens: 1000, CachedInputTokens: 3000, CacheWriteTokens: 1000, OutputTokens: 400, ReasoningTokens: 250}
	if r.Usage != want || r.Usage.TotalInput() != 5000 {
		t.Fatalf("usage = %+v, want %+v", r.Usage, want)
	}
	if r.Model != "gpt-6.2-sol-2026-11-01" || string(r.Structured) != `{"reply":"Hello"}` || r.StopReason != StopEnd {
		t.Fatalf("response = %+v", r)
	}
	r, err = parseOpenAIResponse(fixture(t, "openai_function_call.json"), false)
	if err != nil || len(r.ToolCalls) != 1 || r.ToolCalls[0].ID != "call_abc" || string(r.ToolCalls[0].Arguments) != `{"order_id":"A-17"}` ||
		r.StopReason != StopToolUse || r.Usage.InputTokens != 90 {
		t.Fatalf("function call = %+v %v", r, err)
	}
	refusal := `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal","refusal":"I can't help with that."}]}],"usage":{"input_tokens":3,"output_tokens":4}}`
	if _, err := parseOpenAIResponse([]byte(refusal), true); !errors.Is(err, ErrRefusal) {
		t.Fatalf("refusal = %v", err)
	}
	incomplete := `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[],"usage":{"input_tokens":3,"output_tokens":4}}`
	if r, _ := parseOpenAIResponse([]byte(incomplete), false); r.StopReason != StopMaxTokens {
		t.Fatalf("incomplete = %s", r.StopReason)
	}
}

func TestParseGeminiUsage(t *testing.T) {
	var res genai.GenerateContentResponse
	if err := json.Unmarshal(fixture(t, "gemini_response.json"), &res); err != nil {
		t.Fatal(err)
	}
	r, err := parseGemini(&res, true)
	if err != nil {
		t.Fatal(err)
	}
	// promptTokenCount includes the cached part; thinking is billed as output.
	want := Usage{InputTokens: 800, CachedInputTokens: 1200, OutputTokens: 200, ReasoningTokens: 140}
	if r.Usage != want {
		t.Fatalf("usage = %+v, want %+v", r.Usage, want)
	}
	if r.Text != `{"reply":"Merhaba"}` || r.StopReason != StopEnd || r.Model != "gemini-4-hydrogen" {
		t.Fatalf("thought parts must not reach the answer: %+v", r)
	}
	blocked := genai.GenerateContentResponse{PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: "SAFETY"}}
	if _, err := parseGemini(&blocked, false); !errors.Is(err, ErrRefusal) {
		t.Fatalf("blocked = %v", err)
	}
	safety := genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonSafety}}}
	if _, err := parseGemini(&safety, false); !errors.Is(err, ErrRefusal) {
		t.Fatalf("safety = %v", err)
	}
	call := genai.GenerateContentResponse{Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonStop, Content: &genai.Content{
		Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "order_status", Args: map[string]any{"order_id": "A-17"}}}}}}}}
	if r, err := parseGemini(&call, true); err != nil || len(r.ToolCalls) != 1 || r.StopReason != StopToolUse || string(r.ToolCalls[0].Arguments) != `{"order_id":"A-17"}` {
		t.Fatalf("function call = %+v %v", r, err)
	}
}

func TestParseDeepSeekUsage(t *testing.T) {
	r, err := parseChatCompletion(KindDeepSeek, fixture(t, "deepseek_completion.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Usage{InputTokens: 300, CachedInputTokens: 700, OutputTokens: 50}); r.Usage != want {
		t.Fatalf("top-level hit/miss: %+v", r.Usage)
	}
	r, err = parseChatCompletion(KindDeepSeek, fixture(t, "deepseek_completion_details.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Usage{InputTokens: 300, CachedInputTokens: 100, OutputTokens: 90, ReasoningTokens: 60}); r.Usage != want {
		t.Fatalf("hit/miss under prompt_tokens_details: %+v", r.Usage)
	}
	if len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "order_status" || r.StopReason != StopToolUse {
		t.Fatalf("tool calls = %+v", r)
	}
	// OpenAI-style cached_tokens only.
	r, _ = parseChatCompletion(KindDeepSeek, []byte(`{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":40}}}`))
	if want := (Usage{InputTokens: 60, CachedInputTokens: 40, OutputTokens: 5}); r.Usage != want {
		t.Fatalf("cached_tokens fallback: %+v", r.Usage)
	}
	if _, err := parseChatCompletion(KindDeepSeek, []byte(`{"choices":[{"message":{"content":""},"finish_reason":"content_filter"}],"usage":{}}`)); !errors.Is(err, ErrRefusal) {
		t.Fatalf("content filter = %v", err)
	}
}

func TestHTTPErrorMapping(t *testing.T) {
	h := http.Header{"Retry-After": []string{"7"}}
	cases := []struct {
		name   string
		kind   Kind
		status int
		header http.Header
		body   string
		want   ErrorKind
		retry  time.Duration
	}{
		{"openai rate limit", KindOpenAI, 429, h, `{"error":{"message":"Rate limit reached","type":"requests","code":"rate_limit_exceeded"}}`, KindRateLimited, 7 * time.Second},
		{"openai quota is not a rate limit", KindOpenAI, 429, h, `{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","code":"insufficient_quota"}}`, KindQuotaExceeded, 0},
		{"openai bad key", KindOpenAI, 401, nil, `{"error":{"message":"Incorrect API key provided","code":"invalid_api_key"}}`, KindAuth, 0},
		{"openai server error", KindOpenAI, 500, nil, `{"error":{"message":"boom"}}`, KindUnavailable, 0},
		{"openai overloaded", KindOpenAI, 503, nil, `{"error":{"message":"overloaded"}}`, KindOverloaded, 0},
		{"anthropic rate limit", KindAnthropic, 429, http.Header{"Retry-After": []string{"30"}}, `{"type":"error","error":{"type":"rate_limit_error","message":"Number of requests has exceeded your rate limit"}}`, KindRateLimited, 30 * time.Second},
		{"anthropic spend limit", KindAnthropic, 429, h, `{"type":"error","error":{"type":"rate_limit_error","message":"spend limit","details":{"error_code":"enforced_spend_limit_reached"}}}`, KindQuotaExceeded, 0},
		{"anthropic overloaded", KindAnthropic, 529, nil, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, KindOverloaded, 0},
		{"anthropic billing", KindAnthropic, 400, nil, `{"type":"error","error":{"type":"billing_error","message":"Your credit balance is too low"}}`, KindQuotaExceeded, 0},
		{"anthropic bad request", KindAnthropic, 400, nil, `{"type":"error","error":{"type":"invalid_request_error","message":"tool_choice forced is not supported"}}`, KindBadRequest, 0},
		{"anthropic auth", KindAnthropic, 401, nil, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`, KindAuth, 0},
		{"gemini exhausted", KindGemini, 429, nil, `{"error":{"code":429,"message":"Resource has been exhausted","status":"RESOURCE_EXHAUSTED"}}`, KindRateLimited, 0},
		{"gemini invalid key", KindGemini, 400, nil, `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}`, KindAuth, 0},
		{"gemini denied", KindGemini, 403, nil, `{"error":{"code":403,"message":"denied","status":"PERMISSION_DENIED"}}`, KindAuth, 0},
		{"deepseek balance", KindDeepSeek, 402, nil, `{"error":{"message":"Insufficient Balance","type":"unknown_error"}}`, KindQuotaExceeded, 0},
		{"deepseek busy", KindDeepSeek, 429, nil, `{"error":{"message":"Rate Limit Reached"}}`, KindRateLimited, 0},
		{"deepseek overloaded", KindDeepSeek, 503, nil, `{"error":{"message":"Server Overloaded"}}`, KindOverloaded, 0},
		{"non-json body", KindOpenAI, 502, nil, `<html>bad gateway</html>`, KindUnavailable, 0},
	}
	for _, c := range cases {
		e := httpError(c.kind, c.status, c.header, []byte(c.body), "sk-secret-key-123456")
		if e.Kind != c.want || e.RetryAfter != c.retry {
			t.Errorf("%s: kind=%s retry=%s, want %s %s", c.name, e.Kind, e.RetryAfter, c.want, c.retry)
		}
		if e.Retryable() != (c.want == KindRateLimited || c.want == KindOverloaded || c.want == KindUnavailable) {
			t.Errorf("%s: retryable=%v", c.name, e.Retryable())
		}
	}
	if !errors.Is(httpError(KindOpenAI, 429, nil, nil, ""), ErrRateLimited) || errors.Is(httpError(KindOpenAI, 401, nil, nil, ""), ErrRateLimited) {
		t.Fatal("errors.Is by kind")
	}
	if d := retryAfter(http.Header{"Retry-After": []string{time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)}}, time.Now()); d < 80*time.Second || d > 91*time.Second {
		t.Fatalf("http-date retry-after = %s", d)
	}
}

func TestKeyRedaction(t *testing.T) {
	key := "sk-proj-AbCdEf0123456789xyz"
	e := httpError(KindOpenAI, 401, nil, []byte(`{"error":{"message":"Incorrect API key provided: `+key+`. You can find your API key at …"}}`), key)
	if strings.Contains(e.Error(), key) || !strings.Contains(e.Error(), "[redacted]") {
		t.Fatalf("key leaked: %s", e.Error())
	}
	// Key-shaped strings are hidden even when they are not the key in hand.
	for _, s := range []string{"sk-ant-api03-abcdefghijklmnop", "AIzaSyD-abcdefghijklmnopqrstuvwx", "sk-1234567890abcdef"} {
		if out := scrub("before "+s+" after", ""); strings.Contains(out, s) {
			t.Errorf("not scrubbed: %s", out)
		}
	}
	if Last4("sk-abcdefgh1234") != "1234" || Last4("short") != "" {
		t.Fatal("last4")
	}
}

func TestValidateJSONAndExtract(t *testing.T) {
	schema := ReplySchema([]DataField{{Key: "email", Label: "E-mail"}})
	good := `{"reply":"Hi","confidence":0.5,"intent":"greeting","escalate":false,"escalate_reason":"","extracted_fields":{"email":null},"actions":[]}`
	if p := ValidateJSON([]byte(good), schema); len(p) != 0 {
		t.Fatalf("valid reply rejected: %v", p)
	}
	bad := `{"reply":3,"confidence":"high","intent":"x","escalate":"no","extracted_fields":{"phone":"1"}}`
	p := ValidateJSON([]byte(bad), schema)
	joined := strings.Join(p, "\n")
	for _, want := range []string{"$.reply: expected string", "$.confidence: expected number", "$.escalate: expected boolean",
		`missing "escalate_reason"`, `$.extracted_fields: unexpected property "phone"`, `$.extracted_fields: missing "email"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing problem %q in %v", want, p)
		}
	}
	bounded := map[string]any{"type": "object", "properties": map[string]any{"n": map[string]any{"type": "integer", "minimum": 0, "maximum": 1},
		"s": map[string]any{"type": "string", "minLength": 2, "maxLength": 3, "enum": []any{"ab", "abc"}}}}
	if p := ValidateJSON([]byte(`{"n":2,"s":"a"}`), bounded); len(p) != 3 {
		t.Fatalf("bounds: %v", p)
	}
	if p := ValidateJSON([]byte(`{"n":0.5,"s":"ab"}`), bounded); len(p) != 1 || !strings.Contains(p[0], "integer") {
		t.Fatalf("integer: %v", p)
	}
	if r, err := DecodeReply([]byte(`{"reply":"x","confidence":1.7}`)); err != nil || r.Confidence != 1 || r.ExtractedFields == nil {
		t.Fatalf("clamp: %+v %v", r, err)
	}
	if p := ValidateJSON([]byte(`{"a":1} x`), schema); len(p) != 1 {
		t.Fatalf("trailing data: %v", p)
	}
	if got := string(extractJSON("```json\n{\"a\":1}\n```")); got != `{"a":1}` {
		t.Fatalf("fence: %s", got)
	}
	if got := string(extractJSON("Sure! {\"a\":1} hope it helps")); got != `{"a":1}` {
		t.Fatalf("prose: %s", got)
	}
}

func TestParseLiteLLM(t *testing.T) {
	doc := `{
	  "sample_spec": {"litellm_provider": "openai", "input_cost_per_token": 0, "output_cost_per_token": 0},
	  "gpt-6.2-sol": {"litellm_provider": "openai", "mode": "responses", "input_cost_per_token": 2e-06, "output_cost_per_token": 1e-05,
	    "cache_read_input_token_cost": 1e-07, "input_cost_per_token_above_272k_tokens": 4e-06, "output_cost_per_token_above_272k_tokens": 1.5e-05},
	  "claude-haiku-5-5": {"litellm_provider": "anthropic", "mode": "chat", "input_cost_per_token": 1e-07, "output_cost_per_token": 5e-07,
	    "cache_creation_input_token_cost": 1.25e-07, "cache_creation_input_token_cost_above_1hr": 2e-07, "cache_read_input_token_cost": 1e-08},
	  "gemini/gemini-4-hydrogen": {"litellm_provider": "gemini", "mode": "chat", "input_cost_per_token": 0.0000015, "output_cost_per_token": 0.0000075},
	  "deepseek/deepseek-flash": {"litellm_provider": "deepseek", "mode": "chat", "input_cost_per_token": 1.5e-07, "output_cost_per_token": 6e-07},
	  "text-embedding-9": {"litellm_provider": "openai", "mode": "embedding", "input_cost_per_token": 1e-08, "output_cost_per_token": 0},
	  "mistral/large": {"litellm_provider": "mistral", "mode": "chat", "input_cost_per_token": 1e-06, "output_cost_per_token": 1e-06},
	  "vertex-only": {"litellm_provider": "vertex_ai-language-models", "input_cost_per_token": 1e-06, "output_cost_per_token": 1e-06},
	  "no-price": {"litellm_provider": "openai", "mode": "chat"}
	}`
	eff := day(2026, 12, 1)
	res, err := ParseLiteLLM(strings.NewReader(doc), eff)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Prices) != 4 || res.Skipped != 5 {
		t.Fatalf("prices=%d skipped=%d", len(res.Prices), res.Skipped)
	}
	by := map[string]PriceInput{}
	for _, p := range res.Prices {
		by[p.Pattern] = p
		if p.Source != "litellm" || !p.EffectiveFrom.Equal(eff) || p.SourceURL != LiteLLMSource {
			t.Errorf("%s: source/date %+v", p.Pattern, p)
		}
	}
	gpt := by["gpt-6.2-sol"]
	if gpt.Kind != KindOpenAI || gpt.Input != "2" || gpt.Output != "10" || deref(gpt.CachedInput) != "0.1" ||
		len(gpt.Rules.Thresholds) != 1 || gpt.Rules.Thresholds[0].InputTokensOver != 272000 ||
		deref(gpt.Rules.Thresholds[0].Input) != "4" || deref(gpt.Rules.Thresholds[0].Output) != "15" {
		t.Fatalf("gpt = %+v %+v", gpt, gpt.Rules)
	}
	haiku := by["claude-haiku-5-5"]
	if haiku.Input != "0.1" || deref(haiku.CacheWrite5m) != "0.125" || deref(haiku.CacheWrite1h) != "0.2" || deref(haiku.CachedInput) != "0.01" {
		t.Fatalf("haiku = %+v", haiku)
	}
	if g := by["gemini-4-hydrogen"]; g.Kind != KindGemini || g.Input != "1.5" || g.Output != "7.5" {
		t.Fatalf("gemini prefix not stripped: %+v", g)
	}
	if d := by["deepseek-flash"]; d.Kind != KindDeepSeek || d.Input != "0.15" {
		t.Fatalf("deepseek = %+v", d)
	}
	if _, err := ParseLiteLLM(strings.NewReader(`[1,2]`), eff); err == nil {
		t.Fatal("non-object accepted")
	}
	if _, err := ParseLiteLLM(strings.NewReader(`{"x":{"litellm_provider":"mistral"}}`), eff); err == nil {
		t.Fatal("file without supported providers accepted")
	}
}

func TestSeedPricesAreOnlyTheResearchedOnes(t *testing.T) {
	seed, err := SeedPrices()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"claude-opus-5-5": {"4", "20"}, "claude-sonnet-5-5": {"2", "10"}, "claude-haiku-5-5": {"0.1", "0.5"},
		"gpt-6.2-sol": {"2", "10"}, "gpt-6-luna": {"0.1", "0.5"}, "gemini-4-hydrogen": {"1.5", "7.5"}, "deepseek-flash": {"0.15", "0.6"},
	}
	if len(seed) != len(want) {
		t.Fatalf("seed has %d rows, want %d", len(seed), len(want))
	}
	for _, s := range seed {
		w, ok := want[s.Pattern]
		if !ok || s.Input != w[0] || s.Output != w[1] || !s.EffectiveFrom.Equal(SeedDate) || s.Source != "seed" || !strings.HasPrefix(s.SourceURL, "https://") {
			t.Errorf("unexpected seed row %+v", s)
		}
	}
}

func TestProfileNormalize(t *testing.T) {
	low := 0.6
	p := Profile{Name: " Destek ", DisclosureMode: DiscloseOff, Status: ProfileActive,
		Handoff:        Handoff{OnRequest: false, OnLowConfidence: &low, OnCriticalIntents: []string{"Complaint", "complaint", " refund "}},
		DataCollection: DataCollection{Enabled: true, Fields: []DataField{{Key: "email"}, {Key: "attr:city", Label: "Şehir", Required: true}}},
		AllowedActions: []string{"add_tag", "add_tag", "automation:0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"}}
	if err := p.normalize(); err != nil {
		t.Fatal(err)
	}
	if !p.Handoff.OnRequest {
		t.Fatal("handoff on request must be forced on")
	}
	if strings.Join(p.Handoff.OnCriticalIntents, ",") != "complaint,refund" || p.Name != "Destek" || len(p.AllowedActions) != 2 ||
		p.DataCollection.Fields[0].Label != "email" {
		t.Fatalf("normalised = %+v", p)
	}
	if w := p.Warnings(); strings.Join(w, ",") != "disclosure_off,no_number" {
		t.Fatalf("warnings = %v", w)
	}
	bad := []Profile{
		{Name: "", DisclosureMode: DiscloseOff, Status: ProfileActive},
		{Name: "x", DisclosureMode: "sometimes", Status: ProfileActive},
		{Name: "x", DisclosureMode: DiscloseOff, Status: "on"},
		{Name: "x", DisclosureMode: DiscloseOff, Status: ProfileActive, Handoff: Handoff{OnLowConfidence: ptr(1.5)}},
		{Name: "x", DisclosureMode: DiscloseOff, Status: ProfileActive, DataCollection: DataCollection{Fields: []DataField{{Key: "phone_e164"}}}},
		{Name: "x", DisclosureMode: DiscloseOff, Status: ProfileActive, AllowedActions: []string{"delete_everything"}},
		{Name: "x", DisclosureMode: DiscloseOff, Status: ProfileActive, AllowedActions: []string{"automation:nope"}},
		{Name: "x", DisclosureMode: DiscloseOff, Status: ProfileActive, SystemPrompt: strings.Repeat("a", MaxSystemPrompt+1)},
	}
	for i, b := range bad {
		if err := b.normalize(); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestInstructionsCarryPlatformRules(t *testing.T) {
	p := Profile{SystemPrompt: "Sen Acme'nin asistanısın.", DisclosureMode: DiscloseFirstMessage, Handoff: Handoff{OnNoAnswer: true, OnCriticalIntents: []string{"complaint"}},
		DataCollection: DataCollection{Enabled: true, Fields: []DataField{{Key: "email", Label: "E-posta", Required: true}}}}
	s := DefaultSettings()
	got := Instructions(p, s, LangTR)
	for _, want := range []string{"Sen Acme'nin asistanısın.", "human_request", "complaint", s.Disclosure.TR, "E-posta (required, key email)", "never reveal"} {
		if !strings.Contains(got, want) {
			t.Errorf("instructions miss %q:\n%s", want, got)
		}
	}
	p.DisclosureMode = DiscloseOff
	if strings.Contains(Instructions(p, s, LangEN), s.Disclosure.EN) {
		t.Fatal("disclosure off still discloses")
	}
}
