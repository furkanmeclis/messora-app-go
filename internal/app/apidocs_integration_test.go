//go:build integration

package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
)

func TestPublicOpenAPIDocument(t *testing.T) {
	h := newHarness(t)

	// Anonymous, from outside the internal network.
	req, _ := http.NewRequest(http.MethodGet, h.srv.URL+"/api/v1/openapi.json", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	var doc map[string]any
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if info := doc["info"].(map[string]any); info["title"] != "Messora API" || info["version"] != Version {
		t.Fatalf("info = %v", info)
	}
	paths := doc["paths"].(map[string]any)
	if paths["/messages"] == nil || paths["/contacts"] == nil || paths["/api-keys/whoami"] == nil {
		t.Fatal("public operations missing")
	}
	for _, internal := range []string{"/auth/login", "/setup", "/users", "/roles", "/me", "/settings", "/api-keys", "/system/status"} {
		if paths[internal] != nil {
			t.Errorf("panel-only path %s leaked", internal)
		}
	}
	for p, item := range paths {
		for m, op := range item.(map[string]any) {
			for _, req := range op.(map[string]any)["security"].([]any) {
				if _, ok := req.(map[string]any)["apiKey"]; !ok {
					t.Errorf("%s %s keeps a non-API-key requirement", m, p)
				}
			}
		}
	}

	// Conditional requests.
	req, _ = http.NewRequest(http.MethodGet, h.srv.URL+"/api/v1/openapi.json", nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusNotModified {
		t.Fatalf("If-None-Match: %d", res2.StatusCode)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	r := h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: map[string]any{"name": "Load test", "scopes": []string{"contacts.read"}}})
	h.expect(r, 201, "")
	key := r.data()["secret"].(string)

	limit := strconv.Itoa(apiguard.APIKeyPerMinute)
	// The budget refills while the loop runs (one request per 200 ms), so
	// the 429 comes a few requests after the nominal limit.
	for i := 0; ; i++ {
		if i > 2*apiguard.APIKeyPerMinute {
			t.Fatal("never rate limited")
		}
		r = h.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: key, noBFF: true})
		if r.status == http.StatusTooManyRequests {
			if i < apiguard.APIKeyPerMinute {
				t.Fatalf("limited after %d requests", i)
			}
			break
		}
		h.expect(r, 200, "")
		if r.header.Get("RateLimit-Limit") != limit || r.header.Get("RateLimit-Policy") != limit+";w=60" {
			t.Fatalf("request %d: limit headers %v", i, r.header)
		}
		rem, err := strconv.Atoi(r.header.Get("RateLimit-Remaining"))
		if err != nil || rem < 0 || rem >= apiguard.APIKeyPerMinute {
			t.Fatalf("request %d: remaining %q", i, r.header.Get("RateLimit-Remaining"))
		}
		if i == 0 && rem != apiguard.APIKeyPerMinute-1 {
			t.Fatalf("first request: remaining %d", rem)
		}
		if reset, err := strconv.Atoi(r.header.Get("RateLimit-Reset")); err != nil || reset < 1 || reset > 60 {
			t.Fatalf("request %d: reset %q", i, r.header.Get("RateLimit-Reset"))
		}
	}
	h.expect(r, 429, "RATE_LIMITED")
	if r.header.Get("Retry-After") == "" || r.header.Get("RateLimit-Remaining") != "0" || r.header.Get("RateLimit-Limit") != limit {
		t.Fatalf("429 headers: %v", r.header)
	}
	// Sessions have their own budget.
	r = h.do(call{method: "GET", path: "/api/v1/me", token: owner})
	h.expect(r, 200, "")
	if r.header.Get("RateLimit-Limit") != strconv.Itoa(apiguard.SessionPerMinute) {
		t.Fatalf("session limit header: %v", r.header)
	}
}
