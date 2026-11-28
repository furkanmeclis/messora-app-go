package apidocs_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/platform/apidocs"
)

// Each workspace's document names the workspace's own API address as the
// server (Scalar builds every code sample from it) and speaks about the
// workspace's keys; the platform panel stays out.
func TestDocumentPerWorkspace(t *testing.T) {
	p, err := apidocs.NewPublished(embedded(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	w := apidocs.Workspace{Name: "Acme Gıda", BaseURL: "https://acme.messora.tech", RequestURL: "https://api.acmegida.com.tr"}
	b, err := p.For(w)
	if err != nil {
		t.Fatal(err)
	}
	var d apidocs.Doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	servers := d["servers"].([]any)
	// The address the reader opened first (Scalar's "try it" target, same
	// origin), the primary address second.
	if len(servers) != 2 || servers[0].(map[string]any)["url"] != "https://api.acmegida.com.tr/api/v1" ||
		servers[1].(map[string]any)["url"] != "https://acme.messora.tech/api/v1" {
		t.Fatalf("servers = %v", servers)
	}
	desc := d["info"].(map[string]any)["description"].(string)
	for _, want := range []string{"**Acme Gıda** workspace", "curl https://api.acmegida.com.tr/api/v1/api-keys/whoami", "401 API_KEY_INVALID", "FEATURE_NOT_IN_PLAN"} {
		if !strings.Contains(desc, want) {
			t.Errorf("introduction lacks %q", want)
		}
	}
	if strings.Contains(desc, "<workspace>") {
		t.Error("workspace introduction still shows the placeholder address")
	}
	key := d["components"].(map[string]any)["securitySchemes"].(map[string]any)[apidocs.SchemeAPIKey].(map[string]any)
	if !strings.Contains(key["description"].(string), "Acme Gıda") {
		t.Errorf("API key scheme: %v", key["description"])
	}
	for path := range d["paths"].(map[string]any) {
		if apidocs.IsPlatformPath(path) {
			t.Errorf("workspace document lists %s", path)
		}
	}
	// The generic document is untouched by rendering a workspace's.
	if strings.Contains(string(p.Generic()), "Acme") {
		t.Fatal("generic document changed")
	}
	// Same address: one server; another workspace gets its own.
	b, _ = p.For(apidocs.Workspace{Name: "Beta", BaseURL: "https://beta.messora.tech", RequestURL: "https://beta.messora.tech"})
	if !strings.Contains(string(b), `"url": "https://beta.messora.tech/api/v1"`) || strings.Contains(string(b), "acme.messora.tech") {
		t.Fatal("beta's document")
	}
	if err := json.Unmarshal(b, &d); err != nil || len(d["servers"].([]any)) != 1 {
		t.Fatalf("beta servers: %v %v", err, d["servers"])
	}
}
