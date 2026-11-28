package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// E-mail (Phase 17 WP8, docs/EMAIL.md "Delivery"):
//
//   - The platform sends everything through its own sender (Resend over
//     SMTP): the sender set in the platform admin panel
//     (platform_settings "email.smtp"), or SMTP_* of the environment.
//     Mail about a company goes out with the company's name as the display
//     name and the company's reply-to address (settings "email").
//   - A company whose plan includes custom_smtp (enterprise) may send
//     through its own SMTP server (settings "smtp"). When a send through it
//     fails, the message goes out through the platform sender instead and
//     the failure is kept (settings "smtp_failure") and shown on the E-mail
//     page until a send through the company's server works again.
//   - Other companies' stored SMTP configuration is kept but not used.

const (
	smtpKey        = "smtp"
	smtpFailureKey = "smtp_failure"
	emailKey       = "email"
	// PlatformSMTPKey is the platform sender set in the admin panel.
	PlatformSMTPKey = "email.smtp"
	// platformReplyToKey is the platform's Reply-To (identity.SettingReplyTo).
	platformReplyToKey = "email.reply_to"
)

// SMTP is an outgoing e-mail configuration. The password is stored
// encrypted and never returned.
type SMTP struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordEnc []byte `json:"password_enc,omitempty"`
	From        string `json:"from"`
	TLS         string `json:"tls"`
}

// SMTPView is what the panel sees.
type SMTPView struct {
	Host         string
	Port         int
	Username     string
	From         string
	TLS          string
	PasswordSet  bool
	ManagedByEnv bool
	Enabled      bool
}

// SMTPInput changes a configuration. A nil Password keeps the stored one;
// an empty Host removes the configuration.
type SMTPInput struct {
	Host     string
	Port     int
	Username string
	Password *string
	From     string
	TLS      string
}

// Mail is the e-mail sender for the whole application.
type Mail struct {
	q      db.Querier
	sealer *crypto.Sealer
	env    *mailer.Config // nil unless SMTP_HOST is set
	fixed  mailer.Sender  // the SMTP_HOST sender
	plans  entitlements.Provider
	log    *slog.Logger

	senders  *tenant.Cache[mailer.Sender]
	identity *tenant.Cache[sendIdentity]

	mu       sync.Mutex
	platform mailer.Sender
	from     string
	expires  time.Time
}

// sendIdentity is how a company's mail looks through the platform sender.
type sendIdentity struct {
	name, replyTo string
}

const smtpAD = "settings:smtp"

// platformSMTPAD is the associated data of the platform sender's password.
const platformSMTPAD = "platform:email.smtp"

// mailTTL is how long senders and identities are cached.
const mailTTL = 15 * time.Second

// NewMail returns the application sender. env is the environment
// configuration (an empty Host means "not set").
func NewMail(q db.Querier, sealer *crypto.Sealer, env mailer.Config) (*Mail, error) {
	m := &Mail{q: q, sealer: sealer, senders: tenant.NewCache[mailer.Sender](mailTTL),
		identity: tenant.NewCache[sendIdentity](mailTTL), log: slog.Default()}
	if env.Host != "" {
		s, err := mailer.New(env)
		if err != nil {
			return nil, err
		}
		m.env, m.fixed = &env, s
	}
	return m, nil
}

// SetPlans installs the plan provider (custom_smtp). Without one every
// company may use its own server (the behaviour before plans).
func (m *Mail) SetPlans(p entitlements.Provider) { m.plans = p }

// SetLogger sets the logger of fallbacks.
func (m *Mail) SetLogger(l *slog.Logger) { m.log = l }

// ManagedByEnv reports whether SMTP_HOST is set.
func (m *Mail) ManagedByEnv() bool { return m.env != nil }

