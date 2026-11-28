// Package templates renders Messora's branded transactional e-mails.
//
// Every kind has a subject, a preheader, a plain-text body and an HTML body
// in Turkish and English. Copy lives in embedded template files
// (files/<locale>/<kind>.txt and .html); the shared chrome (logo, card,
// footer) lives in files/layout.* and files/partials.html. HTML uses inline
// styles and table layout only, so it survives clients that strip <style>.
//
// Data is typed per kind (VerifyEmailData, InvoiceIssuedData, ...). Render
// validates the required fields and that every link is an absolute https
// URL; html/template escapes all user-supplied values.
package templates

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"regexp"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
)

//go:embed files
var files embed.FS

// Kind identifies a transactional e-mail.
type Kind string

// The transactional e-mails.
const (
	VerifyEmail           Kind = "verify_email"
	PasswordReset         Kind = "password_reset"
	PasswordChanged       Kind = "password_changed"
	Invitation            Kind = "invitation"
	Welcome               Kind = "welcome"
	LoginAlert            Kind = "login_alert"
	TwoFactorCode         Kind = "two_factor_code"
	EmailChangeConfirm    Kind = "email_change_confirm"
	InvoiceIssued         Kind = "invoice_issued"
	PaymentReminder       Kind = "payment_reminder"
	PaymentReceived       Kind = "payment_received"
	SubscriptionSuspended Kind = "subscription_suspended"
	CreditLow             Kind = "credit_low"
	WebhookDisabled       Kind = "webhook_disabled"
	// PlanChangeRequest tells the platform admins about a workspace's
	// request for another plan (Phase 17 WP8).
	PlanChangeRequest Kind = "plan_change_request"
)

// kinds lists every kind in a stable order (preview gallery, tests).
var kinds = []Kind{
	VerifyEmail, PasswordReset, PasswordChanged, Invitation, Welcome,
	LoginAlert, TwoFactorCode, EmailChangeConfirm, InvoiceIssued,
	PaymentReminder, PaymentReceived, SubscriptionSuspended, CreditLow,
	WebhookDisabled, PlanChangeRequest,
}

// unsubscribable kinds are notifications the recipient may opt out of; only
// these may carry an UnsubscribeURL (security and billing mail may not).
var unsubscribable = map[Kind]bool{CreditLow: true}

// Kinds returns every kind in a stable order.
func Kinds() []Kind { return append([]Kind(nil), kinds...) }

// Locales are the supported locales; anything else falls back to English.
var Locales = []string{"tr", "en"}

// Locale normalises a locale or Accept-Language style tag ("tr-TR",
// "tr_TR", "TR") to a supported locale, falling back to "en".
func Locale(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_,;"); i >= 0 {
		tag = tag[:i]
	}
	if tag == "tr" {
		return "tr"
	}
	return "en"
}

// ErrInvalidData wraps every validation failure.
var ErrInvalidData = errors.New("mail template: invalid data")

// Brand is the platform identity shown in every e-mail.
type Brand struct {
	ProductName    string // "Messora"
	CompanyName    string // legal name in the footer
	CompanyAddress string // optional postal address in the footer
	WebsiteURL     string // https
	SupportURL     string // https; the footer help link
	// AssetBaseURL, when set (https), serves <AssetBaseURL>/email/logo.png
	// (264x56, shown at 132x28). Without it the logo is drawn from table
	// cells and styled text, which needs no image at all.
	AssetBaseURL string
}

// DefaultBrand is the Messora platform brand. CompanyName is a placeholder
// until the legal entity is configured (WP8 platform settings).
func DefaultBrand() Brand {
	return Brand{
		ProductName: "Messora",
		CompanyName: "Messora",
		WebsiteURL:  "https://messora.tech",
		SupportURL:  "https://messora.tech/destek",
	}
}

// Renderer renders e-mails for one brand. It is safe for concurrent use.
type Renderer struct {
	brand     Brand
	allowHTTP bool
	text      map[string]*texttemplate.Template
	html      map[string]*htmltemplate.Template
}

// New parses every template for brand.
func New(brand Brand) (*Renderer, error) {
	if brand.ProductName == "" {
		brand.ProductName = "Messora"
	}
	if brand.CompanyName == "" {
		brand.CompanyName = brand.ProductName
	}
	var v validator
	v.url("Brand.WebsiteURL", brand.WebsiteURL, true)
	v.url("Brand.SupportURL", brand.SupportURL, true)
	v.url("Brand.AssetBaseURL", brand.AssetBaseURL, false)
	if err := v.err("brand"); err != nil {
		return nil, err
	}
	brand.AssetBaseURL = strings.TrimRight(brand.AssetBaseURL, "/")
	r := &Renderer{brand: brand, text: map[string]*texttemplate.Template{}, html: map[string]*htmltemplate.Template{}}
	for _, loc := range Locales {
		fm := funcs(loc)
		for _, k := range kinds {
			key := loc + "/" + string(k)
			tt, err := texttemplate.New("").Funcs(texttemplate.FuncMap(fm)).
				ParseFS(files, "files/layout.txt", "files/"+key+".txt")
			if err != nil {
				return nil, fmt.Errorf("mail template %s (text): %w", key, err)
			}
			ht, err := htmltemplate.New("").Funcs(htmltemplate.FuncMap(fm)).
				ParseFS(files, "files/layout.html", "files/partials.html", "files/"+key+".html")
			if err != nil {
				return nil, fmt.Errorf("mail template %s (html): %w", key, err)
			}
			for _, name := range []string{"subject", "preheader", "reason", "content"} {
				if tt.Lookup(name) == nil {
					return nil, fmt.Errorf("mail template %s.txt: missing {{define %q}}", key, name)
				}
			}
			if ht.Lookup("content") == nil {
				return nil, fmt.Errorf("mail template %s.html: missing {{define \"content\"}}", key)
			}
			r.text[key], r.html[key] = tt, ht
		}
	}
	return r, nil
}

