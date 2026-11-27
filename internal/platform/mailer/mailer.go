// Package mailer sends transactional e-mail over SMTP.
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Message is a plain-text e-mail.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
	// Enabled reports whether mail can be sent for ctx (its tenant).
	Enabled(ctx context.Context) bool
}

// Config holds SMTP settings.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string // "Messora <no-reply@example.com>"
	// TLS: "starttls" (default, port 587), "tls" (implicit, port 465) or
	// "none" (only for local test servers such as Mailpit).
	TLS string
}

// Disabled is used when SMTP is not configured.
type Disabled struct{}

// Send always fails.
func (Disabled) Send(context.Context, Message) error {
	return errors.New("mailer: e-mail is not configured")
}

// Enabled is false.
func (Disabled) Enabled(context.Context) bool { return false }

// SMTP sends via an SMTP server.
type SMTP struct {
	cfg  Config
	from *mail.Address
}

// New validates cfg and returns a Sender. An empty host yields Disabled.
func New(cfg Config) (Sender, error) {
	if cfg.Host == "" {
		return Disabled{}, nil
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("mailer: SMTP_FROM: %w", err)
	}
	switch cfg.TLS {
	case "", "starttls":
		cfg.TLS = "starttls"
	case "tls", "none":
	default:
		return nil, fmt.Errorf("mailer: SMTP_TLS must be starttls, tls or none")
	}
	if cfg.Port == 0 {
		cfg.Port = map[string]int{"starttls": 587, "tls": 465, "none": 25}[cfg.TLS]
	}
	return &SMTP{cfg: cfg, from: from}, nil
}

// Enabled is true.
func (s *SMTP) Enabled(context.Context) bool { return true }

// Send delivers m, honouring ctx's deadline (default 15 s).
func (s *SMTP) Send(ctx context.Context, m Message) error {
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("mailer: invalid recipient: %w", err)
	}
	if strings.ContainsAny(m.Subject, "\r\n") {
		return errors.New("mailer: subject contains a line break")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))
	dialer := &net.Dialer{Deadline: deadline}
	tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	if s.cfg.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mailer: connect: %w", err)
	}
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mailer: greeting: %w", err)
	}
	defer c.Close()
	if s.cfg.TLS == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("mailer: server does not support STARTTLS")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("mailer: starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("mailer: auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mailer: RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	if _, err := w.Write(s.render(to, m)); err != nil {
		return fmt.Errorf("mailer: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close data: %w", err)
	}
	return c.Quit()
}

func (s *SMTP) render(to *mail.Address, m Message) []byte {
	var b bytes.Buffer
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := s.from.Address[strings.LastIndex(s.from.Address, "@")+1:]
	fmt.Fprintf(&b, "From: %s\r\n", s.from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\nAuto-Submitted: auto-generated\r\n\r\n")
	body := strings.ReplaceAll(m.Body, "\r\n", "\n")
	// net/smtp's DATA writer performs dot-stuffing; lines are written as-is.
	for line := range strings.SplitSeq(body, "\n") {
		b.WriteString(line + "\r\n")
	}
	return b.Bytes()
}