// CustomAllowed reports whether ctx's company may send through its own SMTP
// server (custom_smtp).
func (m *Mail) CustomAllowed(ctx context.Context) bool {
	if _, ok := tenant.FromCtx(ctx); !ok || tenant.IsPlatform(ctx) || tenant.IsSystem(ctx) {
		return false
	}
	if m.plans == nil {
		return true
	}
	p, err := entitlements.For(ctx, m.plans)
	return err == nil && p.Has(entitlements.CustomSMTP)
}

// Platform returns the platform sender: the admin panel's, else SMTP_*.
func (m *Mail) Platform(ctx context.Context) mailer.Sender {
	s, _ := m.platformSender(ctx)
	return s
}

// PlatformFrom is the platform sender's From address ("" when none).
func (m *Mail) PlatformFrom(ctx context.Context) string {
	_, from := m.platformSender(ctx)
	return from
}

func (m *Mail) platformSender(ctx context.Context) (mailer.Sender, string) {
	m.mu.Lock()
	if m.platform != nil && time.Now().Before(m.expires) {
		s, from := m.platform, m.from
		m.mu.Unlock()
		return s, from
	}
	m.mu.Unlock()
	var (
		s    mailer.Sender = mailer.Disabled{}
		from string
	)
	if m.fixed != nil {
		s, from = m.fixed, m.env.From
	}
	if c, err := m.loadPlatform(ctx, m.q); err != nil {
		m.log.WarnContext(ctx, "read the platform e-mail sender", "err", err)
	} else if c.Host != "" {
		if built, err := m.build(c, platformSMTPAD); err == nil {
			s, from = built, c.From
		} else {
			m.log.WarnContext(ctx, "the platform e-mail sender is invalid; using the environment's", "err", err)
		}
	}
	m.mu.Lock()
	m.platform, m.from, m.expires = s, from, time.Now().Add(mailTTL)
	m.mu.Unlock()
	return s, from
}

// custom returns ctx's company's own sender when its plan allows it and one
// is configured (nil otherwise).
func (m *Mail) custom(ctx context.Context) mailer.Sender {
	if !m.CustomAllowed(ctx) {
		return nil
	}
	if s, ok := m.senders.Get(ctx); ok {
		if !s.Enabled(ctx) {
			return nil
		}
		return s
	}
	var s mailer.Sender = mailer.Disabled{}
	if cfg, err := m.load(ctx, m.q); err == nil && cfg.Host != "" {
		if built, err := m.build(cfg, smtpAD); err == nil {
			s = built
		}
	}
	m.senders.Set(ctx, s)
	if !s.Enabled(ctx) {
		return nil
	}
	return s
}

// Send implements mailer.Sender: through the company's own server when it
// may and has one (falling back to the platform sender when that fails),
// else through the platform sender with the company's name and reply-to.
func (m *Mail) Send(ctx context.Context, msg mailer.Message) error {
	if c := m.custom(ctx); c != nil {
		err := c.Send(ctx, msg)
		if err == nil {
			m.clearFailure(ctx)
			return nil
		}
		m.log.WarnContext(ctx, "the company's SMTP server failed; sending through the platform sender", "err", err)
		m.recordFailure(ctx, err)
	}
	p := m.Platform(ctx)
	if !p.Enabled(ctx) {
		return p.Send(ctx, msg)
	}
	return p.Send(ctx, m.decorate(ctx, msg))
}

// Enabled implements mailer.Sender.
func (m *Mail) Enabled(ctx context.Context) bool {
	return m.custom(ctx) != nil || m.Platform(ctx).Enabled(ctx)
}

// decorate gives mail about a company sent through the platform sender
// the company's name (display name) and reply-to address.
func (m *Mail) decorate(ctx context.Context, msg mailer.Message) mailer.Message {
	if _, ok := tenant.FromCtx(ctx); !ok || tenant.IsPlatform(ctx) || tenant.IsSystem(ctx) {
		return msg
	}
	id, ok := m.identity.Get(ctx)
	if !ok {
		id = sendIdentity{name: tenantName(ctx, m.q), replyTo: TenantReplyTo(ctx, m.q)}
		m.identity.Set(ctx, id)
	}
	if msg.FromName == "" {
		msg.FromName = id.name
	}
	if msg.ReplyTo == "" {
		msg.ReplyTo = id.replyTo
		if msg.ReplyTo == "" {
			msg.ReplyTo = PlatformReplyTo(ctx, m.q)
		}
	}
	return msg
}

