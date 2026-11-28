//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

const bffSecret = "test-bff-secret-0123456789abcdef0123456789" // gitleaks:allow (test fixture)

// testBaseDomain is the platform domain of the harness: new workspaces at
// <slug>.saas.test, the admin panel at admin.saas.test. Tenant #1 is at
// messora.test (PUBLIC_APP_URL), as an installation from before tenants.
const testBaseDomain = "saas.test"

// capturingMailer records sent messages.
type capturingMailer struct {
	mu   sync.Mutex
	sent []mailer.Message
}

func (m *capturingMailer) Enabled(context.Context) bool { return true }
func (m *capturingMailer) Send(_ context.Context, msg mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}
func (m *capturingMailer) last() mailer.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent[len(m.sent)-1]
}

type harness struct {
	t *testing.T
	// pool is the services' pool (messora_app, tenant-scoped: queries need
	// h.ctx() and row-level security applies); owner sees every tenant;
	// sys is the system pool (outbox relay).
	pool  *pgxpool.Pool
	owner *pgxpool.Pool
	sys   *pgxpool.Pool
	// The tenant this harness calls the API as, and its host.
	tenantID uuid.UUID
	host     string
	srv      *httptest.Server
	svc      *services
	mail     *capturingMailer
	router   routers.Router
	meta     *whatsapptest.Server
	rdb      *goredis.Client
	// platTok is a platform admin session, created on first use (shared
	// by the harness copies of h.tenant).
	platTok *string
}

const (
	testAppSecret = "meta-app-secret-for-tests" // gitleaks:allow (test fixture)
	testAppID     = whatsapptest.DefaultAppID
)

func newHarness(t *testing.T) *harness {
	t.Helper()
	tdb := testutil.TenantDB(t)
	pool := tdb.App
	rdb := testutil.Redis(t)
	meta := whatsapptest.New()
	t.Cleanup(meta.Close)
	cfg := &config.Config{PublicAppURL: "https://messora.test", TenantBaseDomain: testBaseDomain, EncryptionKey: bytes.Repeat([]byte{7}, 32),
		Storage: config.StorageConfig{Driver: config.StorageLocal, LocalDir: t.TempDir(), SigningKey: bytes.Repeat([]byte{9}, 32)},
		Meta:    config.MetaConfig{GraphBaseURL: meta.URL, GraphVersion: "v26.0", WebhookBaseURL: "https://hooks.messora.test"}}
	cfg.Centrifugo.HMACSecret = testHMACSecret // see realtime_integration_test.go
	// Tenant A is tenant #1, reached at the PUBLIC_APP_URL host.
	if _, err := tenant.BootstrapDomains(context.Background(), tdb.Owner, cfg.PublicAppURL, ""); err != nil {
		t.Fatal(err)
	}
	// Requests that do not go through h.do (Meta's webhooks, raw
	// downloads) reach the test server at 127.0.0.1: tenant A too.
	if _, err := tdb.Owner.Exec(context.Background(), `INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at)
		VALUES ($1, '127.0.0.1', 'custom', now())`, tenant.DefaultID); err != nil {
		t.Fatal(err)
	}
	mail := &capturingMailer{}
	fast := crypto.Argon2Params{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}
	svc, err := newServicesWith(cfg, pool, rdb, testutil.DiscardLogger(), mail, fast)
	if err != nil {
		t.Fatal(err)
	}
	cf, _ := centrifugo.New(config.CentrifugoConfig{APIURL: "http://127.0.0.1:1/api", APIKey: "x"})
	sys := newSystemHandler(pool, rdb, queue.NewInspector(rdb), cf, testutil.DiscardLogger())

	r := chi.NewRouter()
	r.NotFound(httpx.NotFound)
	r.Use(httpx.RequestID, httpx.Recover(testutil.DiscardLogger()), httpx.IdentifyBFF(bffSecret))
	if err := mountAPI(r, svc, pool, sys, rdb, testutil.DiscardLogger()); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	spec, _ := api.GetSpec()
	spec.Servers = nil // match paths relative to the test server
	for p, item := range spec.Paths.Map() {
		spec.Paths.Delete(p)
		spec.Paths.Set("/api/v1"+p, item)
	}
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	return &harness{t: t, pool: pool, owner: tdb.Owner, sys: tdb.System, tenantID: tenant.DefaultID, host: "messora.test",
		srv: srv, svc: svc, mail: mail, router: router, meta: meta, rdb: rdb, platTok: new(string)}
}

