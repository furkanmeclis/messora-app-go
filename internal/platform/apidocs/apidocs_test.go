package apidocs_test

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apidocs"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
)

var opts = apidocs.Options{Version: "v9.9.9", APIKeyPerMinute: apiguard.APIKeyPerMinute}

func embedded(t *testing.T) []byte {
	t.Helper()
	raw, err := api.GetSpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func publicDoc(t *testing.T) apidocs.Doc {
	t.Helper()
	b, err := apidocs.PublicJSON(embedded(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	var d apidocs.Doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// The CI lint: every public operation is fully documented.
func TestPublicOperationsAreDocumented(t *testing.T) {
	full, err := apidocs.Load(embedded(t))
	if err != nil {
		t.Fatal(err)
	}
	if problems := apidocs.Lint(full); len(problems) > 0 {
		t.Fatalf("%d documentation problems (see apidocs.Lint):\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

func TestLintReportsGaps(t *testing.T) {
	d, err := apidocs.Load([]byte(`
openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /things/{id}:
    post:
      security: [{ apiKey: [things.write] }]
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      requestBody:
        content: { application/json: { schema: { type: object } } }
      responses:
        "200":
          description: ok
          content: { application/json: { schema: { type: object } } }
  /internal:
    get:
      security: [{ panel: [] }]
      responses: { "204": { description: ok } }
`))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(apidocs.Lint(d), "\n")
	for _, want := range []string{
		"POST /things/{id}: missing operationId", "missing summary", "missing description", "missing tags",
		"request body application/json has no example", "success response has no example",
		"missing 401 response", "missing 403 response", "missing 404 response", "missing 422 response", "missing 429 response",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lint output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/internal") {
		t.Error("internal operations must not be linted")
	}
}

func TestPublicKeepsOnlyAPIKeyOperations(t *testing.T) {
	full, err := apidocs.Load(embedded(t))
	if err != nil {
		t.Fatal(err)
	}
	pub := publicDoc(t)

	var wantPublic, internal []string
	for _, o := range apidocs.Operations(full) {
		id := o.Method + " " + o.Path
		if apidocs.IsPublic(o.Op) {
			wantPublic = append(wantPublic, id)
		} else {
			internal = append(internal, id)
		}
	}
	var got []string
	for _, o := range apidocs.Operations(pub) {
		got = append(got, o.Method+" "+o.Path)
		for _, req := range o.Op["security"].([]any) {
			for scheme := range req.(map[string]any) {
				if scheme != apidocs.SchemeAPIKey {
					t.Errorf("%s %s keeps scheme %s", o.Method, o.Path, scheme)
				}
			}
		}
		if _, ok := o.Op[apidocs.ScopesExtension]; !ok {
			t.Errorf("%s %s lacks %s", o.Method, o.Path, apidocs.ScopesExtension)
		}
		if desc, _ := o.Op["description"].(string); !strings.Contains(desc, "**Scope:**") || !strings.Contains(desc, "per minute") {
			t.Errorf("%s %s: description does not state scope and rate limit", o.Method, o.Path)
		}
	}
	if !slices.Equal(got, wantPublic) {
		t.Fatalf("public operations differ:\n got %v\nwant %v", got, wantPublic)
	}
	for _, o := range apidocs.Operations(full) {
		if apidocs.IsPlatformPath(o.Path) && apidocs.IsPublic(o.Op) {
			t.Errorf("platform operation %s %s accepts API keys", o.Method, o.Path)
		}
	}
	if len(wantPublic) < 40 || len(internal) < 40 {
		t.Fatalf("suspicious split: %d public, %d internal", len(wantPublic), len(internal))
	}
	// Known panel-only operations never leak.
	b, _ := json.Marshal(pub)
	for _, leak := range []string{`"/auth/login"`, `"/auth/signup"`, `"/users"`, `"/roles"`, `"/api-keys"`, `"panel":`, `"session":`, `"platform":`,
		`"/platform/`, "msp_", "X-Messora-BFF", "LoginRequest", "PlatformTenant", "SupportGrant", "x-messora-host"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("public document contains %s", leak)
		}
	}
}

func TestPublicMetadata(t *testing.T) {
	pub := publicDoc(t)
	info := pub["info"].(map[string]any)
	if info["title"] != "Messora API" || info["version"] != "v9.9.9" {
		t.Fatalf("info = %v", info)
	}
	desc := info["description"].(string)
	for _, want := range []string{"Bearer msk_", "300 requests per minute", "RateLimit-Remaining", "Idempotency-Key", "next_cursor", "Messora-Signature", "VALIDATION_ERROR"} {
		if !strings.Contains(desc, want) {
			t.Errorf("info.description lacks %q", want)
		}
	}
	// Outside a workspace the server is the per-workspace address pattern;
	// there is no shared API host.
	if s := pub["servers"].([]any)[0].(map[string]any); s["url"] != "https://{workspace}.messora.tech/api/v1" || s["variables"] == nil {
		t.Fatalf("servers = %v", pub["servers"])
	}
	if !strings.Contains(desc, "https://<workspace>.messora.tech/api/v1") || !strings.Contains(desc, "no shared API host") {
		t.Errorf("generic introduction does not explain the per-workspace address:\n%s", desc)
	}
	groups := pub["x-tagGroups"].([]any)
	grouped := map[string]bool{}
	for _, g := range groups {
		for _, tag := range g.(map[string]any)["tags"].([]any) {
			grouped[tag.(string)] = true
		}
	}
	for _, tag := range pub["tags"].([]any) {
		if !grouped[tag.(map[string]any)["name"].(string)] {
			t.Errorf("tag %v is in no group", tag)
		}
	}
	comps := pub["components"].(map[string]any)
	if ss := comps["securitySchemes"].(map[string]any); len(ss) != 1 || ss[apidocs.SchemeAPIKey] == nil {
		t.Fatalf("security schemes = %v", ss)
	}
	for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "RateLimit-Policy", "Retry-After"} {
		if comps["headers"].(map[string]any)[h] == nil {
			t.Errorf("header %s not documented", h)
		}
	}
}

// Every $ref resolves and nothing unreferenced is left behind.
func TestPublicComponentsArePruned(t *testing.T) {
	pub := publicDoc(t)
	b, _ := json.Marshal(pub)
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("public document is invalid: %v", err)
	}
	text := string(b)
	for kind, raw := range pub["components"].(map[string]any) {
		if kind == "securitySchemes" {
			continue
		}
		for name := range raw.(map[string]any) {
			if !strings.Contains(text, `"#/components/`+kind+`/`+name+`"`) {
				t.Errorf("unreferenced component %s/%s kept", kind, name)
			}
		}
	}
	full, _ := apidocs.Load(embedded(t))
	if n, m := len(pub["components"].(map[string]any)["schemas"].(map[string]any)),
		len(full["components"].(map[string]any)["schemas"].(map[string]any)); n >= m {
		t.Fatalf("nothing pruned: %d of %d schemas", n, m)
	}
}

// The full spec, examples included, is valid (the guard refuses to start
// otherwise; kin-openapi checks examples against their schemas).
func TestFullSpecExamplesAreValid(t *testing.T) {
	spec, err := api.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryIsUpToDate(t *testing.T) {
	full, err := apidocs.Load(embedded(t))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := apidocs.Public(full, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := apidocs.Inventory(pub)
	got, err := os.ReadFile("../../../docs/API.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("docs/API.md is stale: run make api-docs")
	}
	if !strings.Contains(string(want), "| POST | `/messages` |") {
		t.Fatal("inventory lacks POST /messages")
	}
}
