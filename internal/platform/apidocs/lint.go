package apidocs

import (
	"fmt"
	"strings"
)

// Lint checks that every public operation of the full spec is documented
// well enough to publish: operationId, summary, description and tags; an
// example for each JSON request body and for a JSON success response; the
// error responses every API key caller can get (401, 403, 429, plus 404 with
// path parameters and 422 with a body or parameters); and a scope. It
// returns one line per problem.
func Lint(full Doc) []string {
	var out []string
	for _, o := range Operations(full) {
		if !IsPublic(o.Op) {
			continue
		}
		name := strings.ToUpper(o.Method) + " " + o.Path
		bad := func(format string, args ...any) {
			out = append(out, name+": "+fmt.Sprintf(format, args...))
		}
		for _, f := range []string{"operationId", "summary", "description"} {
			if s, _ := o.Op[f].(string); strings.TrimSpace(s) == "" {
				bad("missing %s", f)
			}
		}
		if len(asList(o.Op["tags"])) == 0 {
			bad("missing tags")
		}
		if Scopes(o.Op) == nil && !anyKey(o.Op) {
			bad("apiKey requirement without scopes")
		}

		hasBody := o.Op["requestBody"] != nil
		if hasBody {
			body := resolve(full, o.Op["requestBody"])
			for mt, raw := range asMap(body["content"]) {
				if isJSON(mt) && !mediaHasExample(full, asMap(raw)) {
					bad("request body %s has no example", mt)
				}
			}
		}

		responses := asMap(o.Op["responses"])
		jsonSuccess, exampled := false, false
		for code, raw := range responses {
			if !strings.HasPrefix(code, "2") {
				continue
			}
			for mt, m := range asMap(resolve(full, raw)["content"]) {
				if isJSON(mt) {
					jsonSuccess = true
					if mediaHasExample(full, asMap(m)) {
						exampled = true
					}
				}
			}
		}
		if jsonSuccess && !exampled {
			bad("success response has no example")
		}

		want := []string{"401", "403", "429"}
		params := parameters(full, o)
		if hasPathParam(params) {
			want = append(want, "404")
		}
		if hasBody || len(params) > 0 {
			want = append(want, "422")
		}
		for _, code := range want {
			r, ok := responses[code]
			if !ok {
				bad("missing %s response", code)
				continue
			}
			if !isErrorResponse(full, r) {
				bad("%s response does not use the shared error format", code)
			}
		}
	}
	return out
}

// anyKey reports whether some alternative accepts any API key (no scope).
func anyKey(op map[string]any) bool {
	for _, r := range requirements(op) {
		if v, ok := r[SchemeAPIKey]; ok && len(asList(v)) == 0 {
			return true
		}
	}
	return false
}

func isJSON(mediaType string) bool {
	return strings.HasPrefix(mediaType, "application/json")
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// resolve follows a local $ref (repeatedly).
func resolve(d Doc, v any) map[string]any {
	m := asMap(v)
	for i := 0; i < 10 && m != nil; i++ {
		ref, ok := m["$ref"].(string)
		if !ok || !strings.HasPrefix(ref, "#/") {
			return m
		}
		var cur any = d
		for _, p := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			cur = asMap(cur)[unescapePointer(p)]
		}
		m = asMap(cur)
	}
	return m
}

func mediaHasExample(d Doc, media map[string]any) bool {
	if media["example"] != nil || len(asMap(media["examples"])) > 0 {
		return true
	}
	return schemaHasExample(d, media["schema"], 0)
}

// schemaHasExample accepts an example on the schema itself or, for the
// response envelope, on its data property (or the items of a data list).
func schemaHasExample(d Doc, v any, depth int) bool {
	if depth > 6 {
		return false
	}
	s := resolve(d, v)
	if s == nil {
		return false
	}
	if s["example"] != nil || len(asList(s["examples"])) > 0 {
		return true
	}
	if data, ok := asMap(s["properties"])["data"]; ok {
		return schemaHasExample(d, data, depth+1)
	}
	if items, ok := s["items"]; ok {
		return schemaHasExample(d, items, depth+1)
	}
	for _, k := range []string{"allOf", "oneOf", "anyOf"} {
		for _, x := range asList(s[k]) {
			if schemaHasExample(d, x, depth+1) {
				return true
			}
		}
	}
	return false
}

func parameters(d Doc, o Operation) []map[string]any {
	var out []map[string]any
	for _, src := range []any{o.Item["parameters"], o.Op["parameters"]} {
		for _, p := range asList(src) {
			if r := resolve(d, p); r != nil {
				out = append(out, r)
			}
		}
	}
	return out
}

func hasPathParam(params []map[string]any) bool {
	for _, p := range params {
		if p["in"] == "path" {
			return true
		}
	}
	return false
}

// isErrorResponse reports whether a response uses the ErrorEnvelope schema.
func isErrorResponse(d Doc, v any) bool {
	r := resolve(d, v)
	for _, m := range asMap(r["content"]) {
		if ref, _ := asMap(asMap(m)["schema"])["$ref"].(string); ref == "#/components/schemas/ErrorEnvelope" {
			return true
		}
	}
	return false
}
