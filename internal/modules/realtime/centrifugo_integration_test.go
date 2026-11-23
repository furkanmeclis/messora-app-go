//go:build integration

package realtime_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/provision"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// CentrifugoImage must match docker-compose.yml.
const CentrifugoImage = "centrifugo/centrifugo:v6.10.3"

const (
	cfAPIKey = "centrifugo-api-key-for-tests-0123"   // gitleaks:allow (test fixture)
	cfHMAC   = "centrifugo-hmac-secret-for-tests-01" // gitleaks:allow (test fixture)
	origin   = "https://messora.test"
)

// startCentrifugo runs Centrifugo with exactly the configuration
// `messora init` generates.
func startCentrifugo(t *testing.T) (addr string, logs func() string) {
	t.Helper()
	cfg, err := provision.CentrifugoConfig(cfAPIKey, cfHMAC, origin)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithCmd("centrifugo", "--config", "/etc/centrifugo/config.json"),
		testcontainers.WithFiles(testcontainers.ContainerFile{Reader: bytes.NewReader(cfg), ContainerFilePath: "/etc/centrifugo/config.json", FileMode: 0o644}),
		testcontainers.WithExposedPorts("8000/tcp"),
		testcontainers.WithWaitStrategyAndDeadline(90*time.Second, wait.ForHTTP("/health").WithPort("8000/tcp")),
	}
	c, err := testcontainers.Run(ctx, CentrifugoImage, opts...)
	if err != nil {
		// Docker on developer machines is sometimes slow to start a
		// container; one retry keeps the test from flaking.
		t.Logf("start centrifugo: %v; retrying once", err)
		_ = testcontainers.TerminateContainer(c)
		if c, err = testcontainers.Run(ctx, CentrifugoImage, opts...); err != nil {
			t.Fatalf("start centrifugo: %v", err)
		}
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	ep, err := c.PortEndpoint(ctx, "8000/tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	return ep, func() string {
		r, err := c.Logs(context.Background())
		if err != nil {
			return err.Error()
		}
		defer r.Close()
		b, _ := io.ReadAll(r)
		return string(b)
	}
}

// ws is a minimal RFC 6455 client speaking Centrifugo's JSON protocol: just
// enough to connect, subscribe and receive publications.
type ws struct {
	t      *testing.T
	conn   net.Conn
	r      *bufio.Reader
	nextID int
	pushes []map[string]any
	closed string // close code and reason, once closed by the server
}

func dial(t *testing.T, addr string) *ws {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	_, _ = fmt.Fprintf(conn, "GET /connection/websocket HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nOrigin: %s\r\n\r\n", addr, base64.StdEncoding.EncodeToString(key), origin)
	r := bufio.NewReader(conn)
	res, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A 101 response has no body; the connection itself carries the frames.
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("websocket handshake: %s", res.Status)
	}
	return &ws{t: t, conn: conn, r: r}
}

