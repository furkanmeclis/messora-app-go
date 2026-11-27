package templates

import (
	"bytes"
	"errors"
	"flag"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

func renderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := New(DefaultBrand())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestGolden pins subject, preheader and text body of every kind × locale.
// Regenerate with: go test ./internal/platform/mailer/templates -update
func TestGolden(t *testing.T) {
	r := renderer(t)
	for _, k := range Kinds() {
		for _, loc := range Locales {
			t.Run(string(k)+"."+loc, func(t *testing.T) {
				e, err := r.Email(k, loc, Sample(k, loc))
				if err != nil {
					t.Fatal(err)
				}
				checkHTML(t, e.HTML, e.Subject, e.Preheader)
				got := "Subject: " + e.Subject + "\nPreheader: " + e.Preheader + "\n\n" + e.Text
				path := filepath.Join("testdata", string(k)+"."+loc+".golden")
				if *update {
					if err := os.MkdirAll("testdata", 0o750); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("%v (run with -update to create)", err)
				}
				if got != string(want) {
					t.Errorf("%s is stale (run with -update after reviewing):\n--- got\n%s\n--- want\n%s", path, got, want)
				}
			})
		}
	}
}

var (
	reTag   = regexp.MustCompile(`(?is)<(a|img|link|script|iframe|form|object|embed)\b[^>]*>`)
	reHref  = regexp.MustCompile(`(?i)\bhref="([^"]*)"`)
	reAlt   = regexp.MustCompile(`(?i)\balt="[^"]+"`)
	reTable = regexp.MustCompile(`<table[^>]*>`)
)

// checkHTML enforces the e-mail client rules from docs/EMAIL.md.
func checkHTML(t *testing.T, html, subject, preheader string) {
	t.Helper()
	if preheader == "" || !strings.Contains(html, `<div style="display:none;`) {
		t.Error("preheader missing")
	}
	if len(html) > 100*1024 {
		t.Errorf("html is %d bytes; Gmail clips above ~102 KB", len(html))
	}
	lower := strings.ToLower(html)
	for _, bad := range []string{"@import", "<link", "<script", "javascript:", "http://", "<svg", "zgotmpl"} {
		if strings.Contains(lower, bad) {
			t.Errorf("html contains %q", bad)
		}
	}
	for _, must := range []string{`<meta name="color-scheme" content="light dark">`, "prefers-color-scheme: dark", `role="presentation"`, "max-width:600px"} {
		if !strings.Contains(html, must) {
			t.Errorf("html lacks %q", must)
		}
	}
	if !strings.Contains(html, "<title>") || !strings.Contains(html, "<html lang=") {
		t.Error("title and lang required")
	}
	if subject == "" {
		t.Error("empty subject")
	}
	links := 0
	for _, tag := range reTag.FindAllStringSubmatch(html, -1) {
		switch strings.ToLower(tag[1]) {
		case "a":
			links++
			h := reHref.FindStringSubmatch(tag[0])
			if h == nil || !strings.HasPrefix(h[1], "https://") {
				t.Errorf("link without https href: %s", tag[0])
			}
		case "img":
			if !reAlt.MatchString(tag[0]) {
				t.Errorf("img without alt: %s", tag[0])
			}
		default:
			t.Errorf("forbidden tag: %s", tag[0])
		}
	}
	if links == 0 {
		t.Error("no links (support link at least)")
	}
	for _, tbl := range reTable.FindAllString(html, -1) {
		if !strings.Contains(tbl, `role="presentation"`) {
			t.Errorf("layout table without role=presentation: %s", tbl)
		}
	}
	checkDarkMode(t, html)
}

var (
	reStyled   = regexp.MustCompile(`<[a-zA-Z0-9]+\b[^>]*\bstyle="([^"]*)"[^>]*>`)
	reColor    = regexp.MustCompile(`(?:^|;)\s*(?:background-)?color:`)
	reBgcolor  = regexp.MustCompile(`\bbgcolor="`)
	reThemeCls = regexp.MustCompile(`\bclass="[^"]*\bm-[a-z]+`)
)

// checkDarkMode: Apple Mail and Outlook.com darken the white card but keep
// inline text colours, so every element that sets a colour inline must be
// re-themed by a class in the dark-mode block.
func checkDarkMode(t *testing.T, html string) {
	t.Helper()
	for _, must := range []string{
		"@media (prefers-color-scheme: dark)",
		`<meta name="supported-color-schemes" content="light dark">`,
		"[data-ogsc] .m-text", "[data-ogsb] .m-card",
		"a[x-apple-data-detectors]",
	} {
		if !strings.Contains(html, must) {
			t.Errorf("dark mode / auto-link handling lacks %q", must)
		}
	}
	for _, cls := range []string{".m-bg", ".m-card", ".m-text", ".m-muted", ".m-box", ".m-link", ".m-btn", ".m-mint"} {
		if !strings.Contains(html, "  "+cls) && !strings.Contains(html, ", "+cls) {
			t.Errorf("dark theme has no rule for %s", cls)
		}
	}
	tags := reStyled.FindAllStringSubmatch(html, -1)
	if len(tags) == 0 {
		t.Fatal("no styled elements found")
	}
	for _, m := range tags {
		if (reColor.MatchString(m[1]) || reBgcolor.MatchString(m[0])) && !reThemeCls.MatchString(m[0]) {
			t.Errorf("inline colour without a theming class: %.160s", m[0])
		}
	}
}

func TestEveryKindHasSampleAndBothLocales(t *testing.T) {
	r := renderer(t)
	for _, k := range Kinds() {
		if Sample(k, "tr") == nil {
			t.Fatalf("%s: no sample", k)
		}
		tr, _, _, err := r.Render(k, "tr", Sample(k, "tr"))
		if err != nil {
			t.Fatal(err)
		}
		en, _, _, _ := r.Render(k, "en", Sample(k, "en"))
		if tr == en {
			t.Errorf("%s: tr and en subjects are identical (%q)", k, tr)
		}
	}
}

func TestLocaleFallback(t *testing.T) {
	for in, want := range map[string]string{"tr": "tr", "tr-TR": "tr", "TR_tr": "tr", "tr,en;q=0.8": "tr", "en-GB": "en", "de": "en", "": "en"} {
		if got := Locale(in); got != want {
			t.Errorf("Locale(%q) = %q, want %q", in, got, want)
		}
	}
	r := renderer(t)
	en, _, _, _ := r.Render(Welcome, "en", Sample(Welcome, "en"))
	de, _, _, err := r.Render(Welcome, "de-DE", Sample(Welcome, "en"))
	if err != nil || de != en {
		t.Fatalf("unsupported locale must fall back to en: %q %v", de, err)
	}
}

func TestMissingFields(t *testing.T) {
	r := renderer(t)
	cases := []struct {
		kind Kind
		data Data
		want []string
	}{
		{VerifyEmail, VerifyEmailData{}, []string{"missing VerifyURL", "missing ExpiresIn"}},
		{PasswordReset, PasswordResetData{ResetURL: "http://insecure.example.com/r", ExpiresIn: time.Hour}, []string{"ResetURL must be an absolute https URL"}},
		{PasswordChanged, PasswordChangedData{}, []string{"missing ChangedAt", "missing SecureURL"}},
		{Invitation, InvitationData{AcceptURL: "/relative", ExpiresIn: time.Hour}, []string{"missing InviterName", "missing WorkspaceName", "missing Role", "AcceptURL must be an absolute https URL"}},
		{Welcome, WelcomeData{DashboardURL: "https://a.example.com"}, []string{"missing ConnectWhatsAppURL", "missing ImportContactsURL", "missing NewCampaignURL"}},
		{LoginAlert, LoginAlertData{At: time.Now(), SecureURL: "javascript:alert(1)"}, []string{"missing IP", "SecureURL must be an absolute https URL"}},
		{TwoFactorCode, TwoFactorCodeData{Code: " "}, []string{"missing Code", "missing ExpiresIn"}},
		{EmailChangeConfirm, EmailChangeConfirmData{}, []string{"missing NewEmail", "missing ConfirmURL"}},
		{InvoiceIssued, InvoiceIssuedData{InvoiceNo: "1", Amount: Money{Minor: 100, Currency: "try"}, Bank: &BankTransfer{}}, []string{"Amount.Currency", "missing DueDate", "missing Bank.IBAN", "missing Bank.Reference", "missing InvoiceURL"}},
		{PaymentReminder, PaymentReminderData{InvoiceNo: "1", Amount: Money{Minor: 1, Currency: "TRY"}, DueDate: time.Now(), Overdue: true, PayURL: "https://a.example.com"}, []string{"missing GraceEndsAt"}},
		{PaymentReceived, PaymentReceivedData{}, []string{"missing InvoiceNo", "missing Amount.Minor", "missing PaidAt"}},
		{SubscriptionSuspended, SubscriptionSuspendedData{}, []string{"missing SuspendedAt", "missing BillingURL"}},
		{CreditLow, CreditLowData{CreditType: "sms", Balance: -1}, []string{"CreditType", "Balance", "missing Threshold", "missing TopUpURL"}},
		// Security mail must never carry an unsubscribe link.
		{TwoFactorCode, TwoFactorCodeData{Common: Common{UnsubscribeURL: "https://a.example.com/u"}, Code: "1", ExpiresIn: time.Minute}, []string{"UnsubscribeURL is not allowed"}},
		{Welcome, func() Data {
			d := Sample(Welcome, "en").(WelcomeData)
			d.TenantLogoURL = "http://cdn.example.com/logo.png"
			return d
		}(), []string{"TenantLogoURL must be an absolute https URL", "missing TenantName"}},
	}
	for _, c := range cases {
		_, _, _, err := r.Render(c.kind, "tr", c.data)
		if !errors.Is(err, ErrInvalidData) {
			t.Errorf("%s: want ErrInvalidData, got %v", c.kind, err)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: error %q lacks %q", c.kind, err, w)
			}
		}
	}
	if _, _, _, err := r.Render(VerifyEmail, "tr", Sample(Welcome, "tr")); !errors.Is(err, ErrInvalidData) {
		t.Errorf("mismatched data type accepted: %v", err)
	}
	if _, _, _, err := r.Render(VerifyEmail, "tr", nil); !errors.Is(err, ErrInvalidData) {
		t.Errorf("nil data accepted: %v", err)
	}
}

