// Package apidocs derives the public developer documentation from the full
// OpenAPI contract (api/openapi.yaml).
//
// The public API is exactly the set of operations whose security requirement
// accepts the apiKey scheme; everything else (panel sign-in flows, panel-only
// administration) is internal and never published. The same derivation feeds
// GET /api/v1/openapi.json, the Scalar reference at /docs/api and the
// endpoint inventory in docs/API.md, and Lint keeps the documentation of every
// public operation complete.
package apidocs

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"

	"go.yaml.in/yaml/v4"
)

// Security scheme names (kept in sync with apiguard).
const (
	SchemeAPIKey  = "apiKey"
	SchemePanel   = "panel"
	SchemeSession = "session"
)

// ScopesExtension lists, on every public operation, the API key scopes that
// grant it (any one of them is enough).
const ScopesExtension = "x-required-scopes"

// RateLimitExtension states an operation's request budget in the public spec.
const RateLimitExtension = "x-rate-limit"

// Methods in the order they are listed.
var Methods = []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"}

// Options configures the public document.
type Options struct {
	// Version is the application version (info.version).
	Version string
	// APIKeyPerMinute is the request budget of one API key.
	APIKeyPerMinute int
}

// Doc is a parsed OpenAPI document as plain JSON values.
type Doc = map[string]any

// Load parses a YAML or JSON OpenAPI document.
func Load(raw []byte) (Doc, error) {
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("apidocs: parse spec: %w", err)
	}
	d, ok := normalize(v).(map[string]any)
	if !ok {
		return nil, errors.New("apidocs: spec is not an object")
	}
	return d, nil
}

// normalize turns YAML values into JSON-compatible ones (string map keys).
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = normalize(x)
		}
		return t
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[fmt.Sprint(k)] = normalize(x)
		}
		return out
	case []any:
		for i, x := range t {
			t[i] = normalize(x)
		}
		return t
	}
	return v
}

// Operation is one operation of a document.
type Operation struct {
	Method, Path string
	// Op is the operation object; Item is its path item.
	Op, Item map[string]any
}

// Operations lists the operations of d, sorted by path then method.
func Operations(d Doc) []Operation {
	paths, _ := d["paths"].(map[string]any)
	var out []Operation
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		item, _ := paths[path].(map[string]any)
		for _, m := range Methods {
			if op, ok := item[m].(map[string]any); ok {
				out = append(out, Operation{Method: m, Path: path, Op: op, Item: item})
			}
		}
	}
	return out
}

