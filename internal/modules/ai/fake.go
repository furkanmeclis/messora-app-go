package ai

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// FakeScript answers a request of the fake provider. Tests (P6–P8) install
// one with Service.SetFakeScript to drive conversations deterministically.
type FakeScript func(ctx context.Context, req Request) (Response, error)

// FakeClient is the scriptable provider of kind "fake". It is only
// available when AI_FAKE_PROVIDER is on (development and test).
//
// Without a script it answers deterministically: the text is
// "Fake reply to: <last user message>"; with an output schema it returns
// an object built from the schema (strings get that text, numbers 0.9,
// booleans false, enums their first value, arrays empty). A last user
// message starting with "fake:error:<kind>" fails with that error kind.
type FakeClient struct {
	Script FakeScript
}

// Generate implements Client.
func (f *FakeClient) Generate(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
	if f.Script != nil {
		r, err := f.Script(ctx, req)
		if r.Model == "" {
			r.Model = req.Model
		}
		r.Latency = time.Since(start)
		return r, err
	}
	last := ""
	for _, m := range req.Messages {
		if m.Role == RoleUser {
			last = m.Text
		}
	}
	in := int64(len(strings.Fields(req.System)))
	for _, m := range req.Messages {
		in += int64(len(strings.Fields(m.Text)))
	}
	r := Response{Model: req.Model, StopReason: StopEnd, Usage: Usage{InputTokens: max(in, 1)}}
	if kind, ok := strings.CutPrefix(strings.TrimSpace(last), "fake:error:"); ok {
		r.StopReason = StopOther
		return r, &Error{Kind: ErrorKind(strings.Fields(kind + " x")[0]), Provider: KindFake, Message: "scripted failure"}
	}
	r.Text = "Fake reply to: " + last
	if req.Output != nil {
		v := fakeValue(req.Output.Schema, r.Text)
		b, _ := json.Marshal(v)
		r.Structured = b
		r.Text = string(b)
	}
	r.Usage.OutputTokens = int64(len(strings.Fields(r.Text)))
	return r, nil
}

func fakeValue(s map[string]any, text string) any {
	if enum, ok := s["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}
	types := schemaTypes(s["type"])
	t := ""
	for _, x := range types {
		if x != "null" {
			t = x
			break
		}
	}
	switch t {
	case "object":
		props, _ := s["properties"].(map[string]any)
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := map[string]any{}
		for _, k := range keys {
			ps, _ := props[k].(map[string]any)
			out[k] = fakeValue(ps, text)
		}
		return out
	case "array":
		return []any{}
	case "number":
		return 0.9
	case "integer":
		return 1
	case "boolean":
		return false
	case "string":
		return text
	}
	return nil
}

func fakeModels(ProviderConfig) ([]ModelInfo, error) {
	yes := ptr(true)
	return []ModelInfo{
		{ID: "fake-small", DisplayName: "Fake small", ContextWindow: ptr(int64(32000)), MaxOutput: ptr(int64(4096)),
			Capabilities: Capabilities{Tools: yes, StructuredOutput: yes}, Lifecycle: ptr("active")},
		{ID: "fake-large", DisplayName: "Fake large", ContextWindow: ptr(int64(200000)), MaxOutput: ptr(int64(16000)),
			Capabilities: Capabilities{Tools: yes, StructuredOutput: yes, Thinking: yes}, Lifecycle: ptr("active")},
	}, nil
}
