package mailer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

// fakeSMTP accepts one message and returns its DATA section.
func fakeSMTP(t *testing.T) (addr string, got <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan string, 1)
	go func() {
		defer ln.Close()
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		w := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
		w("220 fake ESMTP")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					w("250 queued")
					ch <- data.String()
					continue
				}
				data.WriteString(line)
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				w("250 fake")
			case strings.HasPrefix(cmd, "DATA"):
				inData = true
				w("354 go ahead")
			case strings.HasPrefix(cmd, "QUIT"):
				w("221 bye")
				return
			default:
				w("250 ok")
			}
		}
	}()
	return ln.Addr().String(), ch
}

func TestSendPlainSMTP(t *testing.T) {
	addr, got := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	var p int
	_, _ = fmtSscan(port, &p)
	s, err := New(Config{Host: host, Port: p, From: "Messora <no-reply@example.com>", TLS: "none"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Send(ctx, Message{To: "ali@example.com", Subject: "Şifre sıfırlama", Body: "Merhaba\n.line starting with dot"}); err != nil {
		t.Fatal(err)
	}
	data := <-got
	for _, want := range []string{"From: \"Messora\" <no-reply@example.com>", "To: <ali@example.com>", "Subject: =?utf-8?q?", "Content-Type: text/plain; charset=utf-8", "Merhaba\r\n..line starting with dot\r\n"} {
		if !strings.Contains(data, want) {
			t.Errorf("missing %q in:\n%s", want, data)
		}
	}
}

func TestRejectsHeaderInjectionAndBadConfig(t *testing.T) {
	s, _ := New(Config{Host: "127.0.0.1", Port: 1, From: "a@example.com", TLS: "none"})
	if err := s.Send(context.Background(), Message{To: "b@example.com", Subject: "x\r\nBcc: evil@example.com"}); err == nil {
		t.Fatal("header injection accepted")
	}
	if err := s.Send(context.Background(), Message{To: "not an address", Subject: "x"}); err == nil {
		t.Fatal("bad recipient accepted")
	}
	if _, err := New(Config{Host: "h", From: "bad"}); err == nil {
		t.Fatal("bad from accepted")
	}
	if _, err := New(Config{Host: "h", From: "a@b.co", TLS: "ssl3"}); err == nil {
		t.Fatal("bad tls mode accepted")
	}
	d, _ := New(Config{})
	if d.Enabled() || d.Send(context.Background(), Message{}) == nil {
		t.Fatal("empty config must be disabled")
	}
}

func fmtSscan(s string, p *int) (int, error) { return fmt.Sscan(s, p) }

func testSMTP(t *testing.T) *SMTP {
	t.Helper()
	s, err := New(Config{Host: "127.0.0.1", From: "Messora Bildirim <no-reply@messora.tech>", TLS: "none"})
	if err != nil {
		t.Fatal(err)
	}
	return s.(*SMTP)
}

// parse reads a rendered message back the way a mail client would.
func parse(t *testing.T, raw []byte) (*mail.Message, string) {
	t.Helper()
	for i, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("line %d is %d bytes long", i, len(line))
		}
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatal(err)
	}
	return msg, subject
}

const turkish = "Şifreniz değişti — İğdır'da ÇAY içen öğrenci ışığı gördü: ğüşıöçĞÜŞİÖÇ"