// ctx is a context of the harness's tenant, for calling services and
// h.pool directly.
func (h *harness) ctx() context.Context { return tenant.WithID(context.Background(), h.tenantID) }

// wire is the Centrifugo name of the harness tenant's logical channel.
func (h *harness) wire(logical string) string { return realtime.TenantChannel(h.tenantID, logical) }

// tenant creates another tenant reached at <slug>.messora.test and returns
// a harness that calls the same server as that tenant.
func (h *harness) tenant(slug string) *harness {
	h.t.Helper()
	var id uuid.UUID
	if err := h.owner.QueryRow(context.Background(), `INSERT INTO tenants (slug, name) VALUES ($1, $1) RETURNING id`, slug).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	host := slug + ".messora.test"
	if _, err := h.owner.Exec(context.Background(), `INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary)
		VALUES ($1, $2, 'subdomain', now(), true)`, id, host); err != nil {
		h.t.Fatal(err)
	}
	other := *h
	other.tenantID, other.host = id, host
	return &other
}

type call struct {
	// host overrides the harness's tenant host.
	host         string
	method, path string
	body         any
	token        string // bearer token (session or API key)
	noBFF        bool   // simulate a request from outside the internal network
	headers      map[string]string
}

type resp struct {
	status int
	header http.Header
	raw    []byte
	body   map[string]any
}

func (r resp) data() map[string]any {
	d, _ := r.body["data"].(map[string]any)
	return d
}
func (r resp) list() []any {
	d, _ := r.body["data"].([]any)
	return d
}
func (r resp) errCode() string {
	e, _ := r.body["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}

// do sends the request and validates the response against the OpenAPI spec.
func (h *harness) do(c call) resp {
	h.t.Helper()
	var body io.Reader
	if c.body != nil {
		b, _ := json.Marshal(c.body)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(c.method, h.srv.URL+c.path, body)
	if c.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	host := h.host
	if c.host != "" {
		host = c.host
	}
	// As through Caddy: the browser's Host, which the panel forwards in
	// the tenant host header.
	req.Host = host
	if !c.noBFF {
		req.Header.Set(httpx.BFFHeader, bffSecret)
		req.Header.Set(tenant.HostHeader, host)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: raw}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out.body); err != nil {
			h.t.Fatalf("%s %s: non-JSON response %d: %s", c.method, c.path, res.StatusCode, raw)
		}
	}
	h.validate(req, out)
	return out
}

// validate checks the response against the operation's documented schema.
func (h *harness) validate(req *http.Request, r resp) {
	h.t.Helper()
	vreq := req.Clone(context.Background())
	vreq.URL.Host, vreq.URL.Scheme, vreq.Host = "", "", ""
	route, params, err := h.router.FindRoute(vreq)
	if err != nil {
		return // unknown route: nothing documented to validate against
	}
	err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: vreq, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}},
		Status: r.status, Header: r.header, Body: io.NopCloser(bytes.NewReader(r.raw)),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	})
	if err != nil {
		h.t.Errorf("%s %s → %d violates the OpenAPI contract: %v\nbody: %s", req.Method, req.URL.Path, r.status, err, r.raw)
	}
}

func (h *harness) expect(r resp, status int, code string) {
	h.t.Helper()
	if r.status != status || (code != "" && r.errCode() != code) {
		h.t.Fatalf("want %d %s, got %d %s: %s", status, code, r.status, r.errCode(), r.raw)
	}
}

const ownerPassword = "Horse-battery-staple-42" // gitleaks:allow (test fixture)

