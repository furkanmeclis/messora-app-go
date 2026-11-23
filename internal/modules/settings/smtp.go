package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
)

const smtpKey = "smtp"

// SMTP is the outgoing e-mail configuration chosen in the panel. The
// password is stored encrypted and never returned.
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

// SMTPInput changes the panel configuration. A nil Password keeps the
// stored one; an empty Host turns e-mail off.
type SMTPInput struct {
	Host     string
	Port     int
	Username string
	Password *string
	From     string
	TLS      string
}

// Mail is the e-mail sender for the whole application. SMTP_HOST in the
// environment wins; otherwise the panel configuration is used, so a change
// there applies without a restart.
type Mail struct {
	q      db.Querier
	sealer *crypto.Sealer
	env    *mailer.Config // nil unless SMTP_HOST is set
	envErr error
	ttl    time.Duration

	mu      sync.Mutex
	sender  mailer.Sender
	expires time.Time
}

const smtpAD = "settings:smtp"

// NewMail returns the application sender. env is the environment
// configuration (an empty Host means "not set").
func NewMail(q db.Querier, sealer *crypto.Sealer, env mailer.Config) (*Mail, error) {
	m := &Mail{q: q, sealer: sealer, ttl: 15 * time.Second}
	if env.Host != "" {
		s, err := mailer.New(env)
		if err != nil {
			return nil, err
		}
		m.env, m.sender, m.expires = &env, s, time.Now().Add(100*365*24*time.Hour)
	}
	return m, nil
}

// ManagedByEnv reports whether SMTP_HOST is set.
func (m *Mail) ManagedByEnv() bool { return m.env != nil }

func (m *Mail) current(ctx context.Context) mailer.Sender {
	m.mu.Lock()
	if m.sender != nil && time.Now().Before(m.expires) {
		s := m.sender
		m.mu.Unlock()
		return s
	}
	m.mu.Unlock()
	var s mailer.Sender = mailer.Disabled{}
	if cfg, err := m.load(ctx, m.q); err == nil && cfg.Host != "" {
		if built, err := m.build(cfg); err == nil {
			s = built
		}
	}
	m.mu.Lock()
	m.sender, m.expires = s, time.Now().Add(m.ttl)
	m.mu.Unlock()
	return s
}

// Send implements mailer.Sender.
func (m *Mail) Send(ctx context.Context, msg mailer.Message) error {
	return m.current(ctx).Send(ctx, msg)
}

// Enabled implements mailer.Sender.
func (m *Mail) Enabled() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return m.current(ctx).Enabled()
}

// Invalidate drops the cached sender after a change commits.
func (m *Mail) Invalidate() {
	if m.env != nil {
		return
	}
	m.mu.Lock()
	m.sender = nil
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

func (m *Mail) build(c SMTP) (mailer.Sender, error) {
	cfg := mailer.Config{Host: c.Host, Port: c.Port, Username: c.Username, From: c.From, TLS: c.TLS}
	if len(c.PasswordEnc) > 0 {
		pw, err := m.sealer.Open(c.PasswordEnc, smtpAD)
		if err != nil {
			return nil, fmt.Errorf("settings: SMTP password: %w", err)
		}
		cfg.Password = string(pw)
	}
	return mailer.New(cfg)
}

// View returns the configuration in effect, without the password.
func (m *Mail) View(ctx context.Context) (SMTPView, error) {
	if m.env != nil {
		return SMTPView{Host: m.env.Host, Port: m.env.Port, Username: m.env.Username, From: m.env.From,
			TLS: m.env.TLS, PasswordSet: m.env.Password != "", ManagedByEnv: true, Enabled: true}, nil
	}
	c, err := m.load(ctx, m.q)
	if err != nil {
		return SMTPView{}, err
	}
	return SMTPView{Host: c.Host, Port: c.Port, Username: c.Username, From: c.From, TLS: c.TLS,
		PasswordSet: len(c.PasswordEnc) > 0, Enabled: c.Host != ""}, nil
}

// ErrSMTPManagedByEnv is returned for changes while SMTP_HOST is set.
var ErrSMTPManagedByEnv = errors.New("settings: e-mail is configured by environment variables")

// Prepare validates in and returns the document to store (the stored
// password is kept when in.Password is nil).
func (m *Mail) Prepare(ctx context.Context, q db.Querier, in SMTPInput) (SMTP, error) {
	if m.env != nil {
		return SMTP{}, ErrSMTPManagedByEnv
	}
	out := SMTP{Host: strings.TrimSpace(in.Host), Port: in.Port, Username: strings.TrimSpace(in.Username),
		From: strings.TrimSpace(in.From), TLS: in.TLS}
	if out.Host == "" {
		return SMTP{}, nil // e-mail off
	}
	if in.Password != nil {
		if *in.Password != "" {
			enc, err := m.sealer.Seal([]byte(*in.Password), smtpAD)
			if err != nil {
				return SMTP{}, err
			}
			out.PasswordEnc = enc
		}
	} else {
		prev, err := m.load(ctx, q)
		if err != nil {
			return SMTP{}, err
		}
		out.PasswordEnc = prev.PasswordEnc
	}
	if _, err := m.build(out); err != nil {
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

// Test sends a test message with the configuration in effect, or with the
// unsaved one when in is not nil.
func (m *Mail) Test(ctx context.Context, in *SMTPInput, to, org string) error {
	var s mailer.Sender
	if in == nil || m.env != nil {
		s = m.current(ctx)
	} else {
		c, err := m.Prepare(ctx, m.q, *in)
		if err != nil {
			return err
		}
		if c.Host == "" {
			s = mailer.Disabled{}
		} else if s, err = m.build(c); err != nil {
			return err
		}
	}
	if !s.Enabled() {
		return fmt.Errorf("%w: e-mail is not configured", ErrInvalid)
	}
	return s.Send(ctx, mailer.Message{
		To:      to,
		Subject: org + ": test e-mail",
		Body:    "This is a test message from " + org + ". E-mail delivery works.\n",
	})
}
