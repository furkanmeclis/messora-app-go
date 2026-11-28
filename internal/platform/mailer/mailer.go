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
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// Message is a transactional e-mail. Body is the plain-text part and is always
// sent; when HTML is set the message becomes multipart/alternative with the
// text part first, so clients that cannot (or will not) render HTML still get
// a readable message.
type Message struct {
	To      string
	Subject string
	Body    string
	// HTML is the optional HTML alternative of Body.
	HTML string
	// ReplyTo is an optional RFC 5322 address for the Reply-To header.
	ReplyTo string
	// FromName optionally replaces the display name of the sender (the
	// address stays the sender's): the platform sender sends a company's
	// mail with the company's name.
	FromName string
	// ListUnsubscribe is an optional https or mailto URL for the
	// List-Unsubscribe header. An https URL also gets RFC 8058 one-click
	// (List-Unsubscribe-Post). Only for mail the recipient may opt out of.
	ListUnsubscribe string
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
	if err := validateOptional(m); err != nil {
		return err
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
	raw, err := s.render(to, m, time.Now())
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("mailer: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close data: %w", err)
	}
	return c.Quit()
}

// validateOptional checks the optional headers for injection and syntax.
func validateOptional(m Message) error {
	if strings.ContainsAny(m.FromName, "\r\n") || len(m.FromName) > 200 {
		return errors.New("mailer: invalid sender name")
	}
	if m.ReplyTo != "" {
		if strings.ContainsAny(m.ReplyTo, "\r\n") {
			return errors.New("mailer: reply-to contains a line break")
		}
		if _, err := mail.ParseAddress(m.ReplyTo); err != nil {
			return fmt.Errorf("mailer: invalid reply-to: %w", err)
		}
	}
	if u := m.ListUnsubscribe; u != "" {
		if strings.ContainsAny(u, "\r\n<> ,") ||
			(!strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "mailto:")) {
			return errors.New("mailer: list-unsubscribe must be a single https or mailto URL")
		}
	}
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// encodeSubject applies RFC 2047 Q-encoding (only when needed) and folds the
// encoded-words onto continuation lines so header lines stay short.
func encodeSubject(subject string) string {
	enc := mime.QEncoding.Encode("utf-8", subject)
	if enc == subject {
		return subject
	}
	return strings.ReplaceAll(enc, "?= =?", "?=\r\n =?")
}

// render builds the full RFC 5322 message. Bodies are quoted-printable so
// every line stays 7-bit and short regardless of content.
func (s *SMTP) render(to *mail.Address, m Message, now time.Time) ([]byte, error) {
	if err := validateOptional(m); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	domain := s.from.Address[strings.LastIndex(s.from.Address, "@")+1:]
	from := *s.from
	if name := strings.TrimSpace(m.FromName); name != "" {
		from.Name = name
	}
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	if m.ReplyTo != "" {
		rt, _ := mail.ParseAddress(m.ReplyTo) // validated above
		fmt.Fprintf(&b, "Reply-To: %s\r\n", rt.String())
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", encodeSubject(m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", randomHex(12), domain)
	if m.ListUnsubscribe != "" {
		fmt.Fprintf(&b, "List-Unsubscribe: <%s>\r\n", m.ListUnsubscribe)
		if strings.HasPrefix(m.ListUnsubscribe, "https://") {
			b.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
		}
	}
	b.WriteString("MIME-Version: 1.0\r\nAuto-Submitted: auto-generated\r\n")

	if m.HTML == "" {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQP(&b, m.Body); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}

	var parts bytes.Buffer
	mw := multipart.NewWriter(&parts)
	if err := mw.SetBoundary("messora-" + randomHex(16)); err != nil {
		return nil, fmt.Errorf("mailer: boundary: %w", err)
	}
	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=utf-8", m.Body},
		{"text/html; charset=utf-8", m.HTML},
	} {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", part.ctype)
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		pw, err := mw.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("mailer: part: %w", err)
		}
		if err := writeQP(pw, part.body); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("mailer: multipart: %w", err)
	}
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", mw.Boundary())
	b.Write(parts.Bytes())
	b.WriteString("\r\n")
	return b.Bytes(), nil
}

// writeQP writes body as quoted-printable with CRLF line endings. net/smtp's
// DATA writer performs dot-stuffing on the encoded lines.
func writeQP(w io.Writer, body string) error {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	qp := quotedprintable.NewWriter(w)
	if _, err := io.WriteString(qp, body); err != nil {
		return fmt.Errorf("mailer: encode body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("mailer: encode body: %w", err)
	}
	return nil
}
