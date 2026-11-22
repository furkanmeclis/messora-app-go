package mailer

import (
	"bufio"
	"context"
	"fmt"
	"net"
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
