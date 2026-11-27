//go:build integration

package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Phase 17 WP2: global accounts, self sign-up, e-mail verification,
// workspace hand-over, the platform admin panel and support access.

const (
	platformHost = "app." + testBaseDomain
	adminHost    = "admin." + testBaseDomain
)

var clientSeq atomic.Int32

// fromNewIP gives a call its own client address (the BFF's
// X-Forwarded-For), so per-IP sign-up limits do not interfere.
func fromNewIP(c call) call {
	if c.headers == nil {
		c.headers = map[string]string{}
	}
	c.headers["X-Forwarded-For"] = fmt.Sprintf("203.0.113.%d", clientSeq.Add(1)%250+1)
	return c
}

var tokenInLink = regexp.MustCompile(`token=([A-Za-z0-9_\-]+)`)

// mailTo returns the last e-mail to addr and the token of its first link.
func (h *harness) mailTo(addr string) (mailer.Message, string) {
	h.t.Helper()
	h.mail.mu.Lock()
	defer h.mail.mu.Unlock()
	for i := len(h.mail.sent) - 1; i >= 0; i-- {
		if m := h.mail.sent[i]; strings.EqualFold(m.To, addr) {
			tok := ""
			if sm := tokenInLink.FindStringSubmatch(m.Body); sm != nil {
				tok = sm[1]
			}
			return m, tok
		}
	}
	h.t.Fatalf("no e-mail to %s", addr)
	return mailer.Message{}, ""
}

func (h *harness) countMailTo(addr, subjectPart string) int {
	h.mail.mu.Lock()
	defer h.mail.mu.Unlock()
	n := 0
	for _, m := range h.mail.sent {
		if strings.EqualFold(m.To, addr) && strings.Contains(m.Subject, subjectPart) {
			n++
		}
	}
	return n
}

// signup opens a workspace through the API and returns its id.
func (h *harness) signup(slug, email string) string {
	h.t.Helper()
	r := h.do(fromNewIP(call{host: platformHost, method: "POST", path: "/api/v1/auth/signup", body: map[string]any{
		"name": "Deniz Kurucu", "email": email, "password": ownerPassword, "company_name": "Firma " + slug,
		"workspace_slug": slug, "locale": "tr", "accept_terms": true}}))
	h.expect(r, 201, "")
	return r.data()["tenant"].(map[string]any)["id"].(string)
}

// verify follows the verification link of the last sign-up e-mail to addr.
func (h *harness) verify(addr string) resp {
	h.t.Helper()
	_, tok := h.mailTo(addr)
	return h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": tok}})
}

// at returns a harness that calls the API at host as the tenant id.
func (h *harness) at(host string, id uuid.UUID) *harness {
	other := *h
	other.host, other.tenantID = host, id
	return &other
}