// requirements returns the operation's security requirements.
func requirements(op map[string]any) []map[string]any {
	list, _ := op["security"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, r := range list {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// IsPublic reports whether an API key may call the operation.
func IsPublic(op map[string]any) bool {
	for _, r := range requirements(op) {
		if _, ok := r[SchemeAPIKey]; ok {
			return true
		}
	}
	return false
}

// Scopes returns the API key scopes that grant op, any one being enough. An
// empty result with IsPublic means any valid key may call it. A requirement
// needing several scopes at once is returned joined with " + ".
func Scopes(op map[string]any) []string {
	var out []string
	for _, r := range requirements(op) {
		v, ok := r[SchemeAPIKey]
		if !ok {
			continue
		}
		list, _ := v.([]any)
		var all []string
		for _, s := range list {
			all = append(all, fmt.Sprint(s))
		}
		if len(all) == 0 {
			return nil // one alternative needs no scope at all
		}
		out = append(out, strings.Join(all, " + "))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

//go:embed intro.md
var introTemplate string

// TagGroups groups the public tags in the reference's navigation. Tags not
// listed here end up in "More".
var TagGroups = []struct {
	Name string
	Tags []string
}{
	{"Messaging", []string{"messages", "templates", "channels"}},
	{"Inbox", []string{"conversations"}},
	{"Contacts & audiences", []string{"contacts", "segments"}},
	{"Campaigns", []string{"campaigns"}},
	{"Automation & integrations", []string{"automations", "webhooks"}},
	{"Account", []string{"api-keys"}},
}

// Public derives the public document from the full one. full is not
// modified.
func Public(full Doc, o Options) (Doc, error) {
	if o.APIKeyPerMinute <= 0 {
		return nil, errors.New("apidocs: APIKeyPerMinute is required")
	}
	d, err := clone(full)
	if err != nil {
		return nil, err
	}

	usedTags := map[string]bool{}
	paths, _ := d["paths"].(map[string]any)
	for path, raw := range paths {
		item, _ := raw.(map[string]any)
		kept := 0
		for _, m := range Methods {
			op, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			if !IsPublic(op) {
				delete(item, m)
				continue
			}
			kept++
			publishOperation(op, o)
			for _, t := range asStrings(op["tags"]) {
				usedTags[t] = true
			}
		}
		if kept == 0 {
			delete(paths, path)
		}
	}

	d["info"] = info(full, o)
	d["servers"] = []any{map[string]any{"url": "/api/v1", "description": "This Messora installation"}}
	d["security"] = []any{map[string]any{SchemeAPIKey: []any{}}}

	// Tags: only those in use, in the order of the groups.
	var tags []any
	var groups []any
	byName := map[string]map[string]any{}
	for _, t := range asList(full["tags"]) {
		if m, ok := t.(map[string]any); ok {
			byName[fmt.Sprint(m["name"])] = m
		}
	}
	grouped := map[string]bool{}
	addTag := func(name string) {
		t := map[string]any{"name": name}
		if src, ok := byName[name]; ok {
			for k, v := range src {
				t[k] = v
			}
		}
		tags = append(tags, t)
	}
	for _, g := range TagGroups {
		var names []any
		for _, t := range g.Tags {
			if usedTags[t] {
				names = append(names, t)
				grouped[t] = true
				addTag(t)
			}
		}
		if len(names) > 0 {
			groups = append(groups, map[string]any{"name": g.Name, "tags": names})
		}
	}
	var rest []any
	for _, t := range slices.Sorted(maps.Keys(usedTags)) {
		if !grouped[t] {
			rest = append(rest, t)
			addTag(t)
		}
	}
	if len(rest) > 0 {
		groups = append(groups, map[string]any{"name": "More", "tags": rest})
	}
	d["tags"] = tags
	d["x-tagGroups"] = groups

	comps, _ := d["components"].(map[string]any)
	if comps != nil {
		if ss, ok := comps["securitySchemes"].(map[string]any); ok {
			for name := range ss {
				if name != SchemeAPIKey {
					delete(ss, name)
				}
			}
			if k, ok := ss[SchemeAPIKey].(map[string]any); ok {
				k["description"] = "An API key created under **API keys** in the panel: " +
					"`Authorization: Bearer msk_live_…`. The key's scopes decide which operations it may call."
			}
		}
		prune(d)
	}
	return d, nil
}

// publishOperation rewrites one public operation for outside readers.
func publishOperation(op map[string]any, o Options) {
	var reqs []any
	for _, r := range requirements(op) {
		if v, ok := r[SchemeAPIKey]; ok {
			reqs = append(reqs, map[string]any{SchemeAPIKey: v})
		}
	}
	op["security"] = reqs
	scopes := Scopes(op)
	list := make([]any, 0, len(scopes))
	for _, s := range scopes {
		list = append(list, s)
	}
	op[ScopesExtension] = list
	op[RateLimitExtension] = map[string]any{"limit": o.APIKeyPerMinute, "window": "1m", "per": "api key"}

	desc := strings.TrimSpace(fmt.Sprint(op["description"]))
	if op["description"] == nil {
		desc = ""
	}
	var note string
	switch len(scopes) {
	case 0:
		note = "**Scope:** any valid API key."
	case 1:
		note = "**Scope:** `" + scopes[0] + "`."
	default:
		note = "**Scope:** one of `" + strings.Join(scopes, "`, `") + "`."
	}
	note += fmt.Sprintf(" **Rate limit:** %d requests per minute per API key.", o.APIKeyPerMinute)
	if desc != "" {
		desc += "\n\n"
	}
	op["description"] = desc + note

	// Every response that went through the guard carries the budget.
	responses, _ := op["responses"].(map[string]any)
	for code, raw := range responses {
		resp, ok := raw.(map[string]any)
		if !ok || resp["$ref"] != nil || !strings.HasPrefix(code, "2") {
			continue
		}
		headers, _ := resp["headers"].(map[string]any)
		if headers == nil {
			headers = map[string]any{}
			resp["headers"] = headers
		}
		for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "RateLimit-Policy"} {
			if _, ok := headers[h]; !ok {
				headers[h] = map[string]any{"$ref": "#/components/headers/" + h}
			}
		}
	}
}

func info(full Doc, o Options) map[string]any {
	var buf bytes.Buffer
	t := template.Must(template.New("intro").Parse(introTemplate))
	if err := t.Execute(&buf, o); err != nil {
		panic(err) // the embedded template is fixed and tested
	}
	out := map[string]any{
		"title":       "Messora API",
		"version":     o.Version,
		"description": buf.String(),
	}
	if src, ok := full["info"].(map[string]any); ok {
		if l, ok := src["license"]; ok {
			out["license"] = l
		}
	}
	return out
}

// prune removes components that nothing outside components (transitively)
// refers to.
func prune(d Doc) {
	comps := d["components"].(map[string]any)
	used := map[string]bool{}
	var queue []string
	var visit func(v any)
	visit = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if ref, ok := t["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/") && !used[ref] {
				used[ref] = true
				queue = append(queue, ref)
			}
			for _, x := range t {
				visit(x)
			}
		case []any:
			for _, x := range t {
				visit(x)
			}
		}
	}
	for k, v := range d {
		if k != "components" {
			visit(v)
		}
	}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		parts := strings.SplitN(strings.TrimPrefix(ref, "#/components/"), "/", 2)
		if len(parts) != 2 {
			continue
		}
		if kind, ok := comps[parts[0]].(map[string]any); ok {
			visit(kind[unescapePointer(parts[1])])
		}
	}
	for kindName, raw := range comps {
		kind, ok := raw.(map[string]any)
		if !ok || kindName == "securitySchemes" {
			continue
		}
		for name := range kind {
			if !used["#/components/"+kindName+"/"+escapePointer(name)] {
				delete(kind, name)
			}
		}
		if len(kind) == 0 {
			delete(comps, kindName)
		}
	}
}

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func unescapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

func clone(d Doc) (Doc, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("apidocs: %w", err)
	}
	var out Doc
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("apidocs: %w", err)
	}
	return out, nil
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func asStrings(v any) []string {
	var out []string
	for _, x := range asList(v) {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

// PublicJSON loads raw (the full spec) and returns the public document as
// indented JSON.
func PublicJSON(raw []byte, o Options) ([]byte, error) {
	full, err := Load(raw)
	if err != nil {
		return nil, err
	}
	pub, err := Public(full, o)
	if err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(pub, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("apidocs: %w", err)
	}
	return append(b, '\n'), nil
}