func (w *ws) writeFrame(payload []byte) {
	w.t.Helper()
	var b bytes.Buffer
	b.WriteByte(0x81) // FIN + text
	switch n := len(payload); {
	case n < 126:
		b.WriteByte(0x80 | byte(n))
	case n <= 0xFFFF:
		b.WriteByte(0x80 | 126)
		_ = binary.Write(&b, binary.BigEndian, uint16(n))
	default:
		b.WriteByte(0x80 | 127)
		_ = binary.Write(&b, binary.BigEndian, uint64(n))
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	b.Write(mask)
	for i, c := range payload {
		b.WriteByte(c ^ mask[i%4])
	}
	if _, err := w.conn.Write(b.Bytes()); err != nil {
		w.t.Fatal(err)
	}
}

// readFrame returns one text frame; a close frame ends the connection.
func (w *ws) readFrame(deadline time.Time) ([]byte, error) {
	_ = w.conn.SetReadDeadline(deadline)
	var h [2]byte
	if _, err := io.ReadFull(w.r, h[:]); err != nil {
		return nil, err
	}
	n := uint64(h[1] & 0x7F)
	switch n {
	case 126:
		var l uint16
		if err := binary.Read(w.r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		n = uint64(l)
	case 127:
		if err := binary.Read(w.r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(w.r, p); err != nil {
		return nil, err
	}
	if op := h[0] & 0x0F; op == 0x8 {
		code := 0
		if len(p) >= 2 {
			code = int(binary.BigEndian.Uint16(p))
			p = p[2:]
		}
		w.closed = fmt.Sprintf("%d %s", code, p)
		return nil, errors.New("closed: " + w.closed)
	}
	return p, nil
}

// call sends a command and waits for its reply, collecting pushes and
// answering pings on the way.
func (w *ws) call(method string, params any) (map[string]any, error) {
	w.t.Helper()
	w.nextID++
	id := w.nextID
	cmd, _ := json.Marshal(map[string]any{"id": id, method: params})
	w.writeFrame(cmd)
	deadline := time.Now().Add(10 * time.Second)
	for {
		frame, err := w.readFrame(deadline)
		if err != nil {
			return nil, err
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(frame)), "\n") {
			var m map[string]any
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				w.t.Fatalf("bad frame %q", line)
			}
			switch {
			case len(m) == 0:
				w.writeFrame([]byte("{}")) // ping → pong
			case m["push"] != nil:
				w.pushes = append(w.pushes, m["push"].(map[string]any))
			case m["id"] == float64(id):
				return m, nil
			}
		}
	}
}

// awaitPush waits for a publication in channel.
func (w *ws) awaitPush(channel string) map[string]any {
	w.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		for i, p := range w.pushes {
			if p["channel"] == channel && p["pub"] != nil {
				w.pushes = slices.Delete(w.pushes, i, i+1)
				return p["pub"].(map[string]any)
			}
		}
		frame, err := w.readFrame(deadline)
		if err != nil {
			w.t.Fatalf("waiting for a publication in %s: %v", channel, err)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(frame)), "\n") {
			var m map[string]any
			_ = json.Unmarshal([]byte(line), &m)
			if len(m) == 0 {
				w.writeFrame([]byte("{}"))
			} else if m["push"] != nil {
				w.pushes = append(w.pushes, m["push"].(map[string]any))
			}
		}
	}
}

func errorCode(reply map[string]any) float64 {
	e, _ := reply["error"].(map[string]any)
	c, _ := e["code"].(float64)
	return c
}