// ownerSession makes email the owner of the harness's tenant the way a
// platform admin does (an owner invitation, accepted through the API) and
// returns the new session.
func (h *harness) ownerSession(email, name string) string {
	h.t.Helper()
	inv, err := h.svc.identity.InviteOwner(context.Background(), h.tenantID, email)
	if err != nil {
		h.t.Fatalf("invite owner: %v", err)
	}
	token := inv.AcceptUrl[strings.Index(inv.AcceptUrl, "token=")+6:]
	r := h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": token, "name": name, "password": ownerPassword}})
	h.expect(r, 201, "")
	return r.data()["session_token"].(string)
}

// enrolTOTP turns on two-factor authentication for the session's user.
func (h *harness) enrolTOTP(token string) string {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/me/mfa/totp", token: token})
	h.expect(r, 200, "")
	secret := r.data()["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	r = h.do(call{method: "POST", path: "/api/v1/me/mfa/totp/confirm", token: token, body: map[string]any{"code": code}})
	h.expect(r, 200, "")
	return secret
}

// setupOwner creates the tenant's owner and enrols TOTP.
func (h *harness) setupOwner() (token, totpSecret string) {
	h.t.Helper()
	token = h.ownerSession("Owner@Example.com", "Ayşe Owner")
	return token, h.enrolTOTP(token)
}

func TestOwnerInvitationFlow(t *testing.T) {
	h := newHarness(t)
	token, _ := h.setupOwner()
	r := h.do(call{method: "GET", path: "/api/v1/me", token: token})
	h.expect(r, 200, "")
	user := r.data()["user"].(map[string]any)
	if user["email"] != "owner@example.com" || user["role"].(map[string]any)["key"] != "owner" || user["mfa_enabled"] != true {
		t.Fatalf("unexpected me: %s", r.raw)
	}
	if r.data()["mfa_required"] != false || r.data()["tenant"].(map[string]any)["id"] != h.tenantID.String() {
		t.Fatalf("me: %s", r.raw)
	}
	// The setup wizard's endpoints are gone.
	if code, _ := h.rawAt(h.host, "GET", "/api/v1/setup/status", "", true); code != 404 {
		t.Fatalf("setup status still served: %d", code)
	}
}

func TestBoundaryAndValidation(t *testing.T) {
	h := newHarness(t)
	token, _ := h.setupOwner()

	// Session tokens are worthless outside the internal network.
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: token, noBFF: true}), 401, "UNAUTHORIZED")
	// Panel-only operations reject external callers.
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login", noBFF: true, body: map[string]any{"email": "a@b.co", "password": "x"}}), 403, "FORBIDDEN")
	// Missing and invalid credentials.
	h.expect(h.do(call{method: "GET", path: "/api/v1/me"}), 401, "UNAUTHORIZED")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: "mss_bogus"}), 401, "SESSION_INVALID")

	// Schema validation with field details.
	r := h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "not-an-email", "password": "x", "extra": true}})
	h.expect(r, 422, "VALIDATION_ERROR")
	details := r.body["error"].(map[string]any)["details"].([]any)
	if len(details) < 2 {
		t.Fatalf("expected one detail per problem: %s", r.raw)
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/not-a-uuid", token: token, body: map[string]any{"status": "disabled"}}), 422, "VALIDATION_ERROR")

	// Unknown routes keep the error envelope.
	h.expect(h.do(call{method: "GET", path: "/api/v1/nope", token: token}), 404, "NOT_FOUND")
}

