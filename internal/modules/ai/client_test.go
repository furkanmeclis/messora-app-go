package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai/aitest"
)

const testKey = "sk-test-0123456789abcdef"

func emulator(t *testing.T) *aitest.Server {
	t.Helper()
	s := aitest.New(testKey)
	t.Cleanup(s.Close)
	return s
}

func cfgFor(s *aitest.Server, kind Kind) ProviderConfig {
	return ProviderConfig{Kind: kind, APIKey: testKey, BaseURL: s.BaseURLs()[string(kind)], HTTP: s.Client()}
}

func sampleRequest(model string) Request {
	return Request{Model: model, System: "You are Acme's assistant.",
		Messages: []Message{
			{Role: RoleUser, Text: "Siparişim nerede?"},
			{Role: RoleOperatorNote, Text: "VIP customer, offer free shipping"},
			{Role: RoleAssistant, Text: "Sipariş numaranız nedir?"},
			{Role: RoleUser, Text: "A-17"},
		},
		Tools:  []Tool{{Name: "order_status", Description: "Look up an order", Parameters: map[string]any{"type": "object", "properties": map[string]any{"order_id": map[string]any{"type": "string"}}, "required": []any{"order_id"}}}},
		Output: &OutputSchema{Name: ReplySchemaName, Schema: ReplySchema(nil)}, MaxTokens: 500}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("request body is not JSON: %s", b)
	}
	return m
}

// userTexts collects every user-authored string of a request body.
func userTexts(v any, inUser bool, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		u := inUser || x["role"] == "user"
		for _, e := range x {
			userTexts(e, u, out)
		}
	case []any:
		for _, e := range x {
			userTexts(e, inUser, out)
		}
	case string:
		if inUser {
			*out = append(*out, x)
		}
	}
}

func assertNoteNotUserContent(t *testing.T, body map[string]any) {
	t.Helper()
	var texts []string
	userTexts(body["messages"], false, &texts)
	userTexts(body["input"], false, &texts)
	userTexts(body["contents"], false, &texts)
	for _, s := range texts {
		if strings.Contains(s, "VIP customer") {
			t.Fatalf("operator note sent as user content: %q", s)
		}
	}
	raw, _ := json.Marshal(body)
	if !strings.Contains(string(raw), "VIP customer") {
		t.Fatal("operator note missing from the request")
	}
}

func TestOpenAIClientRequestAndUsage(t *testing.T) {
	s := emulator(t)
	c, _ := NewClient(cfgFor(s, KindOpenAI))
	temp := 0.2
	req := sampleRequest("gpt-6.2-sol")
	req.Temperature = &temp
	r, err := c.Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Usage{InputTokens: 1000, CachedInputTokens: 200, OutputTokens: 100, ReasoningTokens: 20}); r.Usage != want {
		t.Fatalf("usage = %+v", r.Usage)
	}
	if string(r.Structured) != aitest.DefaultReply {
		t.Fatalf("structured = %s", r.Structured)
	}
	got, _ := s.Last("openai", "/responses")
	body := decode(t, got.Body)
	if body["store"] != false {
		t.Fatalf("store must be false (no retention): %s", got.Body)
	}
	format := body["text"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["strict"] != true || format["name"] != ReplySchemaName {
		t.Fatalf("text.format = %v", format)
	}
	if body["instructions"] != "You are Acme's assistant." || body["max_output_tokens"] != float64(500) || body["temperature"] != 0.2 {
		t.Fatalf("params: %s", got.Body)
	}
	tools := body["tools"].([]any)
	if tool := tools[0].(map[string]any); tool["type"] != "function" || tool["name"] != "order_status" {
		t.Fatalf("tools = %v", tools)
	}
	// The note is a developer message.
	if !strings.Contains(string(got.Body), `"role":"developer"`) {
		t.Fatalf("operator note is not a developer message: %s", got.Body)
	}
	assertNoteNotUserContent(t, body)
}