// tenantName is the company's name for the display name of its mail.
func tenantName(ctx context.Context, q db.Querier) string {
	raw, err := q.GetSetting(ctx, workspaceKey)
	if err == nil {
		var s struct {
			OrganizationName string `json:"organization_name"`
		}
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s.OrganizationName) != "" {
			return strings.TrimSpace(s.OrganizationName)
		}
	}
	if id, ok := tenant.FromCtx(ctx); ok {
		if t, err := q.GetTenant(tenant.System(ctx), id); err == nil {
			return t.Name
		}
	}
	return ""
}

// EmailIdentity is the company's e-mail settings (settings "email").
type EmailIdentity struct {
	ReplyTo string `json:"reply_to,omitempty"`
}

// TenantReplyTo is ctx's company's reply-to address ("" when none).
func TenantReplyTo(ctx context.Context, q db.Querier) string {
	raw, err := q.GetSetting(ctx, emailKey)
	if err != nil {
		return ""
	}
	var e EmailIdentity
	_ = json.Unmarshal(raw, &e)
	return e.ReplyTo
}

// PlatformReplyTo is the platform's reply-to address ("" when none).
func PlatformReplyTo(ctx context.Context, q db.Querier) string {
	raw, err := q.GetPlatformSetting(tenant.System(ctx), platformReplyToKey)
	if err != nil {
		return ""
	}
	var v string
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return v
}

// SetReplyTo stores the company's reply-to address with q ("" removes it).
func (m *Mail) SetReplyTo(ctx context.Context, q db.Querier, replyTo string, by *uuid.UUID) error {
	replyTo = strings.TrimSpace(replyTo)
	if replyTo != "" {
		a, err := mail.ParseAddress(replyTo)
		if err != nil || strings.ContainsAny(replyTo, "\r\n") {
			return fmt.Errorf("%w: enter a valid reply-to address", ErrInvalid)
		}
		replyTo = a.Address
	}
	raw, _ := json.Marshal(EmailIdentity{ReplyTo: replyTo})
	return q.UpsertSetting(ctx, db.UpsertSettingParams{Key: emailKey, Value: raw, UpdatedBy: by})
}

// Failure is the last failed send through a company's own server.
type Failure struct {
	At    time.Time `json:"at"`
	Error string    `json:"error"`
}

func (m *Mail) recordFailure(ctx context.Context, err error) {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	raw, _ := json.Marshal(Failure{At: time.Now().UTC(), Error: msg})
	if err := m.q.UpsertSetting(context.WithoutCancel(ctx), db.UpsertSettingParams{Key: smtpFailureKey, Value: raw}); err != nil {
		m.log.WarnContext(ctx, "record the SMTP failure", "err", err)
	}
}

func (m *Mail) clearFailure(ctx context.Context) {
	if _, err := m.q.GetSetting(ctx, smtpFailureKey); err == nil {
		_ = m.q.DeleteSetting(context.WithoutCancel(ctx), smtpFailureKey)
	}
}

// LastFailure returns the last failed send through the company's own
// server (nil when the last send worked or none failed).
func (m *Mail) LastFailure(ctx context.Context) *Failure {
	raw, err := m.q.GetSetting(ctx, smtpFailureKey)
	if err != nil {
		return nil
	}
	var f Failure
	if json.Unmarshal(raw, &f) != nil || f.At.IsZero() {
		return nil
	}
	return &f
}

// Invalidate drops the cached senders after a change commits.
func (m *Mail) Invalidate() {
	m.senders.Clear()
	m.identity.Clear()
	m.mu.Lock()
	m.expires = time.Time{}
	m.mu.Unlock()
}

