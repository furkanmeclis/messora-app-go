package edge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

type fakeResolver map[string]error

func (f fakeResolver) Resolve(_ context.Context, host string) (tenant.Info, error) {
	err, ok := f[host]
	if !ok {
		return tenant.Info{}, tenant.ErrUnknownHost
	}
	return tenant.Info{Slug: host}, err
}

func ask(h http.Handler, domain string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, AllowPath+"?domain="+url.QueryEscape(domain), nil))
	return rec.Code
}

func TestPlatformHosts(t *testing.T) {
	got := PlatformHosts(" Messora.Tech. ")
	want := []string{"messora.tech", "app.messora.tech", "www.messora.tech", "admin.messora.tech", "hooks.messora.tech"}
	if !slices.Equal(got, want) {
		t.Fatalf("PlatformHosts = %v", got)
	}
	if PlatformHosts("") != nil {
		t.Fatal("no base domain, no platform hosts")
	}
}

func TestAllowHandler(t *testing.T) {
	res := fakeResolver{
		"acme.messora.tech":  nil,
		"panel.acme.example": nil,
		"frozen.example":     tenant.ErrSuspended,
		"broken.example":     errors.New("database down"),
	}
	h := AllowHandler(Options{Resolver: res, BaseDomain: "messora.tech"})
	for domain, want := range map[string]int{
		"admin.messora.tech": http.StatusOK, // platform host, no lookup
		"messora.tech":       http.StatusOK,
		"HOOKS.messora.tech": http.StatusOK,
		"acme.messora.tech":  http.StatusOK,
		"panel.acme.example": http.StatusOK,
		"Panel.Acme.Example": http.StatusOK,
		"frozen.example":     http.StatusForbidden,
		"stranger.example":   http.StatusForbidden,
		"api.messora.tech":   http.StatusForbidden, // no API host (PHASE-17 §2)
		"broken.example":     http.StatusServiceUnavailable,
		"":                   http.StatusBadRequest,
		"a.example:443":      http.StatusBadRequest,
		"a.example/x":        http.StatusBadRequest,
		"bad host.example":   http.StatusBadRequest,
	} {
		if got := ask(h, domain); got != want {
			t.Errorf("ask(%q) = %d, want %d", domain, got, want)
		}
	}
}

func TestAllowHandlerRateLimit(t *testing.T) {
	h := AllowHandler(Options{Resolver: fakeResolver{}, BaseDomain: "messora.tech", AskRate: 0.001, AskBurst: 2})
	codes := []int{ask(h, "a.example"), ask(h, "b.example"), ask(h, "c.example")}
	if !slices.Equal(codes, []int{http.StatusForbidden, http.StatusForbidden, http.StatusTooManyRequests}) {
		t.Fatalf("codes = %v", codes)
	}
	// Platform hosts are never limited: renewals of the admin host must not
	// wait behind a flood of strangers.
	if got := ask(h, "admin.messora.tech"); got != http.StatusOK {
		t.Fatalf("platform host under limit = %d", got)
	}
}

func TestParseTraefikOptions(t *testing.T) {
	o, err := parseTraefikOptions(url.Values{})
	if err != nil || o.Upstream != "http://messora-edge:80" || o.Resolver != "letsencrypt" || o.Web != "web" || o.WebSecure != "websecure" {
		t.Fatalf("defaults = %+v, %v", o, err)
	}
	o, err = parseTraefikOptions(url.Values{"upstream": {"http://staging-edge:80"}, "resolver": {"le-http"}})
	if err != nil || o.Upstream != "http://staging-edge:80" || o.Resolver != "le-http" {
		t.Fatalf("custom = %+v, %v", o, err)
	}
	for _, bad := range []url.Values{
		{"upstream": {"https://x:80"}},
		{"upstream": {"http://user:pw@x:80"}},
		{"upstream": {"http://x:80/path"}},
		{"upstream": {"http://x:80?q=1"}},
		{"resolver": {"le`) || Host(`x"}},
		{"web": {"a b"}},
	} {
		if _, err := parseTraefikOptions(bad); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}

func TestTraefikConfig(t *testing.T) {
	if got := TraefikConfig(nil, TraefikOptions{}); len(got) != 0 {
		t.Fatalf("no hosts = %v", got)
	}
	o, _ := parseTraefikOptions(url.Values{})
	raw, err := json.Marshal(TraefikConfig([]string{"panel.acme.example", "chat.beta.example"}, o))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		HTTP struct {
			Routers map[string]struct {
				Rule        string   `json:"rule"`
				EntryPoints []string `json:"entryPoints"`
				Service     string   `json:"service"`
				Middlewares []string `json:"middlewares"`
				TLS         *struct {
					CertResolver string `json:"certResolver"`
				} `json:"tls"`
			} `json:"routers"`
			Services map[string]struct {
				LoadBalancer struct {
					Servers []struct {
						URL string `json:"url"`
					} `json:"servers"`
				} `json:"loadBalancer"`
			} `json:"services"`
			Middlewares map[string]any `json:"middlewares"`
		} `json:"http"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.HTTP.Routers) != 4 {
		t.Fatalf("routers = %d, want 2 per host", len(doc.HTTP.Routers))
	}
	secure, plain := 0, 0
	for name, r := range doc.HTTP.Routers {
		if r.Service != "messora-custom-domains" {
			t.Errorf("%s service = %q", name, r.Service)
		}
		switch {
		case r.TLS != nil:
			secure++
			if r.TLS.CertResolver != "letsencrypt" || !slices.Equal(r.EntryPoints, []string{"websecure"}) {
				t.Errorf("%s = %+v", name, r)
			}
		default:
			plain++
			if !slices.Equal(r.EntryPoints, []string{"web"}) || len(r.Middlewares) != 1 {
				t.Errorf("%s = %+v", name, r)
			}
		}
	}
	if secure != 2 || plain != 2 {
		t.Fatalf("secure %d plain %d", secure, plain)
	}
	if got := doc.HTTP.Services["messora-custom-domains"].LoadBalancer.Servers; len(got) != 1 || got[0].URL != "http://messora-edge:80" {
		t.Fatalf("servers = %+v", got)
	}
	if _, ok := doc.HTTP.Middlewares["messora-custom-domains-https"]; !ok {
		t.Fatal("redirect middleware missing")
	}
}

func TestTraefikHandlerRejectsBadOptions(t *testing.T) {
	rec := httptest.NewRecorder()
	TraefikHandler(Options{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, TraefikPath+"?upstream=ftp://x", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}
