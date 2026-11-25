package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

// openaiClient uses the Responses API with store=false, so OpenAI does not
// keep the response for later retrieval; API data is not used for training
// by default (see docs/AI.md).
type openaiClient struct {
	cfg ProviderConfig
	sdk openai.Client
}

func newOpenAI(cfg ProviderConfig) *openaiClient {
	return &openaiClient{cfg: cfg, sdk: openai.NewClient(openAIOptions(cfg)...)}
}

func openAIOptions(cfg ProviderConfig) []option.RequestOption {
	return []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(cfg.base() + "/"),
		option.WithHTTPClient(cfg.httpClient()),
		option.WithMaxRetries(0), // retries are the caller's decision (typed errors)
	}
}

// openAIParams builds the request: the system prompt is the instructions,
// operator notes are developer messages in their place in the conversation.
func openAIParams(req Request) responses.ResponseNewParams {
	p := responses.ResponseNewParams{
		Model:           req.Model,
		Store:           openai.Bool(false),
		MaxOutputTokens: openai.Int(maxTokens(req)),
	}
	if req.System != "" {
		p.Instructions = openai.String(req.System)
	}
	if req.Temperature != nil {
		p.Temperature = openai.Float(*req.Temperature)
	}
	var input responses.ResponseInputParam
	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			input = append(input, responses.ResponseInputItemParamOfMessage(m.Text, responses.EasyInputMessageRoleUser))
		case RoleAssistant:
			input = append(input, responses.ResponseInputItemParamOfMessage(m.Text, responses.EasyInputMessageRoleAssistant))
		case RoleOperatorNote:
			input = append(input, responses.ResponseInputItemParamOfMessage(OperatorNotesHeader+"\n- "+strings.TrimSpace(m.Text),
				responses.EasyInputMessageRoleDeveloper))
		}
	}
	p.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}
	for _, t := range req.Tools {
		tool := responses.ToolParamOfFunction(t.Name, t.Parameters, false)
		if t.Description != "" {
			tool.OfFunction.Description = openai.String(t.Description)
		}
		p.Tools = append(p.Tools, tool)
	}
	if req.Output != nil {
		f := responses.ResponseFormatTextConfigParamOfJSONSchema(schemaName(req.Output.Name), req.Output.Schema)
		f.OfJSONSchema.Strict = openai.Bool(true)
		p.Text = responses.ResponseTextConfigParam{Format: f}
	}
	return p
}

func schemaName(n string) string {
	if n == "" {
		return "output"
	}
	return n
}

func (c *openaiClient) Generate(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
	res, err := c.sdk.Responses.New(ctx, openAIParams(req))
	if err != nil {
		return Response{Latency: time.Since(start), Model: req.Model}, openAIError(KindOpenAI, err, c.cfg.APIKey)
	}
	resp, err := parseOpenAIResponse([]byte(res.RawJSON()), req.Output != nil)
	resp.Latency = time.Since(start)
	return resp, err
}

type openAIResponse struct {
	Model             string `json:"model"`
	Status            string `json:"status"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Output []struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Content   []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens        int64 `json:"input_tokens"`
		InputTokensDetails struct {
			CachedTokens     int64 `json:"cached_tokens"`
			CacheWriteTokens int64 `json:"cache_write_tokens"`
		} `json:"input_tokens_details"`
		OutputTokens        int64 `json:"output_tokens"`
		OutputTokensDetails struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

// parseOpenAIResponse maps a Responses API result. input_tokens includes
// cached reads and cache writes, which are priced separately, so they are
// subtracted; output_tokens includes reasoning.
func parseOpenAIResponse(raw []byte, structured bool) (Response, error) {
	var m openAIResponse
	if err := json.Unmarshal(raw, &m); err != nil {
		return Response{}, &Error{Kind: KindUnavailable, Provider: KindOpenAI, Message: "unexpected response from the provider"}
	}
	d := m.Usage.InputTokensDetails
	r := Response{Model: m.Model, Usage: Usage{
		InputTokens:       max(0, m.Usage.InputTokens-d.CachedTokens-d.CacheWriteTokens),
		CachedInputTokens: d.CachedTokens, CacheWriteTokens: d.CacheWriteTokens,
		OutputTokens: m.Usage.OutputTokens, ReasoningTokens: m.Usage.OutputTokensDetails.ReasoningTokens,
	}}
	var text []string
	refused := ""
	for _, o := range m.Output {
		switch o.Type {
		case "message":
			for _, c := range o.Content {
				switch c.Type {
				case "output_text":
					text = append(text, c.Text)
				case "refusal":
					refused = c.Refusal
				}
			}
		case "function_call":
			args := json.RawMessage(o.Arguments)
			if !json.Valid(args) {
				args = json.RawMessage("{}")
			}
			r.ToolCalls = append(r.ToolCalls, ToolCall{ID: o.CallID, Name: o.Name, Arguments: args})
		}
	}
	r.Text = strings.Join(text, "")
	switch {
	case refused != "":
		r.StopReason = StopRefusal
		return r, &Error{Kind: KindRefusal, Provider: KindOpenAI, Message: shorten(refused)}
	case m.Status == "incomplete" && m.IncompleteDetails != nil && m.IncompleteDetails.Reason == "content_filter":
		r.StopReason = StopRefusal
		return r, &Error{Kind: KindRefusal, Provider: KindOpenAI, Message: "the response was filtered"}
	case m.Status == "incomplete":
		r.StopReason = StopMaxTokens
	case len(r.ToolCalls) > 0:
		r.StopReason = StopToolUse
	default:
		r.StopReason = StopEnd
	}
	if structured && len(r.ToolCalls) == 0 {
		if !json.Valid([]byte(r.Text)) {
			return r, &Error{Kind: KindInvalidOutput, Provider: KindOpenAI, Message: "the structured output is not valid JSON"}
		}
		r.Structured = json.RawMessage(r.Text)
	}
	return r, nil
}

// openAIError maps an openai-go error (also used for DeepSeek). A 429 with
// code insufficient_quota is billing and must not be retried.
func openAIError(kind Kind, err error, key string) error {
	var oe *openai.Error
	if !errors.As(err, &oe) {
		return transportError(kind, err, key)
	}
	// The SDK decodes the "error" object into Code, Message and Type.
	body, _ := json.Marshal(map[string]any{"error": map[string]any{"message": oe.Message, "code": oe.Code, "type": oe.Type}})
	if oe.Response != nil {
		return httpError(kind, oe.StatusCode, oe.Response.Header, body, key)
	}
	return httpError(kind, oe.StatusCode, nil, body, key)
}