func TestEscapesUserData(t *testing.T) {
	r := renderer(t)
	d := Sample(Invitation, "tr").(InvitationData)
	d.Name = `<script>alert("x")</script>`
	d.InviterName = `Mehmet <img src=x onerror=alert(1)>`
	d.TenantName = `Acme" onmouseover="x`
	_, text, html, err := r.Render(Invitation, "tr", d)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<script>", "<img src=x", `" onmouseover="`} {
		if strings.Contains(html, bad) {
			t.Errorf("unescaped %q in html", bad)
		}
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("name not escaped as text")
	}
	if !strings.Contains(text, `<script>alert("x")</script>`) {
		t.Error("text part must carry the raw value (it is plain text)")
	}
}

func TestTenantBrandingAndLogo(t *testing.T) {
	r := renderer(t)
	_, text, html, _ := r.Render(Invitation, "tr", Sample(Invitation, "tr"))
	if !strings.Contains(html, "Acme Gıda") || !strings.Contains(html, "· Messora ile") || !strings.HasPrefix(text, "Acme Gıda · Messora ile\n") {
		t.Error("tenant header missing")
	}
	_, text, html, _ = r.Render(VerifyEmail, "tr", Sample(VerifyEmail, "tr"))
	if !strings.Contains(html, ">messora</td>") || !strings.HasPrefix(text, "Messora\n") || strings.Contains(html, "<img") {
		t.Error("platform mail must use the HTML wordmark without images")
	}
	d := Sample(Invitation, "en").(InvitationData)
	d.TenantLogoURL = "https://cdn.example.com/acme.png"
	_, _, html, _ = r.Render(Invitation, "en", d)
	if !strings.Contains(html, `src="https://cdn.example.com/acme.png"`) || !strings.Contains(html, `alt="Acme Gıda"`) {
		t.Error("tenant logo missing")
	}

	b := DefaultBrand()
	b.AssetBaseURL = "https://assets.messora.tech/"
	rb, err := New(b)
	if err != nil {
		t.Fatal(err)
	}
	_, _, html, _ = rb.Render(VerifyEmail, "en", Sample(VerifyEmail, "en"))
	if !strings.Contains(html, `src="https://assets.messora.tech/email/logo.png"`) || !strings.Contains(html, `alt="messora"`) {
		t.Error("PNG logo from AssetBaseURL missing")
	}
	b.AssetBaseURL = "http://assets.example.com"
	if _, err := New(b); err == nil {
		t.Error("http asset base accepted")
	}
}

