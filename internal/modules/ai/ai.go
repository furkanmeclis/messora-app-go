// Package ai connects Messora to large language model providers (OpenAI,
// Anthropic, Google Gemini, DeepSeek) behind one interface, keeps their
// model catalogue and prices, records the usage and cost of every request,
// and stores the AI profiles that decide how the AI behaves on a number.
//
// Every AI request of the application goes through Service.Generate: it
// enforces the company switch (ai.enabled), resolves the provider and model,
// asks the provider not to retain or train on the data where it offers such
// an option, maps provider errors to typed errors and writes exactly one
// ai_usage row. Provider API keys are sealed at rest, decrypted only for a
// call and never returned, logged or audited.
//
// All AI configuration is read through Service, so scoping it per tenant
// (Phase 17) changes one place.
package ai

import (
	"context"
	"encoding/json"
	"time"
)

// Kind is a provider type.
type Kind string

// Provider kinds. KindFake is a scriptable provider for tests and
// development (enabled with AI_FAKE_PROVIDER outside production).
const (
	KindOpenAI    Kind = "openai"
	KindAnthropic Kind = "anthropic"
	KindGemini    Kind = "gemini"
	KindDeepSeek  Kind = "deepseek"
	KindFake      Kind = "fake"
)

// Kinds lists the real provider kinds.
var Kinds = []Kind{KindOpenAI, KindAnthropic, KindGemini, KindDeepSeek}

// Role is the author of a message in a request.
type Role string

// Roles. RoleOperatorNote is an internal note written by a team member: it
// is context for the model and is sent as a system/developer instruction,
// never as user content and never to the customer.
const (
	RoleUser         Role = "user"
	RoleAssistant    Role = "assistant"
	RoleOperatorNote Role = "operator_note"
)

// Message is one turn of the conversation given to the model.
type Message struct {
	Role Role   `json:"role"`
	Text string `json:"text"`
}

// Tool is a function the model may call. Parameters is a JSON Schema
// object. Tool choice is always "auto": some models (e.g. Claude Opus 5.5)
// reject forced tool choice.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// OutputSchema asks for structured output matching Schema (JSON Schema).
type OutputSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
}

// Request is a provider-neutral generation request.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Tools       []Tool
	Output      *OutputSchema // nil = free text
	MaxTokens   int64         // 0 = DefaultMaxTokens
	Temperature *float64      // nil = provider default; ignored where the model has none
}

// DefaultMaxTokens caps the output when a request does not.
const DefaultMaxTokens = 1024

// ToolCall is a function call requested by the model.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Usage counts tokens the way they are billed:
//   - InputTokens: input tokens billed at the normal input price (cache reads
//     and cache writes excluded).
//   - CachedInputTokens: input read from the provider's prompt cache.
//   - CacheWriteTokens: input written to the cache (Anthropic: 5-minute
//     cache; OpenAI: cache_write_tokens).
//   - CacheWrite1hTokens: input written to Anthropic's 1-hour cache.
//   - OutputTokens: every output token, reasoning included.
//   - ReasoningTokens: the reasoning share of OutputTokens (informational).
type Usage struct {
	InputTokens        int64 `json:"input_tokens"`
	CachedInputTokens  int64 `json:"cached_input_tokens"`
	CacheWriteTokens   int64 `json:"cache_write_tokens"`
	CacheWrite1hTokens int64 `json:"cache_write_1h_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	ReasoningTokens    int64 `json:"reasoning_tokens"`
}

// TotalInput is every input token, cached or not (what input-size price
// thresholds compare against).
func (u Usage) TotalInput() int64 {
	return u.InputTokens + u.CachedInputTokens + u.CacheWriteTokens + u.CacheWrite1hTokens
}

// Add returns the sum of two usages.
func (u Usage) Add(o Usage) Usage {
	return Usage{InputTokens: u.InputTokens + o.InputTokens, CachedInputTokens: u.CachedInputTokens + o.CachedInputTokens,
		CacheWriteTokens: u.CacheWriteTokens + o.CacheWriteTokens, CacheWrite1hTokens: u.CacheWrite1hTokens + o.CacheWrite1hTokens,
		OutputTokens: u.OutputTokens + o.OutputTokens, ReasoningTokens: u.ReasoningTokens + o.ReasoningTokens}
}

// Stop reasons, normalised across providers.
const (
	StopEnd       = "end"
	StopMaxTokens = "max_tokens"
	StopToolUse   = "tool_use"
	StopRefusal   = "refusal"
	StopOther     = "other"
)

// Response is a provider-neutral generation result.
type Response struct {
	Text       string          `json:"text"`
	ToolCalls  []ToolCall      `json:"tool_calls"`
	Structured json.RawMessage `json:"structured,omitempty"` // set when Request.Output was given
	Usage      Usage           `json:"usage"`
	Model      string          `json:"model"`
	Latency    time.Duration   `json:"latency"`
	StopReason string          `json:"stop_reason"`
}

// Client generates text with one provider connection.
type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

// ModelInfo is one entry of a provider's model list. Nil fields are not
// reported by the provider.
type ModelInfo struct {
	ID            string
	DisplayName   string
	ContextWindow *int64
	MaxOutput     *int64
	Capabilities  Capabilities
	Lifecycle     *string
}

// Capabilities of a model; nil = unknown (the provider does not say).
type Capabilities struct {
	Tools            *bool `json:"tools"`
	StructuredOutput *bool `json:"structured_output"`
	Vision           *bool `json:"vision"`
	Thinking         *bool `json:"thinking"`
}

// KeyStatus is the outcome of a key check.
type KeyStatus string

// Key check outcomes (ai_providers.status).
const (
	StatusUnknown   KeyStatus = "unknown"
	StatusOK        KeyStatus = "ok"
	StatusInvalid   KeyStatus = "invalid"
	StatusNoBalance KeyStatus = "no_balance"
	StatusError     KeyStatus = "error"
)

// Purposes of a request (ai_usage.purpose).
const (
	PurposeReply      = "reply"
	PurposeSuggest    = "suggest"
	PurposeClassify   = "classify"
	PurposeExtract    = "extract"
	PurposeSummarize  = "summarize"
	PurposeAutomation = "automation"
	PurposeTest       = "test"
)

func ptr[T any](v T) *T { return &v }