func TestMultipartRoundTripTurkish(t *testing.T) {
	s := testSMTP(t)
	to, _ := mail.ParseAddress("Ayşe Yılmaz <ayse@example.com>")
	long := strings.Repeat("Güvenliğiniz için bağlantı 30 dakika geçerlidir. ", 6)
	m := Message{
		To:              to.String(),
		Subject:         turkish,
		Body:            "Merhaba Ayşe,\n" + long + "\n.nokta ile başlayan satır\n",
		HTML:            `<p style="color:#0B1220">Merhaba <strong>Ayşe</strong>, ` + long + `</p>`,
		ReplyTo:         "Destek <destek@messora.tech>",
		ListUnsubscribe: "https://app.messora.tech/u/abc",
	}
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	raw, err := s.render(to, m, now)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range raw {
		if b > 127 {
			t.Fatalf("non-ASCII byte at %d: rendered message must be 7-bit", i)
		}
	}
	msg, subject := parse(t, raw)
	if subject != turkish {
		t.Fatalf("subject round-trip: got %q", subject)
	}
	h := msg.Header
	if h.Get("Date") != "Fri, 27 Nov 2026 10:00:00 +0000" {
		t.Errorf("date: %q", h.Get("Date"))
	}
	if id := h.Get("Message-ID"); !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@messora.tech>") {
		t.Errorf("message-id: %q", id)
	}
	if h.Get("Reply-To") != `"Destek" <destek@messora.tech>` {
		t.Errorf("reply-to: %q", h.Get("Reply-To"))
	}
	if h.Get("List-Unsubscribe") != "<https://app.messora.tech/u/abc>" || h.Get("List-Unsubscribe-Post") != "List-Unsubscribe=One-Click" {
		t.Errorf("list-unsubscribe: %q / %q", h.Get("List-Unsubscribe"), h.Get("List-Unsubscribe-Post"))
	}
	mt, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content-type: %q %v", h.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types, bodies []string
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p) // multipart decodes quoted-printable transparently
		types = append(types, p.Header.Get("Content-Type"))
		bodies = append(bodies, strings.ReplaceAll(string(b), "\r\n", "\n"))
	}
	if len(types) != 2 || types[0] != "text/plain; charset=utf-8" || types[1] != "text/html; charset=utf-8" {
		t.Fatalf("parts: %v", types)
	}
	if bodies[0] != m.Body {
		t.Errorf("text part round-trip:\n%q\n%q", bodies[0], m.Body)
	}
	if bodies[1] != m.HTML+"\n" {
		t.Errorf("html part round-trip:\n%q", bodies[1])
	}
}

func TestPlainTextStaysSinglePart(t *testing.T) {
	s := testSMTP(t)
	to, _ := mail.ParseAddress("a@example.com")
	raw, err := s.render(to, Message{To: "a@example.com", Subject: "Hello", Body: "Çok güzel"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	msg, subject := parse(t, raw)
	if subject != "Hello" || msg.Header.Get("Subject") != "Hello" {
		t.Errorf("ASCII subject must not be encoded: %q", msg.Header.Get("Subject"))
	}
	if msg.Header.Get("Content-Type") != "text/plain; charset=utf-8" || msg.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
		t.Errorf("headers: %v", msg.Header)
	}
	if msg.Header.Get("List-Unsubscribe") != "" || msg.Header.Get("Reply-To") != "" {
		t.Error("optional headers must be absent")
	}
	b, _ := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if string(b) != "Çok güzel\r\n" {
		t.Errorf("body: %q", b)
	}
}

func TestRejectsBadOptionalHeaders(t *testing.T) {
	s := testSMTP(t)
	for _, m := range []Message{
		{To: "a@example.com", ReplyTo: "x@example.com\r\nBcc: evil@example.com"},
		{To: "a@example.com", ReplyTo: "not an address"},
		{To: "a@example.com", ListUnsubscribe: "http://insecure.example.com"},
		{To: "a@example.com", ListUnsubscribe: "https://a.example.com>\r\nBcc: e@x.co"},
		{To: "a@example.com", ListUnsubscribe: "javascript:alert(1)"},
	} {
		if err := s.Send(context.Background(), m); err == nil {
			t.Errorf("accepted %+v", m)
		}
	}
	if err := validateOptional(Message{ListUnsubscribe: "mailto:unsubscribe@messora.tech"}); err != nil {
		t.Errorf("mailto rejected: %v", err)
	}
}
