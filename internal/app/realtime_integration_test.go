//go:build integration

package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

const testHMACSecret = "centrifugo-hmac-secret-for-tests" // gitleaks:allow (test fixture)

// jwtClaims verifies an HS256 token with the harness secret.
func jwtClaims(t *testing.T, tok string) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", tok)
	}
	mac := hmac.New(sha256.New, []byte(testHMACSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != parts[2] {
		t.Fatal("token is not signed with CENTRIFUGO_HMAC_SECRET")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var c map[string]any
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// inviteAgent creates a user with the agent role and returns its session.
func (h *harness) inviteAgent(owner string) (token, id string) {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/invitations", token: owner, body: map[string]any{"email": "agent@example.com", "role": "agent"}})
	h.expect(r, 201, "")
	link := r.data()["accept_url"].(string)
	r = h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{
		"token": link[strings.Index(link, "token=")+6:], "name": "Ali Agent", "password": "Correct-horse-staple-9"}}) // gitleaks:allow (test fixture)
	h.expect(r, 201, "")
	return r.data()["session_token"].(string), r.data()["user"].(map[string]any)["id"].(string)
}

type rtEvent struct {
	channel string
	body    map[string]any
}

// rtEvents lists the realtime events in the outbox, oldest first.
func (h *harness) rtEvents() []rtEvent {
	h.t.Helper()
	rows, err := h.pool.Query(h.ctx(), "SELECT topic, payload FROM outbox_events WHERE topic LIKE 'rt:%' ORDER BY id")
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []rtEvent
	for rows.Next() {
		var topic string
		var payload []byte
		if err := rows.Scan(&topic, &payload); err != nil {
			h.t.Fatal(err)
		}
		e := rtEvent{channel: strings.TrimPrefix(topic, realtime.TopicPrefix)}
		if err := json.Unmarshal(payload, &e.body); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

func TestRealtimeTokensAndChannelPermissions(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	agent, agentID := h.inviteAgent(owner)
	r := h.do(call{method: "GET", path: "/api/v1/me", token: owner})
	ownerID := r.data()["user"].(map[string]any)["id"].(string)

	// Connection tokens: signed with the shared secret, the user as subject,
	// short-lived, with the personal channel subscribed server-side.
	r = h.do(call{method: "POST", path: "/api/v1/realtime/connection-token", token: agent})
	h.expect(r, 200, "")
	c := jwtClaims(t, r.data()["token"].(string))
	if c["sub"] != h.tenantID.String()+":"+agentID {
		t.Fatalf("sub %v, want %s", c["sub"], agentID)
	}
	if r.data()["channel_prefix"] != "tenant:"+h.tenantID.String()+":" {
		t.Fatalf("channel prefix %v", r.data()["channel_prefix"])
	}
	if exp := time.Unix(int64(c["exp"].(float64)), 0); time.Until(exp) > realtime.TokenTTL || time.Until(exp) < time.Minute {
		t.Fatalf("exp %v", exp)
	}
	if subs := c["subs"].(map[string]any); subs[h.wire("personal:#"+agentID)] == nil || subs[h.wire(realtime.NotificationsChannel)] == nil || len(subs) != 2 {
		t.Fatalf("subs %v", subs)
	}

	// Only signed-in panel users get tokens.
	h.expect(h.do(call{method: "POST", path: "/api/v1/realtime/connection-token"}), 401, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/realtime/connection-token", token: owner, noBFF: true}), 401, "")
	// API keys are refused by the session-only operations.
	key := "msk_live_realtime-is-panel-only" // gitleaks:allow (test fixture)
	h.expect(h.do(call{method: "POST", path: "/api/v1/realtime/connection-token", token: key}), 403, "FORBIDDEN")

	sub := func(token, channel string) resp {
		return h.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: token, body: map[string]any{"channel": h.wire(channel)}})
	}
	campaign := "campaign:0193a6c4-7f2e-7b3c-9d1e-4a5b6c7d8e9f"
	for _, tc := range []struct {
		who, token, channel string
		ok                  bool
	}{
		{"owner", owner, realtime.InboxChannel, true},
		{"owner", owner, realtime.CampaignsChannel, true},
		{"owner", owner, campaign, true},
		{"owner", owner, realtime.UnassignedChannel, true},
		{"agent", agent, realtime.InboxChannel, false},
		{"agent", agent, realtime.UnassignedChannel, true},
		{"agent", agent, realtime.CampaignsChannel, false},
		{"agent", agent, campaign, false},
		{"agent", agent, "personal:#" + ownerID, false},
		{"agent", agent, "conversation:0193a6c4-7f2e-7b3c-9d1e-4a5b6c7d8e9f", false},
		{"owner", owner, "secret:all", false},
		{"owner", owner, "inbox:other", false},
	} {
		r := sub(tc.token, tc.channel)
		if !tc.ok {
			h.expect(r, 403, "CHANNEL_FORBIDDEN")
			continue
		}
		h.expect(r, 200, "")
		c := jwtClaims(t, r.data()["token"].(string))
		if c["channel"] != h.wire(tc.channel) || (tc.who == "agent" && c["sub"] != h.tenantID.String()+":"+agentID) ||
			(tc.who == "owner" && c["sub"] != h.tenantID.String()+":"+ownerID) {
			t.Fatalf("%s %s: claims %v", tc.who, tc.channel, c)
		}
	}
	// Unprefixed names and other tenants' channels are refused.
	for _, ch := range []string{realtime.InboxChannel, realtime.TenantChannel(uuid.Must(uuid.NewV7()), realtime.InboxChannel)} {
		h.expect(h.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: owner, body: map[string]any{"channel": ch}}), 403, "CHANNEL_FORBIDDEN")
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: owner, body: map[string]any{}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: key, body: map[string]any{"channel": realtime.InboxChannel}}), 403, "FORBIDDEN")
}