func TestUnsubscribeOnlyWhereRelevant(t *testing.T) {
	r := renderer(t)
	m, err := r.Message("Ayşe <ayse@example.com>", CreditLow, "tr", Sample(CreditLow, "tr"))
	if err != nil {
		t.Fatal(err)
	}
	if m.ListUnsubscribe == "" || !strings.Contains(m.HTML, "Bu bildirimleri kapat") || !strings.Contains(m.Body, "Bu bildirimleri kapat:") {
		t.Error("credit_low must carry the unsubscribe link and header")
	}
	if m.To != "Ayşe <ayse@example.com>" || m.Subject == "" || m.Body == "" {
		t.Errorf("message: %+v", m)
	}
	m, _ = r.Message("a@example.com", PasswordReset, "tr", Sample(PasswordReset, "tr"))
	if m.ListUnsubscribe != "" || strings.Contains(m.HTML, "Bu bildirimleri kapat") {
		t.Error("security mail must not offer unsubscribe")
	}
}

func TestFormatting(t *testing.T) {
	tr, en := funcs("tr"), funcs("en")
	dur := func(f map[string]any, d time.Duration) string { return f["dur"].(func(time.Duration) string)(d) }
	money := func(f map[string]any, m Money) string { return f["money"].(func(Money) string)(m) }
	for _, c := range []struct{ got, want string }{
		{dur(tr, 30*time.Minute), "30 dakika"},
		{dur(tr, 90*time.Minute), "1 saat 30 dakika"},
		{dur(en, 90*time.Minute), "1 hour and 30 minutes"},
		{dur(en, time.Hour), "1 hour"},
		{dur(en, 7*24*time.Hour), "7 days"},
		{dur(tr, 48*time.Hour), "2 gün"},
		{money(tr, Money{Minor: 123456789, Currency: "TRY"}), "1.234.567,89 ₺"},
		{money(en, Money{Minor: 123456789, Currency: "TRY"}), "₺1,234,567.89"},
		{money(en, Money{Minor: 5, Currency: "CHF"}), "0.05 CHF"},
		{tr["num"].(func(int64) string)(12500), "12.500"},
		{tr["iban"].(func(string) string)("tr33 0006 1005 1978 6457 8413 26"), "TR33 0006 1005 1978 6457 8413 26"},
		{tr["datetime"].(func(time.Time) string)(time.Date(2026, 2, 1, 9, 5, 0, 0, time.FixedZone("", 3*3600))), "1 Şubat 2026, 09:05 (UTC+03:00)"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

// The rendered subject and bodies survive the real MIME encoder; this lives
// here (not in mailer) because it exercises actual template output.
func TestRenderedMessageAddresses(t *testing.T) {
	r := renderer(t)
	for _, k := range Kinds() {
		m, err := r.Message("Ayşe Yılmaz <ayse@example.com>", k, "tr", Sample(k, "tr"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := mail.ParseAddress(m.To); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(m.Subject, "\r\n") || !bytes.Contains([]byte(m.HTML), []byte("<!DOCTYPE html>")) {
			t.Errorf("%s: bad message", k)
		}
	}
}