func (h *harness) login(host, email string) resp {
	h.t.Helper()
	return h.do(fromNewIP(call{host: host, method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": email, "password": ownerPassword}}))
}

// platformAdmin creates a platform admin, signs in on the admin host and
// enrols TOTP; it returns the session.
func (h *harness) platformAdmin(email string) string {
	h.t.Helper()
	if _, _, err := h.svc.identity.CreatePlatformAdmin(context.Background(), email, "Platform Admin", ownerPassword); err != nil {
		h.t.Fatal(err)
	}
	r := h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/login", body: map[string]any{"email": email, "password": ownerPassword}})
	h.expect(r, 200, "")
	token := r.data()["session"].(map[string]any)["session_token"].(string)
	r = h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/me/mfa/totp", token: token})
	h.expect(r, 200, "")
	code, _ := totp.GenerateCode(r.data()["secret"].(string), time.Now())
	h.expect(h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/me/mfa/totp/confirm", token: token, body: map[string]any{"code": code}}), 200, "")
	return token
}

func (h *harness) admin(token, method, path string, body any) resp {
	h.t.Helper()
	return h.do(call{host: adminHost, method: method, path: "/api/v1/platform" + path, token: token, body: body})
}

func TestSignupVerifyAndLogin(t *testing.T) {
	h := newHarness(t)

	r := h.do(call{host: platformHost, method: "GET", path: "/api/v1/auth/capabilities"})
	h.expect(r, 200, "")
	if r.data()["signup_enabled"] != true {
		t.Fatalf("capabilities %s", r.raw)
	}
	r = h.do(call{host: platformHost, method: "GET", path: "/api/v1/auth/signup/slug-availability?slug=acme"})
	h.expect(r, 200, "")
	if r.data()["available"] != true {
		t.Fatalf("free slug: %s", r.raw)
	}

	id := h.signup("acme", "Kurucu@Example.com")
	host := "acme." + testBaseDomain
	tid := uuid.MustParse(id)

	// The tenant, its sub-domain, the trial and the owner exist; the e-mail
	// link points at the new workspace's address.
	var status, sub string
	var trialEnds *time.Time
	if err := h.owner.QueryRow(context.Background(), `SELECT t.status, t.subscription_status, t.trial_ends_at FROM tenants t WHERE t.id = $1`, tid).Scan(&status, &sub, &trialEnds); err != nil {
		t.Fatal(err)
	}
	if status != "active" || sub != "trial" || trialEnds == nil || time.Until(*trialEnds) < 13*24*time.Hour || time.Until(*trialEnds) > 15*24*time.Hour {
		t.Fatalf("tenant %s %s %v", status, sub, trialEnds)
	}
	msg, tok := h.mailTo("kurucu@example.com")
	if !strings.Contains(msg.Body, "https://"+host+"/verify-email?token=msv_") || msg.HTML == "" || !strings.HasPrefix(tok, "msv_") {
		t.Fatalf("verification e-mail: %q", msg.Body)
	}
	if !strings.Contains(msg.Subject, "doğrulayın") {
		t.Fatalf("verification e-mail is not in the sign-up language: %q", msg.Subject)
	}

	// Taken, reserved and malformed addresses; a second account for the
	// same e-mail address.
	r = h.do(call{host: platformHost, method: "GET", path: "/api/v1/auth/signup/slug-availability?slug=acme"})
	if r.data()["available"] != false || r.data()["reason"] != "taken" {
		t.Fatalf("taken slug: %s", r.raw)
	}
	r = h.do(call{host: platformHost, method: "GET", path: "/api/v1/auth/signup/slug-availability?slug=admin"})
	if r.data()["available"] != false || r.data()["reason"] != "reserved" {
		t.Fatalf("reserved slug: %s", r.raw)
	}
	h.expect(h.do(call{host: platformHost, method: "GET", path: "/api/v1/auth/signup/slug-availability?slug=a--b"}), 422, "VALIDATION_ERROR")
	body := func(slug, email string) map[string]any {
		return map[string]any{"name": "X", "email": email, "password": ownerPassword, "company_name": "X", "workspace_slug": slug, "locale": "en", "accept_terms": true}
	}
	h.expect(h.do(fromNewIP(call{host: platformHost, method: "POST", path: "/api/v1/auth/signup", body: body("acme", "other@example.com")})), 409, "SLUG_TAKEN")
	h.expect(h.do(fromNewIP(call{host: platformHost, method: "POST", path: "/api/v1/auth/signup", body: body("support", "other@example.com")})), 422, "SLUG_RESERVED")
	h.expect(h.do(fromNewIP(call{host: platformHost, method: "POST", path: "/api/v1/auth/signup", body: body("-bad-", "other@example.com")})), 422, "VALIDATION_ERROR")
	h.expect(h.do(fromNewIP(call{host: platformHost, method: "POST", path: "/api/v1/auth/signup", body: body("acme-two", "KURUCU@example.com")})), 409, "EMAIL_TAKEN")
	var tenants int
	if err := h.owner.QueryRow(context.Background(), `SELECT count(*) FROM tenants`).Scan(&tenants); err != nil || tenants != 2 {
		t.Fatalf("failed sign-ups left tenants behind: %d %v", tenants, err)
	}

	// Not verified yet: the right password is not enough.
	h.expect(h.login(host, "kurucu@example.com"), 403, "EMAIL_NOT_VERIFIED")

	r = h.verify("kurucu@example.com")
	h.expect(r, 200, "")
	if r.data()["login_url"] != "https://"+host+"/login?verified=1" || r.data()["tenant"].(map[string]any)["slug"] != "acme" {
		t.Fatalf("verify: %s", r.raw)
	}
	if h.countMailTo("kurucu@example.com", "Hoş geldiniz") != 1 {
		t.Fatalf("no welcome e-mail")
	}
	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": tok}}), 409, "EMAIL_ALREADY_VERIFIED")

	r = h.login(host, "kurucu@example.com")
	h.expect(r, 200, "")
	session := r.data()["session"].(map[string]any)["session_token"].(string)
	acme := h.at(host, tid)
	r = acme.do(call{method: "GET", path: "/api/v1/me", token: session})
	h.expect(r, 200, "")
	if r.data()["user"].(map[string]any)["role"].(map[string]any)["key"] != "owner" || r.data()["tenant"].(map[string]any)["slug"] != "acme" {
		t.Fatalf("me: %s", r.raw)
	}
	r = acme.do(call{method: "GET", path: "/api/v1/settings", token: session})
	h.expect(r, 200, "")
	if r.data()["organization_name"] != "Firma acme" || r.data()["default_locale"] != "tr" || r.data()["default_timezone"] != "Europe/Istanbul" {
		t.Fatalf("settings of the new workspace: %s", r.raw)
	}
	// The new workspace sees only itself.
	r = acme.do(call{method: "GET", path: "/api/v1/users", token: session})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("users of the new workspace: %s", r.raw)
	}
	// Sign-up is anonymous only on the platform; the platform panel says
	// nothing to a workspace session.
	h.expect(h.do(call{host: adminHost, method: "GET", path: "/api/v1/platform/tenants", token: session}), 401, "")

	// Closed sign-up.
	admin := h.platformAdmin("ops@messora.test")
	h.expect(h.admin(admin, "PATCH", "/settings", map[string]any{"signup_enabled": false}), 200, "")
	h.expect(h.do(fromNewIP(call{host: platformHost, method: "POST", path: "/api/v1/auth/signup", body: body("closed", "closed@example.com")})), 403, "SIGNUP_DISABLED")
	r = h.do(call{host: platformHost, method: "GET", path: "/api/v1/auth/capabilities"})
	if r.data()["signup_enabled"] != false {
		t.Fatalf("capabilities after closing: %s", r.raw)
	}
}

func TestEmailVerificationErrorsAndResend(t *testing.T) {
	h := newHarness(t)
	h.signup("late", "late@example.com")
	_, first := h.mailTo("late@example.com")

	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": "msv_unknown-unknown-unknown-unknown"}}), 400, "VERIFICATION_TOKEN_INVALID")
	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": "not-a-verification-token"}}), 400, "VERIFICATION_TOKEN_INVALID")

	// Expired link.
	if _, err := h.owner.Exec(context.Background(), `UPDATE email_verification_tokens SET expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": first}}), 410, "VERIFICATION_TOKEN_EXPIRED")

	// A new link replaces the old one; one e-mail per address and minute.
	r := h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/resend-verification", body: map[string]any{"email": "late@example.com"}})
	h.expect(r, 202, "")
	r = h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/resend-verification", body: map[string]any{"email": "late@example.com"}})
	h.expect(r, 429, "RATE_LIMITED")
	if r.header.Get("Retry-After") == "" {
		t.Fatal("429 without Retry-After")
	}
	// Unknown addresses answer the same (no account probing).
	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/resend-verification", body: map[string]any{"email": "nobody@example.com"}}), 202, "")
	if n := h.countMailTo("nobody@example.com", ""); n != 0 {
		t.Fatalf("%d e-mails to an unknown address", n)
	}
	_, second := h.mailTo("late@example.com")
	if second == first {
		t.Fatal("resend reused the old token")
	}
	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": first}}), 400, "VERIFICATION_TOKEN_INVALID")
	h.expect(h.do(call{host: platformHost, method: "POST", path: "/api/v1/auth/verify-email", body: map[string]any{"token": second}}), 200, "")
}

func TestMembershipsHandoffAndHostBoundSessions(t *testing.T) {
	h := newHarness(t)
	ownerA, _ := h.setupOwner() // tenant #1 at messora.test
	idB := uuid.MustParse(h.signup("beta", "deniz@example.com"))
	h.expect(h.verify("deniz@example.com"), 200, "")
	hostB := "beta." + testBaseDomain
	hb := h.at(hostB, idB)

	// Tenant #1 invites the person who already has an account (from the
	// sign-up of beta): accepting asks for that account's password.
	r := h.do(call{method: "POST", path: "/api/v1/invitations", token: ownerA, body: map[string]any{"email": "deniz@example.com", "role": "manager"}})
	h.expect(r, 201, "")
	inv := r.data()["accept_url"].(string)
	invTok := inv[strings.Index(inv, "token=")+6:]
	r = h.do(call{method: "POST", path: "/api/v1/invitations/lookup", body: map[string]any{"token": invTok}})
	h.expect(r, 200, "")
	if r.data()["account_exists"] != true || r.data()["invited_by"].(map[string]any)["name"] != "Ayşe Owner" || r.data()["tenant"] == nil {
		t.Fatalf("lookup: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": invTok, "password": "wrong-password-123"}}), 401, "INVALID_CREDENTIALS")
	r = h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": invTok, "password": ownerPassword}})
	h.expect(r, 201, "")
	sessA := r.data()["session_token"].(string)
	var accounts int
	if err := h.owner.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = 'deniz@example.com'`).Scan(&accounts); err != nil || accounts != 1 {
		t.Fatalf("%d accounts for one person (%v)", accounts, err)
	}

	// Two memberships, a role in each; the session's own one is current.
	r = h.do(call{method: "GET", path: "/api/v1/me/memberships", token: sessA})
	h.expect(r, 200, "")
	if len(r.list()) != 2 {
		t.Fatalf("memberships: %s", r.raw)
	}
	roles := map[string]string{}
	for _, it := range r.list() {
		m := it.(map[string]any)
		ten := m["tenant"].(map[string]any)
		roles[ten["slug"].(string)] = m["role"].(map[string]any)["key"].(string)
		if (ten["id"] == h.tenantID.String()) != (m["current"] == true) {
			t.Fatalf("current flag: %s", r.raw)
		}
		if ten["slug"] == "beta" && m["url"] != "https://"+hostB {
			t.Fatalf("workspace url: %s", r.raw)
		}
	}
	if roles["beta"] != "owner" || roles["default"] != "manager" {
		t.Fatalf("roles %v", roles)
	}

	// A session is valid at its own workspace's address only.
	if code, _ := h.rawAt(hostB, "GET", "/api/v1/me", sessA, true); code != 401 {
		t.Fatalf("tenant #1 session at beta: %d", code)
	}

	// Switching: a one-time token, short-lived, for the target host only.
	switchTo := func() string {
		r := h.do(call{method: "POST", path: "/api/v1/auth/switch-workspace", token: sessA, body: map[string]any{"tenant_id": idB.String()}})
		h.expect(r, 200, "")
		u, err := url.Parse(r.data()["redirect_url"].(string))
		if err != nil || u.Host != hostB || u.Path != "/auth/handoff" {
			t.Fatalf("redirect %v %v", u, err)
		}
		return u.Query().Get("token")
	}
	tok := switchTo()
	if ttl := h.rdb.TTL(context.Background(), "messora:handoff:"+fmt.Sprintf("%x", sha(tok))).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("hand-over token TTL %v", ttl)
	}
	// Presented at the wrong workspace it fails and is gone.
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": tok}}), 401, "HANDOFF_INVALID")
	h.expect(hb.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": tok}}), 401, "HANDOFF_INVALID")
	tok = switchTo()
	r = hb.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": tok}})
	h.expect(r, 200, "")
	sessB := r.data()["session_token"].(string)
	h.expect(hb.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": tok}}), 401, "HANDOFF_INVALID")
	r = hb.do(call{method: "GET", path: "/api/v1/me", token: sessB})
	h.expect(r, 200, "")
	if r.data()["user"].(map[string]any)["role"].(map[string]any)["key"] != "owner" {
		t.Fatalf("me at beta: %s", r.raw)
	}
	if code, _ := h.rawAt(h.host, "GET", "/api/v1/me", sessB, true); code != 401 {
		t.Fatalf("beta session at tenant #1: %d", code)
	}
	// No membership, no hand-over.
	other := uuid.MustParse(h.signup("gamma", "gamma@example.com"))
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/switch-workspace", token: sessA, body: map[string]any{"tenant_id": other.String()}}), 403, "FORBIDDEN")

	// A disabled membership closes that workspace only.
	r = h.do(call{method: "GET", path: "/api/v1/users", token: ownerA})
	var denizID string
	for _, it := range r.list() {
		if u := it.(map[string]any); u["email"] == "deniz@example.com" {
			denizID = u["id"].(string)
		}
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/" + denizID, token: ownerA, body: map[string]any{"status": "disabled"}}), 200, "")
	if code, _ := h.rawAt(h.host, "GET", "/api/v1/me", sessA, true); code != 401 {
		t.Fatalf("disabled member still signed in at tenant #1: %d", code)
	}
	h.expect(hb.do(call{method: "GET", path: "/api/v1/me", token: sessB}), 200, "")

	// A password change ends the sessions in every workspace but the current one.
	r = h.login(hostB, "deniz@example.com")
	h.expect(r, 200, "")
	sessB2 := r.data()["session"].(map[string]any)["session_token"].(string)
	h.expect(hb.do(call{method: "POST", path: "/api/v1/me/password", token: sessB2, body: map[string]any{"current_password": ownerPassword, "new_password": "Another-long-passphrase-9"}}), 204, "")
	h.expect(hb.do(call{method: "GET", path: "/api/v1/me", token: sessB}), 401, "")
	h.expect(hb.do(call{method: "GET", path: "/api/v1/me", token: sessB2}), 200, "")
	if h.countMailTo("deniz@example.com", "") < 3 {
		t.Fatal("expected verification, invitation and password-changed e-mails")
	}
}

func sha(s string) []byte { return crypto.HashToken(s) }

// platformSession returns the harness's platform admin session (created on
// first use).
func (h *harness) platformSession() string {
	h.t.Helper()
	if *h.platTok == "" {
		*h.platTok = h.platformAdmin("platform-admin@messora.test")
	}
	return *h.platTok
}

// adminDo calls a platform operation (path below /api/v1/platform) as the
// harness's platform admin.
func (h *harness) adminDo(c call) resp {
	h.t.Helper()
	c.host, c.token, c.path = adminHost, h.platformSession(), "/api/v1/platform"+c.path
	return h.do(c)
}

// multipartAt posts a file as multipart/form-data to host with token.
func (h *harness) multipartAt(host, token, path, filename string, content []byte, fields ...string) (int, []byte) {
	h.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i := 0; i+1 < len(fields); i += 2 {
		_ = mw.WriteField(fields[i], fields[i+1])
	}
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+path, &buf)
	req.Host = host
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set(tenant.HostHeader, host)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

func TestPlatformAdminAuthAndScopes(t *testing.T) {
	h := newHarness(t)
	ownerA, _ := h.setupOwner()

	// Bootstrap: the first admin from the one-time link of `messora migrate`.
	r := h.do(call{host: adminHost, method: "GET", path: "/api/v1/platform/auth/capabilities"})
	h.expect(r, 200, "")
	if r.data()["bootstrap_required"] != true {
		t.Fatalf("capabilities %s", r.raw)
	}
	tok, err := identity.IssueBootstrapToken(context.Background(), db.New(h.owner), time.Now())
	if err != nil || tok == "" {
		t.Fatalf("bootstrap token %q %v", tok, err)
	}
	h.expect(h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/bootstrap", body: map[string]any{
		"token": "msb_wrong-wrong-wrong-wrong-wrong", "name": "Root", "email": "root@messora.test", "password": ownerPassword}}), 403, "BOOTSTRAP_TOKEN_INVALID")
	r = h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/bootstrap", body: map[string]any{
		"token": tok, "name": "Root", "email": "root@messora.test", "password": ownerPassword}})
	h.expect(r, 201, "")
	root := r.data()["session_token"].(string)
	if !strings.HasPrefix(root, "msp_") {
		t.Fatalf("platform token %q", root)
	}
	h.expect(h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/bootstrap", body: map[string]any{
		"token": tok, "name": "Again", "email": "again@messora.test", "password": ownerPassword}}), 409, "BOOTSTRAP_COMPLETE")

	// Two-factor authentication first.
	h.expect(h.admin(root, "GET", "/tenants", nil), 403, "MFA_SETUP_REQUIRED")
	r = h.admin(root, "GET", "/me", nil)
	h.expect(r, 200, "")
	if r.data()["mfa_required"] != true {
		t.Fatalf("platform me %s", r.raw)
	}
	r = h.admin(root, "POST", "/me/mfa/totp", nil)
	h.expect(r, 200, "")
	secret := r.data()["secret"].(string)
	code, _ := totp.GenerateCode(secret, time.Now())
	h.expect(h.admin(root, "POST", "/me/mfa/totp/confirm", map[string]any{"code": code}), 200, "")
	h.expect(h.admin(root, "GET", "/tenants", nil), 200, "")

	// Next sign-in needs the code; a challenge is valid only on the admin
	// host.
	r = h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/login", body: map[string]any{"email": "root@messora.test", "password": ownerPassword}})
	h.expect(r, 200, "")
	if r.data()["status"] != "mfa_required" {
		t.Fatalf("platform login %s", r.raw)
	}
	mfa := r.data()["mfa_token"].(string)
	code, _ = totp.GenerateCode(secret, time.Now().Add(30*time.Second))
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/login/mfa", body: map[string]any{"mfa_token": mfa, "code": code}}), 401, "MFA_EXPIRED")
	r = h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/login/mfa", body: map[string]any{"mfa_token": mfa, "code": code}})
	h.expect(r, 200, "")

	// Workspace members are not platform admins.
	h.expect(h.do(call{host: adminHost, method: "POST", path: "/api/v1/platform/auth/login", body: map[string]any{"email": "owner@example.com", "password": ownerPassword}}), 401, "INVALID_CREDENTIALS")

	// Scope separation: neither kind of session works on the other's host,
	// and each host serves only its own operations.
	if code, _ := h.rawAt(adminHost, "GET", "/api/v1/platform/tenants", ownerA, true); code != 401 {
		t.Fatalf("workspace session on the admin API: %d", code)
	}
	if code, _ := h.rawAt(h.host, "GET", "/api/v1/me", root, true); code != 401 {
		t.Fatalf("platform session on a workspace: %d", code)
	}
	if code, errCode := h.rawAt(h.host, "GET", "/api/v1/platform/tenants", root, true); code != 404 || errCode != "NOT_FOUND" {
		t.Fatalf("platform API on a workspace host: %d %s", code, errCode)
	}
	if code, errCode := h.rawAt(adminHost, "GET", "/api/v1/contacts", root, true); code != 404 || errCode != "TENANT_NOT_FOUND" {
		t.Fatalf("workspace API on the admin host: %d %s", code, errCode)
	}
	if code, _ := h.rawAt(platformHost, "GET", "/api/v1/platform/tenants", root, true); code != 404 {
		t.Fatalf("platform API on the public host: %d", code)
	}
	// Platform sessions only through the panel.
	if code, _ := h.rawAt(adminHost, "GET", "/api/v1/platform/tenants", root, false); code != 401 && code != 403 {
		t.Fatalf("platform session from outside: %d", code)
	}
	// Logout.
	h.expect(h.admin(root, "POST", "/auth/logout", nil), 204, "")
	h.expect(h.admin(root, "GET", "/me", nil), 401, "SESSION_INVALID")
}

