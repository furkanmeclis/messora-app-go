package identity

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// workspace is what links and e-mails need to know about a tenant.
type workspace struct {
	ID   uuid.UUID
	Slug string
	// Name is the company's display name (its organisation name setting,
	// else the tenant's name).
	Name string
	// URL is the origin of its primary address, e.g. https://acme.messora.tech.
	URL string
	// LogoURL is an absolute link to its logo, or "".
	LogoURL string
}

// workspaceOf loads the tenant's name, address and logo. It reads the
// registry (readable without a tenant) and the tenant's settings.
func (s *Service) workspaceOf(ctx context.Context, id uuid.UUID) (workspace, error) {
	t, err := s.q.GetTenant(tenant.System(ctx), id)
	if err != nil {
		return workspace{}, err
	}
	w := workspace{ID: id, Slug: t.Slug, Name: t.Name}
	w.URL, err = s.originOf(ctx, id, t.Slug)
	if err != nil {
		return workspace{}, err
	}
	if st, err := s.settings.Get(tenant.WithID(ctx, id)); err == nil {
		if n := strings.TrimSpace(st.OrganizationName); n != "" && n != "Messora" {
			w.Name = n
		}
		if st.Logo != nil {
			w.LogoURL = w.URL + "/bff/logo?v=" + url.QueryEscape(st.Logo.Version)
		}
	}
	return w, nil
}

// originOf is the origin of the tenant's primary address (its oldest
// verified one without a primary), else its platform sub-domain, else
// PUBLIC_APP_URL.
func (s *Service) originOf(ctx context.Context, id uuid.UUID, slug string) (string, error) {
	host, err := s.q.PrimaryHost(tenant.System(ctx), id)
	switch {
	case err == nil:
		return s.o.Hosts.URL(host), nil
	case isNoRows(err):
		if h := s.o.Hosts.TenantHost(slug); h != "" {
			return s.o.Hosts.URL(h), nil
		}
		return strings.TrimRight(s.o.PublicAppURL, "/"), nil
	}
	return "", err
}

// tenantOrigin is originOf, "" when it cannot be read (best effort).
func (s *Service) tenantOrigin(ctx context.Context, id uuid.UUID, slug string) string {
	o, err := s.originOf(ctx, id, slug)
	if err != nil {
		s.o.Logger.WarnContext(ctx, "workspace address", "tenant_id", id, "err", err)
		return ""
	}
	return o
}

// link returns an absolute link into the workspace.
func (w workspace) link(path string, query ...string) string {
	u := w.URL + path
	if len(query) >= 2 {
		v := url.Values{}
		for i := 0; i+1 < len(query); i += 2 {
			v.Set(query[i], query[i+1])
		}
		u += "?" + v.Encode()
	}
	return u
}

// common is the branding block of e-mails about w for the person called name.
func (w workspace) common(name string) templates.Common {
	c := templates.Common{Name: firstName(name), TenantName: w.Name}
	if w.LogoURL != "" {
		c.TenantLogoURL = w.LogoURL
	}
	return c
}

func firstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return ""
}

// sendTo renders kind for to and sends it through the sender of the
// workspace (the platform sender unless the company has its own SMTP
// server). It reports whether the e-mail went out; failures are logged,
// never returned: e-mail is best effort for every caller.
//
// TODO(WP8): a company's own SMTP server is used only with the custom_smtp
// entitlement; settings.Mail decides today.
func (s *Service) sendTo(ctx context.Context, w workspace, to, locale string, kind templates.Kind, data templates.Data) bool {
	mctx := tenant.WithID(ctx, w.ID)
	if !s.mail.Enabled(mctx) {
		return false
	}
	msg, err := s.tpl.Message(to, kind, locale, data)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "render e-mail", "kind", kind, "err", err)
		return false
	}
	msg.ReplyTo = s.replyTo(ctx)
	if err := s.mail.Send(mctx, msg); err != nil {
		s.o.Logger.ErrorContext(ctx, "send e-mail", "kind", kind, "tenant_id", w.ID, "err", err)
		return false
	}
	return true
}

// sendToUser sends kind to u in u's language about the workspace id.
func (s *Service) sendToUser(ctx context.Context, id uuid.UUID, u db.User, kind templates.Kind, build func(w workspace) templates.Data) bool {
	w, err := s.workspaceOf(ctx, id)
	if err != nil {
		s.o.Logger.ErrorContext(ctx, "load workspace for e-mail", "tenant_id", id, "err", err)
		return false
	}
	return s.sendTo(ctx, w, u.Email, u.Locale, kind, build(w))
}

// replyTo is the platform's reply-to address (platform setting), or "".
func (s *Service) replyTo(ctx context.Context) string {
	raw, err := s.q.GetPlatformSetting(tenant.System(ctx), SettingReplyTo)
	if err != nil {
		return ""
	}
	var v string
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return v
}

// roleLabel is the role's name in the recipient's language.
func roleLabel(key, locale string) string {
	tr := map[string]string{"owner": "Sahip", "admin": "Yönetici", "manager": "Müdür", "agent": "Temsilci"}
	en := map[string]string{"owner": "Owner", "admin": "Admin", "manager": "Manager", "agent": "Agent"}
	if templates.Locale(locale) == "tr" {
		if v, ok := tr[key]; ok {
			return v
		}
	}
	if v, ok := en[key]; ok {
		return v
	}
	return key
}

// deviceOf turns a user agent into "Chrome, macOS" (best effort).
func deviceOf(ua string) string {
	if ua == "" {
		return ""
	}
	browser := ""
	for _, b := range []struct{ token, name string }{
		{"Edg/", "Edge"}, {"OPR/", "Opera"}, {"Firefox/", "Firefox"}, {"Chrome/", "Chrome"}, {"Safari/", "Safari"},
	} {
		if strings.Contains(ua, b.token) {
			browser = b.name
			break
		}
	}
	osName := ""
	for _, o := range []struct{ token, name string }{
		{"iPhone", "iOS"}, {"iPad", "iPadOS"}, {"Android", "Android"}, {"Mac OS X", "macOS"}, {"Windows", "Windows"}, {"Linux", "Linux"},
	} {
		if strings.Contains(ua, o.token) {
			osName = o.name
			break
		}
	}
	switch {
	case browser != "" && osName != "":
		return browser + ", " + osName
	case browser != "":
		return browser
	case osName != "":
		return osName
	}
	if len(ua) > 60 {
		ua = ua[:60]
	}
	return ua
}
