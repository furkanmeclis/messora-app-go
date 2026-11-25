// Package aitest emulates the OpenAI, Anthropic, Gemini and DeepSeek HTTP
// APIs with their documented JSON shapes, for tests (no real network).
// Each provider lives under its own prefix; BaseURLs gives the endpoints to
// configure.
package aitest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// DefaultReply is the structured answer every emulated provider gives
// unless a reply is queued.
const DefaultReply = `{"reply":"Merhaba, size nasıl yardımcı olabilirim?","confidence":0.9,"intent":"greeting","escalate":false,"escalate_reason":"","extracted_fields":{},"actions":[]}`

// Request is a recorded call.
type Request struct {
	Provider string // openai | anthropic | gemini | deepseek
	Method   string
	Path     string // without the provider prefix
	Header   http.Header
	Body     []byte
}

// Reply is a queued answer.
type Reply struct {
	Status int
	Header http.Header
	Body   string
}

// Server is the emulator.
type Server struct {
	*httptest.Server
	Key string // the only accepted API key

	mu       sync.Mutex
	requests []Request
	queue    map[string][]Reply // provider → replies for the next generation calls
	// Balance is DeepSeek's is_available.
	Balance bool
}

// New starts the emulator accepting key.
func New(key string) *Server {
	s := &Server{Key: key, queue: map[string][]Reply{}, Balance: true}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// BaseURLs are the endpoints per provider kind.
func (s *Server) BaseURLs() map[string]string {
	return map[string]string{"openai": s.URL + "/openai/v1", "anthropic": s.URL + "/anthropic", "gemini": s.URL + "/gemini", "deepseek": s.URL + "/deepseek"}
}

// Enqueue makes the next generation call of provider answer with r.
func (s *Server) Enqueue(provider string, r Reply) {
	s.mu.Lock()
	s.queue[provider] = append(s.queue[provider], r)
	s.mu.Unlock()
}

// Requests returns the recorded calls.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Last returns the last recorded call of a provider to a path suffix.
func (s *Server) Last(provider, pathSuffix string) (Request, bool) {
	rs := s.Requests()
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i].Provider == provider && strings.HasSuffix(rs[i].Path, pathSuffix) {
			return rs[i], true
		}
	}
	return Request{}, false
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	provider, path := parts[0], ""
	if len(parts) == 2 {
		path = "/" + parts[1]
	}
	if provider == "openai" {
		path = strings.TrimPrefix(path, "/v1") // the base URL ends in /v1
	}
	s.mu.Lock()
	s.requests = append(s.requests, Request{Provider: provider, Method: r.Method, Path: path, Header: r.Header.Clone(), Body: body})
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if !s.authorised(provider, r) {
		s.unauthorised(w, provider)
		return
	}
	switch {
	case r.Method == http.MethodGet && path == "/models" && provider == "openai":
		write(w, 200, `{"object":"list","data":[{"id":"gpt-6.2-sol","object":"model","created":1760000000,"owned_by":"openai"},
			{"id":"gpt-5-old","object":"model","created":1700000000,"owned_by":"openai","shutdown_date":"2027-01-01"}]}`)
	case r.Method == http.MethodGet && path == "/v1/models" && provider == "anthropic":
		if r.URL.Query().Get("after_id") == "" {
			write(w, 200, `{"data":[{"type":"model","id":"claude-opus-5-5","display_name":"Claude Opus 5.5","created_at":"2026-09-01T00:00:00Z",
				"max_input_tokens":1000000,"max_tokens":128000,"lifecycle":"active","capabilities":{"image_input":{"supported":true},
				"pdf_input":{"supported":true},"structured_outputs":{"supported":true},"thinking":{"supported":true,"types":{}}}}],
				"has_more":true,"first_id":"claude-opus-5-5","last_id":"claude-opus-5-5"}`)
			return
		}
		write(w, 200, `{"data":[{"type":"model","id":"claude-haiku-5-5","display_name":"Claude Haiku 5.5","created_at":"2026-09-01T00:00:00Z",
			"max_input_tokens":200000,"max_tokens":64000,"capabilities":{"image_input":{"supported":true},"structured_outputs":{"supported":true},
			"thinking":{"supported":false}}}],"has_more":false,"first_id":"claude-haiku-5-5","last_id":"claude-haiku-5-5"}`)
	case r.Method == http.MethodGet && path == "/v1beta/models" && provider == "gemini":
		write(w, 200, `{"models":[{"name":"models/gemini-4-hydrogen","displayName":"Gemini 4 Hydrogen","inputTokenLimit":1048576,
			"outputTokenLimit":65536,"supportedGenerationMethods":["generateContent","countTokens"],"thinking":true},
			{"name":"models/text-embedding-9","displayName":"Embedding","inputTokenLimit":2048,"outputTokenLimit":1,
			"supportedGenerationMethods":["embedContent"]}]}`)
	case r.Method == http.MethodGet && path == "/models" && provider == "deepseek":
		write(w, 200, `{"object":"list","data":[{"id":"deepseek-flash","object":"model","owned_by":"deepseek","context_window":128000,
			"max_output_tokens":8192,"input_modalities":["text"]},{"id":"deepseek-v4-pro","object":"model","owned_by":"deepseek",
			"context_window":256000,"max_output_tokens":32768}]}`)
	case r.Method == http.MethodGet && path == "/user/balance" && provider == "deepseek":
		write(w, 200, fmt.Sprintf(`{"is_available":%v,"balance_infos":[{"currency":"USD","total_balance":"10.00","granted_balance":"0.00","topped_up_balance":"10.00"}]}`, s.Balance))
	case r.Method == http.MethodPost:
		s.generate(w, provider, path, body)
	default:
		write(w, 404, `{"error":{"message":"not found"}}`)
	}
}