func TestPlatformTenantsDomainsAndSuspension(t *testing.T) {
	h := newHarness(t)
	admin := h.platformAdmin("ops@messora.test")

	r := h.admin(admin, "POST", "/tenants", map[string]any{"name": "Kappa Ltd", "slug": "kappa", "owner_email": "boss@kappa.example", "locale": "en"})
	h.expect(r, 201, "")
	id := r.data()["tenant"].(map[string]any)["id"].(string)
	inv := r.data()["owner_invitation"].(map[string]any)
	if !strings.HasPrefix(inv["accept_url"].(string), "https://kappa."+testBaseDomain+"/invite?token=msi_") || inv["email_sent"] != true {
		t.Fatalf("owner invitation %v", inv)
	}
	h.expect(h.admin(admin, "POST", "/tenants", map[string]any{"name": "Dup", "slug": "kappa"}), 409, "SLUG_TAKEN")
	h.expect(h.admin(admin, "POST", "/tenants", map[string]any{"name": "Res", "slug": "www"}), 422, "SLUG_RESERVED")

	r = h.admin(admin, "GET", "/tenants?q=kap", nil)
	h.expect(r, 200, "")
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["url"] != "https://kappa."+testBaseDomain {
		t.Fatalf("search %s", r.raw)
	}

	// The owner accepts at the workspace's address.
	kappa := h.at("kappa."+testBaseDomain, uuid.MustParse(id))
	invTok := inv["accept_url"].(string)[strings.Index(inv["accept_url"].(string), "token=")+6:]
	r = kappa.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{"token": invTok, "name": "Boss", "password": ownerPassword}})
	h.expect(r, 201, "")
	boss := r.data()["session_token"].(string)
	r = h.admin(admin, "GET", "/tenants/"+id, nil)
	h.expect(r, 200, "")
	if owners := r.data()["owners"].([]any); len(owners) != 1 || r.data()["member_count"].(float64) != 1 {
		t.Fatalf("detail %s", r.raw)
	}

	// Trial end edit and clear.
	h.expect(h.admin(admin, "PATCH", "/tenants/"+id, map[string]any{"trial_ends_at": "2027-01-31T00:00:00Z"}), 200, "")
	r = h.admin(admin, "PATCH", "/tenants/"+id, map[string]any{"trial_ends_at": nil, "subscription_status": "active"})
	h.expect(r, 200, "")
	if r.data()["trial_ends_at"] != nil || r.data()["subscription_status"] != "active" {
		t.Fatalf("trial cleared %s", r.raw)
	}

	// Suspension: the workspace answers 403 TENANT_SUSPENDED until lifted.
	h.expect(h.admin(admin, "POST", "/tenants/"+id+"/suspend", map[string]any{"reason": "x"}), 422, "VALIDATION_ERROR")
	r = h.admin(admin, "POST", "/tenants/"+id+"/suspend", map[string]any{"reason": "Unpaid invoice"})
	h.expect(r, 200, "")
	if r.data()["status"] != "suspended" || r.data()["suspended_reason"] != "Unpaid invoice" {
		t.Fatalf("suspend %s", r.raw)
	}
	if code, errCode := h.rawAt(kappa.host, "GET", "/api/v1/me", boss, true); code != 403 || errCode != "TENANT_SUSPENDED" {
		t.Fatalf("suspended workspace: %d %s", code, errCode)
	}
	h.expect(h.admin(admin, "POST", "/tenants/"+id+"/suspend", map[string]any{"reason": "Again"}), 409, "TENANT_NOT_ACTIVE")
	h.expect(h.admin(admin, "POST", "/tenants/"+id+"/unsuspend", nil), 200, "")
	h.expect(kappa.do(call{method: "GET", path: "/api/v1/me", token: boss}), 200, "")
	// The company's own audit log tells what happened, as does the platform's.
	var actions []string
	rows, err := h.owner.Query(context.Background(), `SELECT action FROM audit_logs WHERE tenant_id = $1 ORDER BY occurred_at`, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		actions = append(actions, a)
	}
	rows.Close()
	for _, want := range []string{"tenant.create", "invitation.create", "tenant.update", "tenant.suspend", "tenant.unsuspend"} {
		if !contains(actions, want) {
			t.Errorf("tenant audit log lacks %s: %v", want, actions)
		}
	}
	r = h.admin(admin, "GET", "/audit-logs?tenant_id="+id, nil)
	h.expect(r, 200, "")
	if len(r.list()) < 4 || r.list()[0].(map[string]any)["actor_email"] != "ops@messora.test" {
		t.Fatalf("platform audit %s", r.raw)
	}

	// Custom domain: answers once its TXT record holds the token.
	h.expect(h.admin(admin, "POST", "/tenants/"+id+"/domains", map[string]any{"hostname": "crm.saas.test"}), 422, "VALIDATION_ERROR")
	r = h.admin(admin, "POST", "/tenants/"+id+"/domains", map[string]any{"hostname": "Panel.Kappa-Example.com"})
	h.expect(r, 201, "")
	dom := r.data()
	ver := dom["verification"].(map[string]any)
	if dom["hostname"] != "panel.kappa-example.com" || ver["name"] != "_messora.panel.kappa-example.com" || dom["verified_at"] != nil {
		t.Fatalf("domain %s", r.raw)
	}
	if code, _ := h.rawAt("panel.kappa-example.com", "GET", "/api/v1/me", boss, true); code != 404 {
		t.Fatalf("unverified domain answers: %d", code)
	}
	txt := map[string][]string{}
	h.svc.platform.SetLookupTXT(func(_ context.Context, name string) ([]string, error) { return txt[name], nil })
	r = h.admin(admin, "POST", "/domains/"+dom["id"].(string)+"/verify", nil)
	h.expect(r, 200, "")
	if r.data()["verified_at"] != nil || r.data()["last_check_error"] == nil {
		t.Fatalf("verified without the record: %s", r.raw)
	}
	txt["_messora.panel.kappa-example.com"] = []string{"something else", ver["value"].(string)}
	r = h.admin(admin, "POST", "/domains/"+dom["id"].(string)+"/verify", nil)
	h.expect(r, 200, "")
	if r.data()["verified_at"] == nil {
		t.Fatalf("not verified: %s", r.raw)
	}
	if code, _ := h.rawAt("panel.kappa-example.com", "GET", "/api/v1/me", "", true); code != 401 {
		t.Fatalf("verified custom domain: %d", code)
	}
	h.expect(h.admin(admin, "POST", "/domains/"+dom["id"].(string)+"/primary", nil), 200, "")
	h.expect(h.admin(admin, "DELETE", "/domains/"+dom["id"].(string), nil), 409, "DOMAIN_PRIMARY")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func TestSupportAccessIsAuditedAndReadOnly(t *testing.T) {
	h := newHarness(t)
	ownerA, _ := h.setupOwner()
	tag := h.created(ownerA, "/api/v1/tags", map[string]any{"name": "VIP"})
	admin := h.platformAdmin("ops@messora.test")

	h.expect(h.admin(admin, "POST", "/tenants/"+h.tenantID.String()+"/support-sessions", map[string]any{"reason": "x"}), 422, "VALIDATION_ERROR")
	r := h.admin(admin, "POST", "/tenants/"+h.tenantID.String()+"/support-sessions", map[string]any{"reason": "Ticket #42: tags missing", "minutes": 15})
	h.expect(r, 201, "")
	grant := r.data()["grant"].(map[string]any)
	if grant["read_only"] != true || grant["active"] != true {
		t.Fatalf("grant %s", r.raw)
	}
	u, _ := url.Parse(r.data()["redirect_url"].(string))
	if u.Host != h.host {
		t.Fatalf("support link host %s", u.Host)
	}
	r = h.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": u.Query().Get("token")}})
	h.expect(r, 200, "")
	support := r.data()["session_token"].(string)

	r = h.do(call{method: "GET", path: "/api/v1/me", token: support})
	h.expect(r, 200, "")
	sup, _ := r.data()["support"].(map[string]any)
	if sup == nil || sup["read_only"] != true || sup["reason"] != "Ticket #42: tags missing" || sup["admin_name"] != "Platform Admin" {
		t.Fatalf("me of a support session: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/tags", token: support}), 200, "")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/tags/" + tag, token: support}), 403, "SUPPORT_READ_ONLY")
	h.expect(h.do(call{method: "POST", path: "/api/v1/tags", token: support, body: map[string]any{"name": "x"}}), 403, "SUPPORT_READ_ONLY")
	// Sensitive actions stay closed even with write access.
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/reauth", token: support, body: map[string]any{"password": ownerPassword}}), 403, "SUPPORT_READ_ONLY")
	// The support admin is not a member and does not show up as one.
	r = h.do(call{method: "GET", path: "/api/v1/users", token: ownerA})
	if len(r.list()) != 1 {
		t.Fatalf("support admin listed as a member: %s", r.raw)
	}
	// The company sees the support session in its audit trail.
	r = h.do(call{method: "GET", path: "/api/v1/support-access", token: ownerA})
	h.expect(r, 200, "")
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["admin"].(map[string]any)["email"] != "ops@messora.test" {
		t.Fatalf("support access list %s", r.raw)
	}
	var n int
	if err := h.owner.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE tenant_id = $1 AND action IN ('support.grant', 'support.session_open')`, h.tenantID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("tenant audit of support: %d %v", n, err)
	}
	// Ending the grant ends the session at once.
	h.expect(h.admin(admin, "POST", "/support-sessions/"+grant["id"].(string)+"/end", nil), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: support}), 401, "SESSION_INVALID")

	// Write access is a deliberate choice.
	r = h.admin(admin, "POST", "/tenants/"+h.tenantID.String()+"/support-sessions", map[string]any{"reason": "Fix the tag", "write": true})
	h.expect(r, 201, "")
	u, _ = url.Parse(r.data()["redirect_url"].(string))
	r = h.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": u.Query().Get("token")}})
	h.expect(r, 200, "")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/tags/" + tag, token: r.data()["session_token"].(string)}), 204, "")
	var actor string
	if err := h.owner.QueryRow(context.Background(), `SELECT u.email FROM audit_logs a JOIN users u ON u.id = a.actor_id WHERE a.tenant_id = $1 AND a.action = 'tag.delete'`, h.tenantID).Scan(&actor); err != nil || actor != "ops@messora.test" {
		t.Fatalf("support change recorded as %q %v", actor, err)
	}
}

func TestRateCardAndAIPricesAreManagedByThePlatform(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	admin := h.platformAdmin("ops@messora.test")

	// The write operations are gone from workspaces.
	if code, _ := h.rawAt(h.host, "POST", "/api/v1/settings/meta-rates/import", owner, true); code != 404 {
		t.Fatalf("tenant rate card import: %d", code)
	}
	if code, _ := h.rawAt(h.host, "POST", "/api/v1/ai/prices", owner, true); code != 405 {
		t.Fatalf("tenant AI price create: %d", code)
	}
	price := map[string]any{"provider_kind": "openai", "model_pattern": "gpt-test-*", "input": "1.25", "output": "10",
		"effective_from": "2026-11-01"}
	r := h.admin(admin, "POST", "/ai-prices", price)
	h.expect(r, 201, "")
	priceID := r.data()["id"].(string)
	h.expect(h.admin(admin, "PATCH", "/ai-prices/"+priceID, map[string]any{"confirm": true}), 200, "")

	csv := "market,countries,category,price,currency,effective_from\nTurkey,TR,marketing,0.0109,USD,2026-11-01\n"
	status, raw := h.multipartAt(adminHost, admin, "/api/v1/platform/meta-rates/import", "rates.csv", []byte(csv))
	if status != 200 {
		t.Fatalf("platform import: %d %s", status, raw)
	}
	// Every workspace reads the shared tables.
	r = h.do(call{method: "GET", path: "/api/v1/settings/meta-rates", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("tenant rate card: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/ai/prices", token: owner})
	h.expect(r, 200, "")
	found := false
	for _, it := range r.list() {
		if it.(map[string]any)["id"] == priceID {
			found = true
		}
	}
	if !found {
		t.Fatalf("tenant does not see the platform price: %s", r.raw)
	}
	r = h.admin(admin, "GET", "/meta-rates", nil)
	h.expect(r, 200, "")
	h.expect(h.admin(admin, "DELETE", "/ai-prices/"+priceID, nil), 204, "")
	var n int
	if err := h.owner.QueryRow(context.Background(), `SELECT count(*) FROM platform_audit_logs WHERE action IN ('ai.price.create', 'ai.price.update', 'ai.price.delete', 'meta_rates.import')`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("platform audit of catalogue changes: %d %v", n, err)
	}
}

func TestPlatformDisablesAccountsAndForcesReset(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	admin := h.platformAdmin("ops@messora.test")

	r := h.admin(admin, "GET", "/users?q=owner@", nil)
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("search %s", r.raw)
	}
	uid := r.list()[0].(map[string]any)["id"].(string)
	r = h.admin(admin, "GET", "/users/"+uid, nil)
	h.expect(r, 200, "")
	if ms := r.data()["memberships"].([]any); len(ms) != 1 || ms[0].(map[string]any)["role"] != "owner" {
		t.Fatalf("user detail %s", r.raw)
	}

	h.expect(h.admin(admin, "POST", "/users/"+uid+"/disable", nil), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: owner}), 401, "")
	h.expect(h.login(h.host, "owner@example.com"), 401, "INVALID_CREDENTIALS")
	h.expect(h.admin(admin, "POST", "/users/"+uid+"/enable", nil), 200, "")

	r = h.admin(admin, "POST", "/users/"+uid+"/force-password-reset", nil)
	h.expect(r, 200, "")
	if r.data()["user"].(map[string]any)["password_reset_required"] != true {
		t.Fatalf("force reset %s", r.raw)
	}
	h.expect(h.login(h.host, "owner@example.com"), 403, "PASSWORD_RESET_REQUIRED")
	_, resetTok := h.mailTo("owner@example.com")
	if !strings.HasPrefix(resetTok, "msr_") {
		t.Fatalf("reset e-mail token %q", resetTok)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/auth/password/reset", body: map[string]any{"token": resetTok, "password": "A-fresh-passphrase-77"}}), 204, "")
	r = h.do(fromNewIP(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": "A-fresh-passphrase-77"}}))
	h.expect(r, 200, "")
	// Platform admins cannot lock themselves out.
	r = h.admin(admin, "GET", "/me", nil)
	self := r.data()["admin"].(map[string]any)["id"].(string)
	h.expect(h.admin(admin, "POST", "/users/"+self+"/disable", nil), 403, "SELF_MODIFICATION")
}