func (m *Mail) load(ctx context.Context, q db.Querier) (SMTP, error) {
	var out SMTP
	raw, err := q.GetSetting(ctx, smtpKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("settings: corrupt SMTP document: %w", err)
	}
	return out, nil
}

func (m *Mail) loadPlatform(ctx context.Context, q db.Querier) (SMTP, error) {
	var out SMTP
	raw, err := q.GetPlatformSetting(tenant.System(ctx), PlatformSMTPKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("settings: corrupt platform SMTP document: %w", err)
	}
	return out, nil
}

func (m *Mail) build(c SMTP, ad string) (mailer.Sender, error) {
	cfg := mailer.Config{Host: c.Host, Port: c.Port, Username: c.Username, From: c.From, TLS: c.TLS}
	if len(c.PasswordEnc) > 0 {
		pw, err := m.sealer.Open(c.PasswordEnc, ad)
		if err != nil {
			return nil, fmt.Errorf("settings: SMTP password: %w", err)
		}
		cfg.Password = string(pw)
	}
	return mailer.New(cfg)
}

// View returns the company's own SMTP configuration, without the password
// (ManagedByEnv is false: the environment configures the platform sender,
// never a company's own server).
func (m *Mail) View(ctx context.Context) (SMTPView, error) {
	c, err := m.load(ctx, m.q)
	if err != nil {
		return SMTPView{}, err
	}
	return SMTPView{Host: c.Host, Port: c.Port, Username: c.Username, From: c.From, TLS: c.TLS,
		PasswordSet: len(c.PasswordEnc) > 0, Enabled: c.Host != ""}, nil
}

// ErrSMTPManagedByEnv is kept for callers of the former panel lock.
var ErrSMTPManagedByEnv = errors.New("settings: e-mail is configured by environment variables")

// Prepare validates a company's own configuration and returns the
// document to store (the stored password is kept when in.Password is nil).
func (m *Mail) Prepare(ctx context.Context, q db.Querier, in SMTPInput) (SMTP, error) {
	prev, err := m.load(ctx, q)
	if err != nil {
		return SMTP{}, err
	}
	return m.prepare(in, prev, smtpAD)
}

func (m *Mail) prepare(in SMTPInput, prev SMTP, ad string) (SMTP, error) {
	out := SMTP{Host: strings.TrimSpace(in.Host), Port: in.Port, Username: strings.TrimSpace(in.Username),
		From: strings.TrimSpace(in.From), TLS: in.TLS}
	if out.Host == "" {
		return SMTP{}, nil // removed
	}
	if in.Password != nil {
		if *in.Password != "" {
			enc, err := m.sealer.Seal([]byte(*in.Password), ad)
			if err != nil {
				return SMTP{}, err
			}
			out.PasswordEnc = enc
		}
	} else {
		out.PasswordEnc = prev.PasswordEnc
	}
	if _, err := m.build(out, ad); err != nil {
		msg := strings.TrimPrefix(err.Error(), "mailer: ")
		msg = strings.NewReplacer("SMTP_FROM: mail:", "sender address:", "SMTP_TLS", "TLS mode").Replace(msg)
		return SMTP{}, fmt.Errorf("%w: %s", ErrInvalid, msg)
	}
	return out, nil
}

// Save stores a prepared document inside q's transaction; call Invalidate
// after it commits.
func (m *Mail) Save(ctx context.Context, q db.Querier, c SMTP, updatedBy *uuid.UUID) error {
	b, _ := json.Marshal(c)
	return q.UpsertSetting(ctx, db.UpsertSettingParams{Key: smtpKey, Value: b, UpdatedBy: updatedBy})
}