func (s *Server) authorised(provider string, r *http.Request) bool {
	switch provider {
	case "anthropic":
		// The raw model list sends x-api-key; anthropic-sdk-go v2 sends the
		// key as a bearer token.
		return r.Header.Get("X-Api-Key") == s.Key || r.Header.Get("Authorization") == "Bearer "+s.Key
	case "gemini":
		return r.Header.Get("X-Goog-Api-Key") == s.Key
	}
	return r.Header.Get("Authorization") == "Bearer "+s.Key
}

func (s *Server) unauthorised(w http.ResponseWriter, provider string) {
	switch provider {
	case "anthropic":
		write(w, 401, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	case "gemini":
		write(w, 400, `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}`)
	default:
		write(w, 401, `{"error":{"message":"Incorrect API key provided.","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}
}

func (s *Server) generate(w http.ResponseWriter, provider, path string, body []byte) {
	s.mu.Lock()
	var q *Reply
	if len(s.queue[provider]) > 0 {
		q = &s.queue[provider][0]
		s.queue[provider] = s.queue[provider][1:]
	}
	s.mu.Unlock()
	if q != nil {
		for k, v := range q.Header {
			w.Header()[k] = v
		}
		write(w, q.Status, q.Body)
		return
	}
	text, _ := json.Marshal(DefaultReply)
	var req struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &req)
	switch {
	case provider == "openai" && path == "/responses":
		write(w, 200, fmt.Sprintf(`{"id":"resp_1","object":"response","status":"completed","model":%q,"output":[{"type":"message","id":"m1",
			"status":"completed","role":"assistant","content":[{"type":"output_text","text":%s,"annotations":[]}]}],
			"usage":{"input_tokens":1200,"input_tokens_details":{"cached_tokens":200},"output_tokens":100,"output_tokens_details":{"reasoning_tokens":20},"total_tokens":1300}}`,
			req.Model, text))
	case provider == "anthropic" && path == "/v1/messages":
		write(w, 200, fmt.Sprintf(`{"id":"msg_1","type":"message","role":"assistant","model":%q,"content":[{"type":"text","text":%s}],
			"stop_reason":"end_turn","usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100}}`,
			req.Model, text))
	case provider == "gemini" && strings.HasSuffix(path, ":generateContent"):
		model := strings.TrimSuffix(strings.TrimPrefix(path, "/v1beta/models/"), ":generateContent")
		write(w, 200, fmt.Sprintf(`{"candidates":[{"content":{"role":"model","parts":[{"text":%s}]},"finishReason":"STOP","index":0}],
			"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":100,"totalTokenCount":1100},"modelVersion":%q}`, text, model))
	case provider == "deepseek" && path == "/chat/completions":
		write(w, 200, fmt.Sprintf(`{"id":"c1","object":"chat.completion","model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":%s},
			"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"prompt_cache_hit_tokens":400,"prompt_cache_miss_tokens":600}}`,
			req.Model, text))
	default:
		write(w, 404, `{"error":{"message":"unknown endpoint `+path+`"}}`)
	}
}

func write(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body) //nolint:gosec // test emulator answering with fixed JSON
}