func TestLoginLockoutAndMFA(t *testing.T) {
	h := newHarness(t)
	_, secret := h.setupOwner()

	// Wrong password is a generic error; unknown e-mail looks identical.
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": "wrong"}}), 401, "INVALID_CREDENTIALS")
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "nobody@example.com", "password": "wrong"}}), 401, "INVALID_CREDENTIALS")

	// Correct password → second factor required.
	r := h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "OWNER@example.com", "password": ownerPassword}})
	h.expect(r, 200, "")
	if r.data()["status"] != "mfa_required" || r.data()["session"] != nil {
		t.Fatalf("expected MFA challenge without a session: %s", r.raw)
	}
	mfa := r.data()["mfa_token"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": mfa, "code": "000000"}}), 401, "MFA_INVALID")

	code, _ := totp.GenerateCode(secret, time.Now().Add(30*time.Second)) // next step: not used during enrolment
	r = h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": mfa, "code": code}})
	h.expect(r, 200, "")
	session := r.data()["session_token"].(string)
	// The challenge is single use.
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": mfa, "code": code}}), 401, "MFA_EXPIRED")

	// A TOTP code cannot be replayed on another challenge.
	r = h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": ownerPassword}})
	mfa2 := r.data()["mfa_token"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": mfa2, "code": code}}), 401, "MFA_INVALID")

	// Recovery codes work once.
	r = h.do(call{method: "POST", path: "/api/v1/auth/reauth", token: session, body: map[string]any{"password": ownerPassword}})
	h.expect(r, 200, "")
	r = h.do(call{method: "POST", path: "/api/v1/me/mfa/recovery-codes", token: session})
	h.expect(r, 200, "")
	rc := r.data()["codes"].([]any)[0].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": mfa2, "recovery_code": rc}}), 200, "")
	r = h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": ownerPassword}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": r.data()["mfa_token"], "recovery_code": rc}}), 401, "MFA_INVALID")

	// Owners cannot switch 2FA off while the policy requires it.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: session, body: map[string]any{"mfa_required_roles": []string{"owner"}}}), 200, "")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/me/mfa/totp", token: session}), 403, "MFA_REQUIRED_BY_POLICY")

	// Ten consecutive failures lock the account, even for the right password.
	for range 10 {
		h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": "wrong-again"},
			headers: map[string]string{"X-Forwarded-For": "203.0.113.9"}})
	}
	r = h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": ownerPassword}})
	if r.status != 429 || (r.errCode() != "ACCOUNT_LOCKED" && r.errCode() != "RATE_LIMITED") || r.header.Get("Retry-After") == "" {
		t.Fatalf("expected lockout with Retry-After, got %d %s", r.status, r.raw)
	}
}