func TestAnthropicClientRequestAndUsage(t *testing.T) {
	s := emulator(t)
	c, _ := NewClient(cfgFor(s, KindAnthropic))
	r, err := c.Generate(context.Background(), sampleRequest("claude-opus-5-5"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Usage.InputTokens != 1000 || r.Usage.OutputTokens != 100 || string(r.Structured) != aitest.DefaultReply {
		t.Fatalf("response = %+v", r)
	}
	got, _ := s.Last("anthropic", "/v1/messages")
	body := decode(t, got.Body)
	if f := body["output_config"].(map[string]any)["format"].(map[string]any); f["type"] != "json_schema" || f["schema"] == nil {
		t.Fatalf("output_config = %v", body["output_config"])
	}
	if tc := body["tool_choice"].(map[string]any); tc["type"] != "auto" {
		t.Fatalf("tool_choice must be auto: %v", tc)
	}
	tool := body["tools"].([]any)[0].(map[string]any)
	if tool["name"] != "order_status" || tool["input_schema"].(map[string]any)["required"] == nil {
		t.Fatalf("tool = %v", tool)
	}
	if _, has := body["temperature"]; has {
		t.Fatal("temperature must not be sent to Claude")
	}
	if got.Header.Get("Authorization") != "Bearer "+testKey || got.Header.Get("Anthropic-Version") == "" {
		t.Fatalf("headers = %v", got.Header)
	}
	sys, _ := json.Marshal(body["system"])
	if !strings.Contains(string(sys), "VIP customer") || !strings.Contains(string(sys), OperatorNotesHeader) {
		t.Fatalf("notes must be in the system prompt: %s", sys)
	}
	assertNoteNotUserContent(t, body)
}

func TestGeminiClientRequestAndUsage(t *testing.T) {
	s := emulator(t)
	c, _ := NewClient(cfgFor(s, KindGemini))
	r, err := c.Generate(context.Background(), sampleRequest("gemini-4-hydrogen"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Usage.InputTokens != 1000 || r.Usage.OutputTokens != 100 || string(r.Structured) != aitest.DefaultReply || r.Model != "gemini-4-hydrogen" {
		t.Fatalf("response = %+v", r)
	}
	got, ok := s.Last("gemini", ":generateContent")
	if !ok || got.Header.Get("X-Goog-Api-Key") != testKey || strings.Contains(got.Path, testKey) {
		t.Fatalf("request = %+v", got)
	}
	body := decode(t, got.Body)
	gc := body["generationConfig"].(map[string]any)
	if gc["responseMimeType"] != "application/json" || gc["responseJsonSchema"] == nil || gc["maxOutputTokens"] != float64(500) {
		t.Fatalf("generationConfig = %v", gc)
	}
	if decls := body["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any); len(decls) != 1 {
		t.Fatalf("tools = %v", body["tools"])
	}
	contents := body["contents"].([]any)
	if len(contents) != 3 || contents[1].(map[string]any)["role"] != "model" {
		t.Fatalf("contents = %v", contents)
	}
	assertNoteNotUserContent(t, body)
}

func TestDeepSeekClientRepairsOnce(t *testing.T) {
	s := emulator(t)
	c, _ := NewClient(cfgFor(s, KindDeepSeek))
	bad := `{"id":"c0","model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"{\"reply\":\"x\"}"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":500,"completion_tokens":10,"prompt_cache_hit_tokens":100,"prompt_cache_miss_tokens":400}}`
	s.Enqueue("deepseek", aitest.Reply{Status: 200, Body: bad})
	r, err := c.Generate(context.Background(), sampleRequest("deepseek-flash"))
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Structured) != aitest.DefaultReply {
		t.Fatalf("structured = %s", r.Structured)
	}
	// Both calls are billed: 400+600 miss, 100+400 hit, 10+100 output.
	if want := (Usage{InputTokens: 1000, CachedInputTokens: 500, OutputTokens: 110}); r.Usage != want {
		t.Fatalf("usage = %+v", r.Usage)
	}
	var calls []aitest.Request
	for _, q := range s.Requests() {
		if q.Path == "/chat/completions" {
			calls = append(calls, q)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d", len(calls))
	}
	first := decode(t, calls[0].Body)
	if rf := first["response_format"].(map[string]any); rf["type"] != "json_object" {
		t.Fatalf("response_format = %v", rf)
	}
	if !strings.Contains(string(calls[0].Body), "JSON Schema") {
		t.Fatal("the schema must be in the instructions")
	}
	if !strings.Contains(string(calls[1].Body), "did not match the required JSON Schema") {
		t.Fatalf("repair prompt missing: %s", calls[1].Body)
	}
	assertNoteNotUserContent(t, first)

	// Still invalid after the repair → invalid_output, usage of both calls kept.
	s.Enqueue("deepseek", aitest.Reply{Status: 200, Body: bad})
	s.Enqueue("deepseek", aitest.Reply{Status: 200, Body: bad})
	r, err = c.Generate(context.Background(), sampleRequest("deepseek-flash"))
	if !errors.Is(err, ErrInvalidOutput) || r.Usage.OutputTokens != 20 {
		t.Fatalf("err = %v usage = %+v", err, r.Usage)
	}
}

func TestClientErrorsAreTypedAndNotRetriedBySDK(t *testing.T) {
	s := emulator(t)
	cases := []struct {
		kind  Kind
		reply aitest.Reply
		want  *Error
		after time.Duration
	}{
		{KindOpenAI, aitest.Reply{Status: 429, Header: http.Header{"Retry-After": {"3"}}, Body: `{"error":{"message":"slow down","type":"requests","code":"rate_limit_exceeded"}}`}, ErrRateLimited, 3 * time.Second},
		{KindOpenAI, aitest.Reply{Status: 429, Body: `{"error":{"message":"quota","type":"insufficient_quota","code":"insufficient_quota"}}`}, ErrQuotaExceeded, 0},
		{KindAnthropic, aitest.Reply{Status: 529, Body: `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`}, ErrOverloaded, 0},
		{KindAnthropic, aitest.Reply{Status: 429, Body: `{"type":"error","error":{"type":"rate_limit_error","message":"limit","details":{"error_code":"enforced_spend_limit_reached"}}}`}, ErrQuotaExceeded, 0},
		{KindGemini, aitest.Reply{Status: 429, Body: `{"error":{"code":429,"message":"Resource has been exhausted","status":"RESOURCE_EXHAUSTED"}}`}, ErrRateLimited, 0},
		{KindDeepSeek, aitest.Reply{Status: 402, Body: `{"error":{"message":"Insufficient Balance","type":"unknown_error"}}`}, ErrQuotaExceeded, 0},
		{KindDeepSeek, aitest.Reply{Status: 400, Body: `{"error":{"message":"bad","type":"invalid_request_error"}}`}, ErrBadRequest, 0},
	}
	for _, c := range cases {
		before := len(s.Requests())
		s.Enqueue(string(c.kind), c.reply)
		cl, _ := NewClient(cfgFor(s, c.kind))
		_, err := cl.Generate(context.Background(), Request{Model: "m", Messages: []Message{{Role: RoleUser, Text: "hi"}}})
		if !errors.Is(err, c.want) || RetryAfterOf(err) != c.after {
			t.Errorf("%s %d: err = %v (retry %s), want %s", c.kind, c.reply.Status, err, RetryAfterOf(err), c.want.Kind)
		}
		if n := len(s.Requests()) - before; n != 1 {
			t.Errorf("%s: the SDK retried (%d calls); retries belong to the caller", c.kind, n)
		}
	}
	// A wrong key: auth error, never echoing the key.
	for _, k := range Kinds {
		cfg := cfgFor(s, k)
		cfg.APIKey = "sk-wrong-key-abcdefghijkl"
		cl, _ := NewClient(cfg)
		_, err := cl.Generate(context.Background(), Request{Model: "m", Messages: []Message{{Role: RoleUser, Text: "hi"}}})
		if !errors.Is(err, ErrAuth) || strings.Contains(err.Error(), cfg.APIKey) {
			t.Errorf("%s wrong key: %v", k, err)
		}
	}
}

func TestListModelsAndCheckKey(t *testing.T) {
	s := emulator(t)
	ms, err := ListModels(context.Background(), cfgFor(s, KindAnthropic))
	if err != nil || len(ms) != 2 {
		t.Fatalf("anthropic paging: %v %+v", err, ms)
	}
	opus := ms[0]
	if opus.ID != "claude-opus-5-5" || *opus.ContextWindow != 1000000 || *opus.MaxOutput != 128000 || !*opus.Capabilities.Tools ||
		!*opus.Capabilities.Vision || !*opus.Capabilities.StructuredOutput || !*opus.Capabilities.Thinking || *opus.Lifecycle != "active" {
		t.Fatalf("opus = %+v", opus)
	}
	if ms[1].Capabilities.Thinking == nil || *ms[1].Capabilities.Thinking || ms[1].Lifecycle != nil {
		t.Fatalf("haiku = %+v", ms[1])
	}
	ms, _ = ListModels(context.Background(), cfgFor(s, KindOpenAI))
	if len(ms) != 2 || ms[0].ContextWindow != nil || ms[0].Capabilities.Tools != nil || *ms[1].Lifecycle != "deprecated" {
		t.Fatalf("openai reports no limits or capabilities: %+v", ms)
	}
	ms, _ = ListModels(context.Background(), cfgFor(s, KindGemini))
	if len(ms) != 1 || ms[0].ID != "gemini-4-hydrogen" || *ms[0].ContextWindow != 1048576 || !*ms[0].Capabilities.Thinking || ms[0].Capabilities.Vision != nil {
		t.Fatalf("gemini = %+v", ms)
	}
	ms, _ = ListModels(context.Background(), cfgFor(s, KindDeepSeek))
	if len(ms) != 2 || *ms[0].MaxOutput != 8192 || *ms[0].Capabilities.Vision || ms[1].Capabilities.Vision != nil {
		t.Fatalf("deepseek = %+v", ms)
	}

	for _, k := range Kinds {
		if st, _ := CheckKey(context.Background(), cfgFor(s, k)); st != StatusOK {
			t.Errorf("%s: status %s", k, st)
		}
		bad := cfgFor(s, k)
		bad.APIKey = "sk-wrong-key-abcdefghijkl"
		if st, detail := CheckKey(context.Background(), bad); st != StatusInvalid || strings.Contains(detail, bad.APIKey) {
			t.Errorf("%s wrong key: %s %q", k, st, detail)
		}
	}
	s.Balance = false
	if st, detail := CheckKey(context.Background(), cfgFor(s, KindDeepSeek)); st != StatusNoBalance || !strings.Contains(detail, "10.00 USD") {
		t.Fatalf("deepseek balance: %s %q", st, detail)
	}
	if req, _ := s.Last("deepseek", "/user/balance"); req.Header.Get("Authorization") != "Bearer "+testKey {
		t.Fatal("balance call not authenticated")
	}
}

func TestValidateBaseURL(t *testing.T) {
	if u, err := ValidateBaseURL(KindDeepSeek, " https://api.deepseek.com/ "); err != nil || u != "https://api.deepseek.com" {
		t.Fatalf("%q %v", u, err)
	}
	for _, bad := range []string{"http://api.deepseek.com", "https://user:pw@x.com", "https://x.com/?a=1", "ftp://x", "https://"} {
		if _, err := ValidateBaseURL(KindDeepSeek, bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if _, err := ValidateBaseURL(KindAnthropic, "https://proxy.example.com"); err == nil {
		t.Fatal("anthropic accepts no override")
	}
}

func TestFakeClient(t *testing.T) {
	c := &FakeClient{}
	r, err := c.Generate(context.Background(), Request{Model: "fake-small", Messages: []Message{{Role: RoleUser, Text: "Merhaba"}},
		Output: &OutputSchema{Schema: ReplySchema([]DataField{{Key: "email"}})}})
	if err != nil {
		t.Fatal(err)
	}
	if p := ValidateJSON(r.Structured, ReplySchema([]DataField{{Key: "email"}})); len(p) != 0 {
		t.Fatalf("fake output does not match the schema: %v %s", p, r.Structured)
	}
	var rep Reply
	_ = json.Unmarshal(r.Structured, &rep)
	if rep.Reply != "Fake reply to: Merhaba" || rep.Confidence != 0.9 || r.Usage.InputTokens == 0 {
		t.Fatalf("reply = %+v", rep)
	}
	_, err = c.Generate(context.Background(), Request{Messages: []Message{{Role: RoleUser, Text: "fake:error:rate_limited"}}})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("scripted error = %v", err)
	}
	scripted := &FakeClient{Script: func(_ context.Context, req Request) (Response, error) {
		return Response{Text: "scripted " + req.Model}, nil
	}}
	if r, _ := scripted.Generate(context.Background(), Request{Model: "m"}); r.Text != "scripted m" || r.Model != "m" {
		t.Fatalf("script = %+v", r)
	}
}