// AllowHTTP returns a renderer that also accepts plain http links. Only for
// development and test stacks that run without TLS (PUBLIC_APP_URL on
// http://); production e-mail links are always https.
func (r *Renderer) AllowHTTP() *Renderer {
	c := *r
	c.allowHTTP = true
	return &c
}

var (
	defaultOnce     sync.Once
	defaultRenderer *Renderer
	errDefault      error
)

// Default returns the renderer for DefaultBrand.
func Default() (*Renderer, error) {
	defaultOnce.Do(func() { defaultRenderer, errDefault = New(DefaultBrand()) })
	return defaultRenderer, errDefault
}

// Render renders kind with the default brand.
func Render(kind Kind, locale string, data Data) (subject, text, html string, err error) {
	r, err := Default()
	if err != nil {
		return "", "", "", err
	}
	return r.Render(kind, locale, data)
}

// view is what every template sees.
type view struct {
	Brand     Brand
	L         chrome
	Locale    string
	Kind      Kind
	C         Common
	D         Data
	Subject   string
	Preheader string
	Reason    string
	LogoURL   string // Messora PNG logo, empty for the HTML/text mark
}

// Email is one rendered e-mail.
type Email struct {
	Subject   string
	Preheader string // inbox preview line, already part of HTML
	Text      string
	HTML      string
	// ListUnsubscribe is the opt-out URL for the List-Unsubscribe header
	// (empty for mail that cannot be unsubscribed from).
	ListUnsubscribe string
}

// Render renders kind in locale (falling back to English) with data, which
// must be the kind's data type (e.g. VerifyEmailData for VerifyEmail).
func (r *Renderer) Render(kind Kind, locale string, data Data) (subject, text, html string, err error) {
	e, err := r.Email(kind, locale, data)
	return e.Subject, e.Text, e.HTML, err
}

// Email is Render with the preheader and unsubscribe URL as well.
func (r *Renderer) Email(kind Kind, locale string, data Data) (Email, error) {
	if data == nil {
		return Email{}, fmt.Errorf("%w: %s: no data", ErrInvalidData, kind)
	}
	if data.kind() != kind {
		return Email{}, fmt.Errorf("%w: %s: got data for %s", ErrInvalidData, kind, data.kind())
	}
	c := data.common()
	v := validator{allowHTTP: r.allowHTTP}
	c.validateCommon(&v, unsubscribable[kind])
	data.validate(&v)
	if err := v.err(string(kind)); err != nil {
		return Email{}, err
	}
	locale = Locale(locale)
	key := locale + "/" + string(kind)
	tt, ht := r.text[key], r.html[key]
	vw := view{Brand: r.brand, L: chromes[locale], Locale: locale, Kind: kind, C: c, D: data}
	if r.brand.AssetBaseURL != "" {
		vw.LogoURL = r.brand.AssetBaseURL + "/email/logo.png"
	}
	exec := func(name string) (string, error) {
		var b bytes.Buffer
		if err := tt.ExecuteTemplate(&b, name, vw); err != nil {
			return "", fmt.Errorf("mail template %s: %s: %w", key, name, err)
		}
		return oneLine(b.String()), nil
	}
	var err error
	if vw.Subject, err = exec("subject"); err != nil {
		return Email{}, err
	}
	if vw.Preheader, err = exec("preheader"); err != nil {
		return Email{}, err
	}
	if vw.Reason, err = exec("reason"); err != nil {
		return Email{}, err
	}
	var tb, hb bytes.Buffer
	if err := tt.ExecuteTemplate(&tb, "layout", vw); err != nil {
		return Email{}, fmt.Errorf("mail template %s: text: %w", key, err)
	}
	if err := ht.ExecuteTemplate(&hb, "layout", vw); err != nil {
		return Email{}, fmt.Errorf("mail template %s: html: %w", key, err)
	}
	return Email{
		Subject: vw.Subject, Preheader: vw.Preheader,
		Text: tidyText(tb.String()), HTML: tidyHTML(hb.String()),
		ListUnsubscribe: c.UnsubscribeURL,
	}, nil
}

// Message renders kind and wraps it as a multipart mailer.Message for to.
func (r *Renderer) Message(to string, kind Kind, locale string, data Data) (mailer.Message, error) {
	e, err := r.Email(kind, locale, data)
	if err != nil {
		return mailer.Message{}, err
	}
	return mailer.Message{
		To: to, Subject: e.Subject, Body: e.Text, HTML: e.HTML,
		ListUnsubscribe: e.ListUnsubscribe,
	}, nil
}

var (
	spaces     = regexp.MustCompile(`\s+`)
	blankLines = regexp.MustCompile(`\n{3,}`)
	trailingWS = regexp.MustCompile(`[ \t]+\n`)
	htmlGaps   = regexp.MustCompile(`>\s*\n\s*<`)
)

func oneLine(s string) string { return strings.TrimSpace(spaces.ReplaceAllString(s, " ")) }

// tidyText trims trailing blanks and collapses runs of empty lines left by
// template actions.
func tidyText(s string) string {
	s = trailingWS.ReplaceAllString(s, "\n")
	s = blankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s) + "\n"
}

// tidyHTML drops the indentation between tags (smaller messages, and Gmail
// clips anything above ~102 KB). Only whitespace containing a newline that sits
// between two tags is removed, so text content keeps its spaces.
func tidyHTML(s string) string {
	return strings.TrimSpace(htmlGaps.ReplaceAllString(s, "><")) + "\n"
}