// inboxMessageEvents lists the message events on the inbox channel (the
// conversation channels and conversation.updated events are checked in
// inbox_integration_test.go).
func (h *harness) inboxMessageEvents() []rtEvent {
	h.t.Helper()
	var out []rtEvent
	for _, e := range h.rtEvents() {
		if t, _ := e.body["type"].(string); e.channel == realtime.InboxChannel && strings.HasPrefix(t, "message.") {
			out = append(out, e)
		}
	}
	return out
}

type recordingPublisher struct {
	mu     sync.Mutex
	events []outbox.Event
}

func (p *recordingPublisher) Publish(_ context.Context, channel string, data []byte, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, outbox.Event{Topic: channel, Payload: data})
	return nil
}

func TestMessageActivityIsPublishedThroughTheOutbox(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()

	// An inbound message announces itself to the inbox in the transaction
	// that stores it, without its text.
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.rt-1", "Gizli içerik", "Ali Veli", time.Now()))
	ev := h.inboxMessageEvents()
	if len(ev) != 1 || ev[0].channel != realtime.InboxChannel || ev[0].body["type"] != realtime.TypeMessageCreated ||
		ev[0].body["direction"] != "inbound" || ev[0].body["v"] != float64(1) || ev[0].body["message_id"] == nil {
		t.Fatalf("inbound events %+v", ev)
	}
	raw, _ := json.Marshal(ev[0].body)
	if strings.Contains(string(raw), "Gizli") {
		t.Fatal("message bodies must not be published")
	}
	// Meta redelivering the same message publishes nothing new.
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.rt-1", "Gizli içerik", "Ali Veli", time.Now()))
	if n := len(h.inboxMessageEvents()); n != 1 {
		t.Fatalf("duplicate delivery published %d events", n)
	}

	// Outbound: queued, accepted by Meta, then delivered.
	r := h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "Kargoda"}})
	h.expect(r, 202, "")
	id := r.data()["id"].(string)
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	wamid := h.message(token, id)["wamid"].(string)
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "delivered", testCustomer, id, time.Now(), 0))
	// A stale status does not change anything and is not announced.
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "sent", testCustomer, id, time.Now(), 0))

	var got []string
	for _, e := range h.inboxMessageEvents()[1:] {
		if e.body["message_id"] != id || e.channel != realtime.InboxChannel {
			t.Fatalf("event %+v", e)
		}
		got = append(got, e.body["type"].(string)+"/"+e.body["status"].(string))
	}
	want := []string{"message.created/pending", "message.status/sent", "message.status/delivered"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("outbound events %v, want %v", got, want)
	}

	// The worker's relay routes rt: events to Centrifugo, keyed by channel.
	all := h.rtEvents()
	rec := &recordingPublisher{}
	router := outbox.NewRouter()
	router.Handle(outbox.TaskTopicPrefix, outbox.PublisherFunc(func(context.Context, outbox.Event) error { return nil }))
	router.Handle(realtime.TopicPrefix, realtime.Publisher(rec))
	relay := outbox.NewRelay(h.pool, router, outbox.RelayOptions{Logger: testutil.DiscardLogger()})
	for {
		n, err := relay.DrainOnce(h.ctx())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	if len(rec.events) != len(all) || len(all) < 4 {
		t.Fatalf("published %d realtime events, want %d", len(rec.events), len(all))
	}
	for i, e := range rec.events {
		// Published on the tenant's wire channel (tenant:<id>:<channel>).
		if e.Topic != h.wire(all[i].channel) || (!strings.HasPrefix(all[i].channel, "inbox:") && !strings.HasPrefix(all[i].channel, "conversation:")) || !json.Valid(e.Payload) {
			t.Fatalf("published %q %s", e.Topic, e.Payload)
		}
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE published_at IS NULL"); n != 0 {
		t.Fatalf("%d outbox events left unpublished", n)
	}
}