func TestMandatoryMFABlocksEverythingElse(t *testing.T) {
	h := newHarness(t)
	// The policy comes from the environment here, which also locks it.
	if err := h.svc.settings.SetOverrides(map[string]string{"mfa_required_roles": "owner,admin"}); err != nil {
		t.Fatal(err)
	}
	token := h.ownerSession("o@example.com", "O")

	r := h.do(call{method: "GET", path: "/api/v1/me", token: token})
	h.expect(r, 200, "")
	if r.data()["mfa_required"] != true {
		t.Fatal("owner without 2FA must be flagged")
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/users", token: token}), 403, "MFA_SETUP_REQUIRED")
	h.expect(h.do(call{method: "GET", path: "/api/v1/settings", token: token}), 403, "MFA_SETUP_REQUIRED")
}

func TestInvitationsRolesAndLastOwner(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()

	// Invite an agent; the link is returned and e-mailed.
	r := h.do(call{method: "POST", path: "/api/v1/invitations", token: owner, body: map[string]any{"email": "agent@example.com", "role": "agent"}})
	h.expect(r, 201, "")
	link := r.data()["accept_url"].(string)
	if r.data()["email_sent"] != true || !strings.Contains(h.mail.last().Body, link) {
		t.Fatal("invitation e-mail missing the link")
	}
	invToken := link[strings.Index(link, "token=")+6:]
	h.expect(h.do(call{method: "POST", path: "/api/v1/invitations", token: owner, body: map[string]any{"email": "agent@example.com", "role": "agent"}}), 409, "INVITATION_EXISTS")
	h.expect(h.do(call{method: "POST", path: "/api/v1/invitations", token: owner, body: map[string]any{"email": "owner@example.com", "role": "agent"}}), 409, "USER_EXISTS")

	r = h.do(call{method: "POST", path: "/api/v1/invitations/lookup", body: map[string]any{"token": invToken}})
	h.expect(r, 200, "")
	if r.data()["email"] != "agent@example.com" {
		t.Fatal(string(r.raw))
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": invToken, "name": "Agent", "password": "password1234"}}), 422, "VALIDATION_ERROR")
	r = h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": invToken, "name": "Ali Agent", "password": "Correct-horse-staple-9"}}) // gitleaks:allow (test fixture)
	h.expect(r, 201, "")
	agent := r.data()["session_token"].(string)
	agentID := r.data()["user"].(map[string]any)["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": invToken, "name": "Again", "password": "Correct-horse-staple-9"}}), 403, "INVITATION_INVALID") // gitleaks:allow (test fixture)

	// Agents lack users.manage.
	r = h.do(call{method: "GET", path: "/api/v1/users", token: agent})
	h.expect(r, 403, "FORBIDDEN")
	if !strings.Contains(r.raw2(), "users.manage") {
		t.Fatalf("error must name the missing permission: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: agent}), 200, "")

	// Promote to admin; an admin cannot create owners or touch the owner.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/" + agentID, token: owner, body: map[string]any{"role": "admin"}}), 200, "")
	r = h.do(call{method: "GET", path: "/api/v1/users", token: owner})
	ownerID := ""
	for _, u := range r.list() {
		if m := u.(map[string]any); m["email"] == "owner@example.com" {
			ownerID = m["id"].(string)
		}
	}
	// Once the policy requires it, the admin must enrol 2FA before using
	// admin features.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"mfa_required_roles": []string{"owner", "admin"}}}), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/users", token: agent}), 403, "MFA_SETUP_REQUIRED")
	h.svc.settings.Invalidate()
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"mfa_required_roles": []string{"owner"}}}), 200, "")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/" + ownerID, token: agent, body: map[string]any{"role": "agent"}}), 403, "INSUFFICIENT_ROLE")
	h.expect(h.do(call{method: "POST", path: "/api/v1/invitations", token: agent, body: map[string]any{"email": "new@example.com", "role": "owner"}}), 403, "INSUFFICIENT_ROLE")

	// Owners cannot change themselves; the last owner cannot be demoted.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/" + ownerID, token: owner, body: map[string]any{"role": "admin"}}), 403, "SELF_MODIFICATION")

	// Disabling a user ends their sessions immediately.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/" + agentID, token: owner, body: map[string]any{"status": "disabled"}}), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: agent}), 401, "SESSION_INVALID")
}

func (r resp) raw2() string { return string(r.raw) }

func TestAPIKeysAndIdempotency(t *testing.T) {
	h := newHarness(t)
	owner, secret := h.setupOwner()

	body := map[string]any{"name": "CRM sync", "scopes": []string{"contacts.read"}}
	// Signing in counts as fresh authentication for 10 minutes; age it out.
	if _, err := h.pool.Exec(h.ctx(), "UPDATE user_sessions SET reauth_at = now() - interval '11 minutes'"); err != nil {
		t.Fatal(err)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: body}), 403, "REAUTH_REQUIRED")
	code, _ := totp.GenerateCode(secret, time.Now().Add(30*time.Second))
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/reauth", token: owner, body: map[string]any{"code": code}}), 200, "")

	idem := map[string]string{"Idempotency-Key": "create-key-0001"}
	r1 := h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: body, headers: idem})
	h.expect(r1, 201, "")
	r2 := h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: body, headers: idem})
	h.expect(r2, 201, "")
	if r2.header.Get("Idempotent-Replayed") != "true" || !bytes.Equal(r1.raw, r2.raw) {
		t.Fatal("retry with the same Idempotency-Key must replay the response")
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: map[string]any{"name": "other", "scopes": []string{"contacts.read"}}, headers: idem}), 409, "IDEMPOTENCY_KEY_REUSED")
	r := h.do(call{method: "GET", path: "/api/v1/api-keys", token: owner})
	if len(r.list()) != 1 {
		t.Fatalf("replay must not create a second key: %s", r.raw)
	}

	key := r1.data()["secret"].(string)
	keyID := r1.data()["api_key"].(map[string]any)["id"].(string)
	// API keys work from outside the internal network…
	r = h.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: key, noBFF: true})
	h.expect(r, 200, "")
	if r.data()["name"] != "CRM sync" {
		t.Fatal(string(r.raw))
	}
	// …but never on session-only operations.
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: key, noBFF: true}), 403, "FORBIDDEN")
	h.expect(h.do(call{method: "GET", path: "/api/v1/users", token: key}), 403, "FORBIDDEN")

	h.expect(h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: map[string]any{"name": "x", "scopes": []string{"nope.nope"}}}), 422, "VALIDATION_ERROR")

	h.expect(h.do(call{method: "DELETE", path: "/api/v1/api-keys/" + keyID, token: owner}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: key, noBFF: true}), 401, "API_KEY_INVALID")
}