func TestCentrifugoAcceptsIssuedTokensAndRefusesOthers(t *testing.T) {
	addr, logs := startCentrifugo(t)
	defer func() {
		if t.Failed() || os.Getenv("TEST_LOG") != "" {
			t.Logf("centrifugo logs:\n%s", logs())
		}
	}()

	agent := &principal.Principal{Kind: principal.KindUser, ID: uuid.Must(uuid.NewV7()),
		Permissions: []string{"contacts.read", "conversations.read", "conversations.reply"}}
	owner := &principal.Principal{Kind: principal.KindUser, ID: uuid.Must(uuid.NewV7()),
		Permissions: []string{"campaigns.read", "conversations.read", "conversations.read_all"}}
	iss, err := realtime.NewIssuer(cfHMAC)
	if err != nil {
		t.Fatal(err)
	}

	// A connection token issued by the API is accepted and the personal
	// channel is subscribed server-side.
	conn, _ := iss.Connection(agent)
	c := dial(t, addr)
	reply, err := c.call("connect", map[string]any{"token": conn.Token, "name": "messora-test"})
	if err != nil || reply["connect"] == nil {
		t.Fatalf("connect: %v %v", reply, err)
	}
	subs, _ := reply["connect"].(map[string]any)["subs"].(map[string]any)
	if subs[realtime.PersonalChannel(agent.ID)] == nil || subs[realtime.NotificationsChannel] == nil {
		t.Fatalf("server-side subscriptions %v", subs)
	}

	// A permitted channel with its subscription token: recoverable history.
	inbox, err := iss.Subscription(agent, realtime.InboxChannel)
	if err != nil {
		t.Fatal(err)
	}
	reply, err = c.call("subscribe", map[string]any{"channel": realtime.InboxChannel, "token": inbox.Token})
	if err != nil || reply["subscribe"] == nil {
		t.Fatalf("subscribe inbox: %v %v", reply, err)
	}
	if r := reply["subscribe"].(map[string]any); r["recoverable"] != true || r["epoch"] == nil {
		t.Fatalf("inbox must be recoverable: %v", r)
	}

	// No token: refused (subscribing without a token is disabled).
	reply, err = c.call("subscribe", map[string]any{"channel": realtime.CampaignsChannel})
	if err != nil || errorCode(reply) != 103 {
		t.Fatalf("subscribe without token: want permission denied (103), got %v %v", reply, err)
	}
	// The API does not issue the agent a campaigns token.
	if _, err := iss.Subscription(agent, realtime.CampaignsChannel); !errors.Is(err, realtime.ErrChannelForbidden) {
		t.Fatalf("agent got a campaigns token: %v", err)
	}

	// Publications relayed from the outbox reach the subscriber once, even
	// when the relay repeats a publish.
	cf, err := centrifugo.New(config.CentrifugoConfig{APIURL: "http://" + addr + "/api", APIKey: cfAPIKey})
	if err != nil {
		t.Fatal(err)
	}
	pub := realtime.Publisher(cf)
	ctx := context.Background()
	first := outbox.Event{ID: 1, Topic: realtime.TopicPrefix + realtime.InboxChannel, Payload: []byte(`{"type":"message.created","v":1,"n":1}`)}
	for range 2 {
		if err := pub.Publish(ctx, first); err != nil {
			t.Fatal(err)
		}
	}
	second := outbox.Event{ID: 2, Topic: realtime.TopicPrefix + realtime.InboxChannel, Payload: []byte(`{"type":"message.status","v":1,"n":2}`)}
	if err := pub.Publish(ctx, second); err != nil {
		t.Fatal(err)
	}
	for _, want := range []float64{1, 2} {
		p := c.awaitPush(realtime.InboxChannel)
		if p["data"].(map[string]any)["n"] != want {
			t.Fatalf("publication %v, want n=%v (duplicates must be dropped)", p, want)
		}
	}
	if err := pub.Publish(ctx, outbox.Event{ID: 3, Topic: realtime.TopicPrefix + realtime.PersonalChannel(agent.ID), Payload: []byte(`{"type":"notice","v":1}`)}); err != nil {
		t.Fatal(err)
	}
	if p := c.awaitPush(realtime.PersonalChannel(agent.ID)); p["data"].(map[string]any)["type"] != "notice" {
		t.Fatalf("personal publication %v", p)
	}

	// A token issued for another channel or another user does not open a
	// forbidden channel.
	ownerCampaigns, _ := iss.Subscription(owner, realtime.CampaignsChannel)
	reply, err = c.call("subscribe", map[string]any{"channel": realtime.CampaignsChannel, "token": ownerCampaigns.Token})
	if err == nil && reply["subscribe"] != nil {
		t.Fatal("another user's subscription token was accepted")
	}
	c2 := dial(t, addr)
	if _, err := c2.call("connect", map[string]any{"token": conn.Token}); err != nil {
		t.Fatal(err)
	}
	reply, err = c2.call("subscribe", map[string]any{"channel": realtime.CampaignsChannel, "token": inbox.Token})
	if err == nil && reply["subscribe"] != nil {
		t.Fatal("an inbox token opened the campaigns channel")
	}

	// Tokens signed with another secret are refused at connect.
	forged, _ := realtime.NewIssuer("not-the-centrifugo-secret-0123456789") // gitleaks:allow (test fixture)
	bad, _ := forged.Connection(owner)
	c3 := dial(t, addr)
	reply, err = c3.call("connect", map[string]any{"token": bad.Token})
	if err == nil && reply["connect"] != nil {
		t.Fatal("a forged connection token was accepted")
	}
	if !strings.Contains(c3.closed, "3500") && errorCode(reply) == 0 {
		t.Fatalf("forged token: want an invalid-token disconnect, got %q %v %v", c3.closed, reply, err)
	}

	// The generated configuration contains only keys Centrifugo knows.
	if l := logs(); strings.Contains(l, "unknown key") {
		t.Fatalf("centrifugo reports unknown configuration keys:\n%s", l)
	}
}