// Test sends a test message through the company's own server: the stored
// one, or the unsaved one when in is not nil. A test never falls back.
func (m *Mail) Test(ctx context.Context, in *SMTPInput, to, org string) error {
	var s mailer.Sender
	if in == nil {
		c, err := m.load(ctx, m.q)
		if err != nil {
			return err
		}
		if c.Host == "" {
			return fmt.Errorf("%w: no SMTP server is set up", ErrInvalid)
		}
		if s, err = m.build(c, smtpAD); err != nil {
			return err
		}
	} else {
		c, err := m.Prepare(ctx, m.q, *in)
		if err != nil {
			return err
		}
		if c.Host == "" {
			return fmt.Errorf("%w: e-mail is not configured", ErrInvalid)
		}
		if s, err = m.build(c, smtpAD); err != nil {
			return err
		}
	}
	if err := s.Send(ctx, testMessage(to, org)); err != nil {
		return err
	}
	m.clearFailure(ctx)
	return nil
}

func testMessage(to, org string) mailer.Message {
	return mailer.Message{
		To:      to,
		Subject: org + ": test e-mail",
		Body:    "This is a test message from " + org + ". E-mail delivery works.\n",
	}
}

// ── The platform sender (admin panel; platform transactions) ───────────────

// PlatformSMTPView is the platform sender for the admin panel.
type PlatformSMTPView struct {
	// Source is "panel", "env" or "none".
	Source string
	SMTPView
	ReplyTo string
	// The environment's sender (SMTP_*).
	EnvConfigured    bool
	EnvHost, EnvFrom string
}

// PlatformView describes the platform sender in effect.
func (m *Mail) PlatformView(ctx context.Context, q db.Querier) (PlatformSMTPView, error) {
	c, err := m.loadPlatform(ctx, q)
	if err != nil {
		return PlatformSMTPView{}, err
	}
	out := PlatformSMTPView{Source: "none", ReplyTo: PlatformReplyTo(ctx, q)}
	if m.env != nil {
		out.EnvConfigured, out.EnvHost, out.EnvFrom = true, m.env.Host, m.env.From
	}
	switch {
	case c.Host != "":
		out.Source = "panel"
		out.SMTPView = SMTPView{Host: c.Host, Port: c.Port, Username: c.Username, From: c.From, TLS: c.TLS,
			PasswordSet: len(c.PasswordEnc) > 0, Enabled: true}
	case m.env != nil:
		out.Source = "env"
		out.SMTPView = SMTPView{Host: m.env.Host, Port: m.env.Port, Username: m.env.Username, From: m.env.From, TLS: m.env.TLS,
			PasswordSet: m.env.Password != "", ManagedByEnv: true, Enabled: true}
	}
	return out, nil
}

// SavePlatform validates and stores the platform sender with q (a platform
// transaction); an empty host removes it. Call Invalidate after commit.
func (m *Mail) SavePlatform(ctx context.Context, q db.Querier, in SMTPInput) (SMTP, error) {
	prev, err := m.loadPlatform(ctx, q)
	if err != nil {
		return SMTP{}, err
	}
	c, err := m.prepare(in, prev, platformSMTPAD)
	if err != nil {
		return SMTP{}, err
	}
	if c.Host == "" {
		return c, q.DeletePlatformSetting(ctx, PlatformSMTPKey)
	}
	b, _ := json.Marshal(c)
	return c, q.UpsertPlatformSetting(ctx, db.UpsertPlatformSettingParams{Key: PlatformSMTPKey, Value: b})
}

// TestPlatform sends a test message through the platform sender in effect,
// or through an unsaved configuration.
func (m *Mail) TestPlatform(ctx context.Context, q db.Querier, in *SMTPInput, to string) error {
	var s mailer.Sender
	if in == nil {
		m.Invalidate()
		s = m.Platform(ctx)
	} else {
		prev, err := m.loadPlatform(ctx, q)
		if err != nil {
			return err
		}
		c, err := m.prepare(*in, prev, platformSMTPAD)
		if err != nil {
			return err
		}
		if c.Host == "" {
			return fmt.Errorf("%w: e-mail is not configured", ErrInvalid)
		}
		if s, err = m.build(c, platformSMTPAD); err != nil {
			return err
		}
	}
	if !s.Enabled(ctx) {
		return fmt.Errorf("%w: e-mail is not configured", ErrInvalid)
	}
	return s.Send(ctx, testMessage(to, "Messora"))
}
