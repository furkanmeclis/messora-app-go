package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

// ValidateJSON checks raw against the subset of JSON Schema that output
// schemas use: type (one or a list), properties, required,
// additionalProperties false, enum, items, minimum/maximum and
// minLength/maxLength. It returns one problem per violation (empty = valid).
// Providers without native schema enforcement (DeepSeek's json_object mode)
// are checked with it.
func ValidateJSON(raw []byte, schema map[string]any) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return []string{"not valid JSON: " + err.Error()}
	}
	if dec.More() {
		return []string{"not valid JSON: trailing data"}
	}
	var out []string
	validate(v, schema, "$", &out)
	return out
}

func validate(v any, s map[string]any, path string, out *[]string) {
	if s == nil {
		return
	}
	if types := schemaTypes(s["type"]); len(types) > 0 && !slices.ContainsFunc(types, func(t string) bool { return hasType(v, t) }) {
		*out = append(*out, fmt.Sprintf("%s: expected %s", path, strings.Join(types, " or ")))
		return
	}
	if enum, ok := s["enum"].([]any); ok && !slices.ContainsFunc(enum, func(e any) bool { return jsonEqual(e, v) }) {
		*out = append(*out, fmt.Sprintf("%s: not one of the allowed values", path))
	}
	switch x := v.(type) {
	case map[string]any:
		props, _ := s["properties"].(map[string]any)
		if req, ok := s["required"].([]any); ok {
			for _, r := range req {
				if name, _ := r.(string); name != "" {
					if _, has := x[name]; !has {
						*out = append(*out, fmt.Sprintf("%s: missing %q", path, name))
					}
				}
			}
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k].(map[string]any); ok {
				validate(x[k], ps, path+"."+k, out)
			} else if ap, ok := s["additionalProperties"].(bool); ok && !ap {
				*out = append(*out, fmt.Sprintf("%s: unexpected property %q", path, k))
			}
		}
	case []any:
		if items, ok := s["items"].(map[string]any); ok {
			for i, e := range x {
				validate(e, items, fmt.Sprintf("%s[%d]", path, i), out)
			}
		}
	case json.Number:
		f, _ := x.Float64()
		if mn, ok := number(s["minimum"]); ok && f < mn {
			*out = append(*out, fmt.Sprintf("%s: below the minimum %v", path, mn))
		}
		if mx, ok := number(s["maximum"]); ok && f > mx {
			*out = append(*out, fmt.Sprintf("%s: above the maximum %v", path, mx))
		}
	case string:
		n := len([]rune(x))
		if mn, ok := number(s["minLength"]); ok && float64(n) < mn {
			*out = append(*out, fmt.Sprintf("%s: shorter than %v characters", path, mn))
		}
		if mx, ok := number(s["maxLength"]); ok && float64(n) > mx {
			*out = append(*out, fmt.Sprintf("%s: longer than %v characters", path, mx))
		}
	}
}

func schemaTypes(t any) []string {
	switch x := t.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	}
	return nil
}

func hasType(v any, t string) bool {
	switch t {
	case "null":
		return v == nil
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "number":
		_, ok := v.(json.Number)
		return ok
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		f, err := n.Float64()
		return err == nil && f == math.Trunc(f)
	}
	return true
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}

// extractJSON returns the JSON object in a model's text output, tolerating
// a Markdown code fence around it.
func extractJSON(s string) []byte {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "```"); ok {
		rest = strings.TrimPrefix(rest, "json")
		if i := strings.LastIndex(rest, "```"); i >= 0 {
			rest = rest[:i]
		}
		s = strings.TrimSpace(rest)
	}
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i > 0 && j > i {
		s = s[i : j+1]
	}
	return []byte(s)
}
