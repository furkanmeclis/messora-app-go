package apiguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

func specWith(t *testing.T, path, security, host string) *openapi3.T {
	t.Helper()
	ext := ""
	if host != "" {
		ext = "\n      x-messora-host: " + host
	}
	doc := `openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  ` + path + `:
    get:
      operationId: op
      security: [{ ` + security + `: [] }]` + ext + `
      responses:
        "200": {description: ok}
components:
  securitySchemes:
    panel: {type: apiKey, in: header, name: X-Messora-BFF}
    session: {type: http, scheme: bearer}
    apiKey: {type: http, scheme: bearer}
    platform: {type: http, scheme: bearer}
`
	s, err := openapi3.NewLoader().LoadFromData([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The guard refuses a spec that would let a platform session reach a
// workspace operation, or workspace credentials reach the platform panel.
func TestSpecHostRules(t *testing.T) {
	for _, c := range []struct {
		path, security, host string
		ok                   bool
	}{
		{"/contacts", "session", "", true},
		{"/auth/signup", "panel", "any", true},
		{"/platform/tenants", "platform", "platform", true},
		{"/platform/auth/login", "panel", "platform", true},
		{"/platform/tenants", "platform", "", false},        // host missing
		{"/platform/tenants", "session", "platform", false}, // workspace session on the panel
		{"/platform/tenants", "apiKey", "platform", false},
		{"/contacts", "platform", "", false},         // platform session on a workspace op
		{"/contacts", "session", "platform", false},  // platform host outside /platform/
		{"/contacts", "session", "elsewhere", false}, // unknown scope
	} {
		_, err := New(specWith(t, c.path, c.security, c.host), Options{})
		if (err == nil) != c.ok {
			t.Errorf("%s %s %q: err=%v, want ok=%v", c.path, c.security, c.host, err, c.ok)
		}
	}
	// The embedded spec passes.
	spec, err := api.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(spec, Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckHost(t *testing.T) {
	req := func(ctx context.Context) *http.Request {
		return httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	}
	bg := context.Background()
	tenantCtx := tenant.WithHostKind(tenant.WithID(bg, uuid.New()), tenant.HostTenant)
	adminCtx := tenant.WithHostKind(tenant.Platform(bg), tenant.HostAdmin)
	publicCtx := tenant.WithHostKind(tenant.System(bg), tenant.HostPlatform)
	code := func(err error) string {
		if e, ok := apperr.As(err); ok {
			return e.Code
		}
		return ""
	}
	for _, c := range []struct {
		ctx   context.Context
		scope string
		want  string
	}{
		{tenantCtx, HostTenant, ""}, {tenantCtx, HostAny, ""}, {tenantCtx, HostPlatform, "NOT_FOUND"},
		{adminCtx, HostPlatform, ""}, {adminCtx, HostTenant, "TENANT_NOT_FOUND"}, {adminCtx, HostAny, ""},
		{publicCtx, HostAny, ""}, {publicCtx, HostTenant, "TENANT_NOT_FOUND"}, {publicCtx, HostPlatform, "NOT_FOUND"},
	} {
		if got := code(checkHost(req(c.ctx), c.scope)); got != c.want {
			t.Errorf("%v on %s: %q, want %q", tenant.HostKindOf(c.ctx), c.scope, got, c.want)
		}
	}
	if !strings.HasPrefix(PlatformTokenPrefix, "msp_") {
		t.Fatal("platform token prefix")
	}
}