func TestPasswordChangeResetAndSessions(t *testing.T) {
	h := newHarness(t)
	owner, secret := h.setupOwner()

	// A second session (e.g. another browser).
	r := h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": ownerPassword}})
	code, _ := totp.GenerateCode(secret, time.Now().Add(30*time.Second))
	r = h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": r.data()["mfa_token"], "code": code}})
	other := r.data()["session_token"].(string)

	r = h.do(call{method: "GET", path: "/api/v1/me/sessions", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 2 {
		t.Fatalf("want 2 sessions: %s", r.raw)
	}

	h.expect(h.do(call{method: "POST", path: "/api/v1/me/password", token: owner, body: map[string]any{"current_password": "wrong", "new_password": "Brand-new-password-77"}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/me/password", token: owner, body: map[string]any{"current_password": ownerPassword, "new_password": "Brand-new-password-77"}}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: other}), 401, "SESSION_INVALID")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: owner}), 200, "")

	// Password reset: unknown address is indistinguishable.
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]any{"email": "nobody@example.com"}}), 202, "")
	before := len(h.mail.sent)
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]any{"email": "owner@example.com"}}), 202, "")
	if len(h.mail.sent) != before+1 {
		t.Fatal("reset e-mail not sent")
	}
	body := h.mail.last().Body
	resetTok := body[strings.Index(body, "token=")+6:]
	resetTok = strings.Fields(resetTok)[0]
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/password/reset", body: map[string]any{"token": resetTok, "password": "Reset-password-value-1"}}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: owner}), 401, "SESSION_INVALID")
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/password/reset", body: map[string]any{"token": resetTok, "password": "Another-password-value-2"}}), 403, "RESET_TOKEN_INVALID")
	r = h.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": "Reset-password-value-1"}})
	h.expect(r, 200, "")
}

func TestAuditTrailAndPagination(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	for i := range 3 {
		h.do(call{method: "POST", path: "/api/v1/invitations", token: owner, body: map[string]any{"email": "p" + string(rune('a'+i)) + "@example.com", "role": "agent"}})
	}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		path := "/api/v1/audit-logs?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := h.do(call{method: "GET", path: path, token: owner})
		h.expect(r, 200, "")
		for _, e := range r.list() {
			seen[e.(map[string]any)["action"].(string)] = true
		}
		pages++
		next, _ := r.body["meta"].(map[string]any)["next_cursor"].(string)
		if next == "" || pages > 20 {
			break
		}
		cursor = next
	}
	for _, a := range []string{"invitation.accept", "user.login", "user.mfa_enable", "invitation.create"} {
		if !seen[a] {
			t.Errorf("audit log missing %s (saw %v)", a, seen)
		}
	}
	if pages < 3 {
		t.Fatalf("expected several pages, got %d", pages)
	}
	// The owner's own invitation (from the platform) and three more.
	r := h.do(call{method: "GET", path: "/api/v1/audit-logs?action=invitation.create", token: owner})
	if len(r.list()) != 4 {
		t.Fatalf("filter by action: %s", r.raw)
	}
}

func TestSystemStatusRequiresPermission(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	r := h.do(call{method: "GET", path: "/api/v1/system/status", token: owner})
	h.expect(r, 200, "")
	comps := r.data()["components"].(map[string]any)
	if comps["postgres"].(map[string]any)["status"] != "ok" || comps["centrifugo"].(map[string]any)["status"] != "fail" {
		t.Fatalf("unexpected component states: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/system/status"}), 401, "UNAUTHORIZED")
}
