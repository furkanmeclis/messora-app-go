package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"
)

// geminiClient uses generateContent through google.golang.org/genai. The
// Gemini API does not use prompts of paid-tier (billing-enabled) projects
// for training; free-tier keys may be, so docs/AI.md asks for a paid key.
type geminiClient struct {
	cfg ProviderConfig
}

func newGemini(cfg ProviderConfig) *geminiClient { return &geminiClient{cfg: cfg} }

// geminiRequest builds contents and config. Gemini has only a system
// instruction, so operator notes are appended to it.
func geminiRequest(req Request) ([]*genai.Content, *genai.GenerateContentConfig) {
	cfg := &genai.GenerateContentConfig{MaxOutputTokens: int32(min(maxTokens(req), 1<<31-1))} //nolint:gosec // clamped to int32 range
	system := req.System
	if notes := operatorNotes(req.Messages); notes != "" {
		system = strings.TrimSpace(system + "\n\n" + notes)
	}
	if system != "" {
		cfg.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: system}}}
	}
	if req.Temperature != nil {
		t := float32(*req.Temperature)
		cfg.Temperature = &t
	}
	var contents []*genai.Content
	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			contents = append(contents, genai.NewContentFromText(m.Text, genai.RoleUser))
		case RoleAssistant:
			contents = append(contents, genai.NewContentFromText(m.Text, genai.RoleModel))
		}
	}
	if len(req.Tools) > 0 {
		var decls []*genai.FunctionDeclaration
		for _, t := range req.Tools {
			decls = append(decls, &genai.FunctionDeclaration{Name: t.Name, Description: t.Description, ParametersJsonSchema: t.Parameters})
		}
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: decls}}
	}
	if req.Output != nil {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseJsonSchema = req.Output.Schema
	}
	return contents, cfg
}

func (c *geminiClient) Generate(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
	cc := &genai.ClientConfig{APIKey: c.cfg.APIKey, Backend: genai.BackendGeminiAPI, HTTPClient: c.cfg.httpClient(),
		HTTPOptions: genai.HTTPOptions{BaseURL: c.cfg.base() + "/", APIVersion: "v1beta"}}
	client, err := genai.NewClient(ctx, cc)
	if err != nil {
		return Response{Model: req.Model}, &Error{Kind: KindBadRequest, Provider: KindGemini, Message: scrub(err.Error(), c.cfg.APIKey)}
	}
	contents, cfg := geminiRequest(req)
	res, err := client.Models.GenerateContent(ctx, req.Model, contents, cfg)
	if err != nil {
		return Response{Latency: time.Since(start), Model: req.Model}, geminiError(err, c.cfg.APIKey)
	}
	resp, err := parseGemini(res, req.Output != nil)
	if resp.Model == "" {
		resp.Model = req.Model
	}
	resp.Latency = time.Since(start)
	return resp, err
}

// parseGemini maps a generateContent result. promptTokenCount includes the
// cached part (cachedContentTokenCount); thinking tokens
// (thoughtsTokenCount) are billed as output.
func parseGemini(res *genai.GenerateContentResponse, structured bool) (Response, error) {
	r := Response{Model: res.ModelVersion, StopReason: StopOther}
	if u := res.UsageMetadata; u != nil {
		cached := int64(u.CachedContentTokenCount)
		r.Usage = Usage{InputTokens: max(0, int64(u.PromptTokenCount)-cached) + int64(u.ToolUsePromptTokenCount),
			CachedInputTokens: cached, OutputTokens: int64(u.CandidatesTokenCount) + int64(u.ThoughtsTokenCount),
			ReasoningTokens: int64(u.ThoughtsTokenCount)}
	}
	if pf := res.PromptFeedback; pf != nil && pf.BlockReason != "" {
		r.StopReason = StopRefusal
		return r, &Error{Kind: KindRefusal, Provider: KindGemini, Message: "the prompt was blocked: " + string(pf.BlockReason)}
	}
	if len(res.Candidates) == 0 {
		return r, &Error{Kind: KindUnavailable, Provider: KindGemini, Message: "the provider returned no answer"}
	}
	cand := res.Candidates[0]
	var text []string
	if cand.Content != nil {
		for i, p := range cand.Content.Parts {
			switch {
			case p.FunctionCall != nil:
				args, _ := json.Marshal(p.FunctionCall.Args)
				if p.FunctionCall.Args == nil {
					args = []byte("{}")
				}
				id := p.FunctionCall.ID
				if id == "" {
					id = p.FunctionCall.Name + "#" + strconv.Itoa(i)
				}
				r.ToolCalls = append(r.ToolCalls, ToolCall{ID: id, Name: p.FunctionCall.Name, Arguments: args})
			case p.Thought:
				// Thought summaries are not part of the answer.
			case p.Text != "":
				text = append(text, p.Text)
			}
		}
	}
	r.Text = strings.Join(text, "")
	switch cand.FinishReason {
	case genai.FinishReasonStop:
		r.StopReason = StopEnd
	case genai.FinishReasonMaxTokens:
		r.StopReason = StopMaxTokens
	case genai.FinishReasonSafety, genai.FinishReasonRecitation, genai.FinishReasonBlocklist,
		genai.FinishReasonProhibitedContent, genai.FinishReasonSPII:
		r.StopReason = StopRefusal
		return r, &Error{Kind: KindRefusal, Provider: KindGemini, Message: "the answer was blocked: " + string(cand.FinishReason)}
	}
	if len(r.ToolCalls) > 0 {
		r.StopReason = StopToolUse
	}
	if structured && len(r.ToolCalls) == 0 {
		if !json.Valid([]byte(r.Text)) {
			return r, &Error{Kind: KindInvalidOutput, Provider: KindGemini, Message: "the structured output is not valid JSON"}
		}
		r.Structured = json.RawMessage(r.Text)
	}
	return r, nil
}

// geminiError maps genai.APIError. 429 RESOURCE_EXHAUSTED carries no
// Retry-After, so the caller backs off with jitter.
func geminiError(err error, key string) error {
	var ae genai.APIError
	if !errors.As(err, &ae) {
		var pae *genai.APIError
		if !errors.As(err, &pae) {
			return transportError(KindGemini, err, key)
		}
		ae = *pae
	}
	reason := ""
	for _, d := range ae.Details {
		if r, ok := d["reason"].(string); ok && r != "" {
			reason = r
		}
	}
	e := &Error{Kind: byStatus(ae.Code), Provider: KindGemini, Status: ae.Code, Code: reason, Message: scrub(shorten(ae.Message), key)}
	refine(e, "", ae.Status)
	return e
}
