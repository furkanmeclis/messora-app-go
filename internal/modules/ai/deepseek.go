package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// deepseekClient uses DeepSeek's OpenAI-compatible Chat Completions API
// through openai-go. response_format only supports json_object, so the
// schema is given in the instructions, the answer is validated here and one
// repair attempt is made. DeepSeek offers no per-request retention or
// training flag (see docs/AI.md).
type deepseekClient struct {
	cfg ProviderConfig
	sdk openai.Client
}

func newDeepSeek(cfg ProviderConfig) *deepseekClient {
	return &deepseekClient{cfg: cfg, sdk: openai.NewClient(openAIOptions(cfg)...)}
}

func deepseekParams(req Request) openai.ChatCompletionNewParams {
	p := openai.ChatCompletionNewParams{Model: req.Model, MaxTokens: openai.Int(maxTokens(req))}
	if req.Temperature != nil {
		p.Temperature = openai.Float(*req.Temperature)
	}
	system := req.System
	if req.Output != nil {
		schema, _ := json.Marshal(req.Output.Schema)
		system = strings.TrimSpace(system + "\n\nRespond with only one JSON object (no prose, no code fence) that matches this JSON Schema:\n" + string(schema))
		p.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{OfJSONObject: &shared.ResponseFormatJSONObjectParam{}}
	}
	if system != "" {
		p.Messages = append(p.Messages, openai.SystemMessage(system))
	}
	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			p.Messages = append(p.Messages, openai.UserMessage(m.Text))
		case RoleAssistant:
			p.Messages = append(p.Messages, openai.AssistantMessage(m.Text))
		case RoleOperatorNote:
			p.Messages = append(p.Messages, openai.SystemMessage(OperatorNotesHeader+"\n- "+strings.TrimSpace(m.Text)))
		}
	}
	for _, t := range req.Tools {
		fd := shared.FunctionDefinitionParam{Name: t.Name, Parameters: shared.FunctionParameters(t.Parameters)}
		if t.Description != "" {
			fd.Description = openai.String(t.Description)
		}
		p.Tools = append(p.Tools, openai.ChatCompletionFunctionTool(fd))
	}
	return p
}

func (c *deepseekClient) call(ctx context.Context, p openai.ChatCompletionNewParams) (Response, error) {
	res, err := c.sdk.Chat.Completions.New(ctx, p)
	if err != nil {
		return Response{Model: p.Model}, openAIError(KindDeepSeek, err, c.cfg.APIKey)
	}
	return parseChatCompletion(KindDeepSeek, []byte(res.RawJSON()))
}

func (c *deepseekClient) Generate(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
	p := deepseekParams(req)
	resp, err := c.call(ctx, p)
	if err != nil || req.Output == nil || len(resp.ToolCalls) > 0 {
		resp.Latency = time.Since(start)
		return resp, err
	}
	raw := extractJSON(resp.Text)
	problems := ValidateJSON(raw, req.Output.Schema)
	if len(problems) > 0 {
		// One repair attempt: show the model its answer and the problems.
		p.Messages = append(p.Messages, openai.AssistantMessage(resp.Text), openai.UserMessage(repairPrompt(problems)))
		second, err2 := c.call(ctx, p)
		second.Usage = resp.Usage.Add(second.Usage)
		resp = second
		if err2 != nil {
			resp.Latency = time.Since(start)
			return resp, err2
		}
		raw = extractJSON(resp.Text)
		problems = ValidateJSON(raw, req.Output.Schema)
	}
	resp.Latency = time.Since(start)
	if len(problems) > 0 {
		return resp, &Error{Kind: KindInvalidOutput, Provider: KindDeepSeek, Message: shorten("the answer did not match the schema: " + strings.Join(problems, "; "))}
	}
	resp.Structured = json.RawMessage(raw)
	return resp, nil
}

// repairPrompt asks for a corrected answer. It is written by Messora, not
// by the customer.
func repairPrompt(problems []string) string {
	if len(problems) > 10 {
		problems = problems[:10]
	}
	return fmt.Sprintf("Your previous answer did not match the required JSON Schema (%s). Reply again with only the corrected JSON object.",
		strings.Join(problems, "; "))
}

type chatCompletion struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string `json:"content"`
			Refusal   string `json:"refusal"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens          int64  `json:"prompt_tokens"`
		CompletionTokens      int64  `json:"completion_tokens"`
		PromptCacheHitTokens  *int64 `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens *int64 `json:"prompt_cache_miss_tokens"`
		PromptTokensDetails   *struct {
			CachedTokens          *int64 `json:"cached_tokens"`
			PromptCacheHitTokens  *int64 `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens *int64 `json:"prompt_cache_miss_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails *struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

// parseChatCompletion maps a Chat Completions result. DeepSeek reports the
// cache split as prompt_cache_hit_tokens / prompt_cache_miss_tokens, at the
// top level of usage or under prompt_tokens_details; OpenAI-style
// prompt_tokens_details.cached_tokens is read as a fallback.
func parseChatCompletion(kind Kind, raw []byte) (Response, error) {
	var m chatCompletion
	if err := json.Unmarshal(raw, &m); err != nil {
		return Response{}, &Error{Kind: KindUnavailable, Provider: kind, Message: "unexpected response from the provider"}
	}
	u := m.Usage
	hit, miss := u.PromptCacheHitTokens, u.PromptCacheMissTokens
	if d := u.PromptTokensDetails; d != nil {
		if hit == nil {
			hit = d.PromptCacheHitTokens
		}
		if miss == nil {
			miss = d.PromptCacheMissTokens
		}
		if hit == nil {
			hit = d.CachedTokens
		}
	}
	usage := Usage{OutputTokens: u.CompletionTokens}
	switch {
	case hit != nil && miss != nil:
		usage.CachedInputTokens, usage.InputTokens = *hit, *miss
	case hit != nil:
		usage.CachedInputTokens, usage.InputTokens = *hit, max(0, u.PromptTokens-*hit)
	default:
		usage.InputTokens = u.PromptTokens
	}
	if d := u.CompletionTokensDetails; d != nil {
		usage.ReasoningTokens = d.ReasoningTokens
	}
	r := Response{Model: m.Model, Usage: usage, StopReason: StopOther}
	if len(m.Choices) == 0 {
		return r, &Error{Kind: KindUnavailable, Provider: kind, Message: "the provider returned no answer"}
	}
	ch := m.Choices[0]
	r.Text = ch.Message.Content
	for _, tc := range ch.Message.ToolCalls {
		args := json.RawMessage(tc.Function.Arguments)
		if !json.Valid(args) {
			args = json.RawMessage("{}")
		}
		r.ToolCalls = append(r.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: args})
	}
	switch ch.FinishReason {
	case "stop":
		r.StopReason = StopEnd
	case "length":
		r.StopReason = StopMaxTokens
	case "tool_calls":
		r.StopReason = StopToolUse
	case "content_filter":
		r.StopReason = StopRefusal
		return r, &Error{Kind: KindRefusal, Provider: kind, Message: "the response was filtered"}
	case "insufficient_system_resource":
		return r, &Error{Kind: KindOverloaded, Provider: kind, Message: "the provider ran out of capacity for this request"}
	}
	if ch.Message.Refusal != "" {
		r.StopReason = StopRefusal
		return r, &Error{Kind: KindRefusal, Provider: kind, Message: shorten(ch.Message.Refusal)}
	}
	return r, nil
}
