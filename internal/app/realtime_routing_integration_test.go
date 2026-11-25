//go:build integration

package app

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

// eventsAbout lists the realtime events about conversation id written after
// the first from outbox rows.
func (h *harness) eventsAbout(id string, from int) []rtEvent {
	h.t.Helper()
	var out []rtEvent
	for _, e := range h.rtEvents()[from:] {
		if e.body["conversation_id"] == id {
			out = append(out, e)
		}
	}
	return out
}

func TestRealtimeRoutingFollowsVisibility(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	agentA, agentAID := h.invite(owner, "a@example.com", "agent")
	_, agentBID := h.invite(owner, "b@example.com", "agent")
	personalA, personalB := "personal:#"+agentAID, "personal:#"+agentBID

	// mayRead reports whether agent A may receive channel: its own
	// server-side personal channel, or one it gets a subscription token for.
	mayRead := func(channel string) bool {
		if channel == personalA {
			return true
		}
		r := h.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: agentA, body: map[string]any{"channel": channel}})
		return r.status == 200
	}

	// Unassigned: inbox:all, the unassigned pool and the conversation.
	from := len(h.rtEvents())
	h.inbound("905550000011", "wamid.rv1", "hello", time.Now().Add(-time.Minute))
	c1 := h.conversationOf("905550000011")
	seen := map[string]int{}
	ids := map[any]bool{}
	for _, e := range h.eventsAbout(c1, from) {
		seen[e.channel]++
		ids[e.body["id"]] = true
		if e.body["id"] == nil || e.body["ts"] == nil || e.body["v"] != float64(1) {
			t.Fatalf("event without envelope: %v", e.body)
		}
	}
	// message.created and conversation.updated on each of three channels,
	// one event ID per event.
	if seen[realtime.InboxChannel] != 2 || seen[realtime.UnassignedChannel] != 2 || seen["conversation:"+c1] != 2 || len(seen) != 3 || len(ids) != 2 {
		t.Fatalf("unassigned conversation went to %v (%d ids)", seen, len(ids))
	}
	if !mayRead(realtime.UnassignedChannel) {
		t.Fatal("agent A may not read the unassigned pool")
	}

	// Assigned to agent B: agent A receives nothing about it afterwards,
	// except that it left the pool.
	from = len(h.rtEvents())
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + c1 + "/assignment", token: owner, body: map[string]any{"assignee_id": agentBID}}), 200, "")
	poolEvents := len(h.rtEvents())
	h.inbound("905550000011", "wamid.rv2", "again", time.Now())
	seen = map[string]int{}
	for _, e := range h.eventsAbout(c1, from) {
		seen[e.channel]++
		if e.channel == realtime.UnassignedChannel && (e.body["type"] != realtime.TypeConversationUpdated || e.body["assignee_id"] != agentBID) {
			t.Fatalf("pool told %v", e.body)
		}
	}
	// assignment: 1 event; inbound: message.created + conversation.updated.
	if seen[realtime.UnassignedChannel] != 1 || seen[personalB] != 3 || seen[realtime.InboxChannel] != 3 || seen["conversation:"+c1] != 3 || seen[personalA] != 0 {
		t.Fatalf("assigned to B: %v", seen)
	}
	for _, e := range h.eventsAbout(c1, poolEvents) {
		if mayRead(e.channel) {
			t.Fatalf("agent A may read %s, which carries B's conversation (%v)", e.channel, e.body["type"])
		}
	}

	// Reassigned from B to A: A's personal channel gets it, and B hears the
	// new state once so its panel can drop it.
	from = len(h.rtEvents())
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + c1 + "/assignment", token: owner, body: map[string]any{"assignee_id": agentAID}}), 200, "")
	seen = map[string]int{}
	for _, e := range h.eventsAbout(c1, from) {
		seen[e.channel]++
		if e.body["assignee_id"] != agentAID {
			t.Fatalf("reassignment event %v", e.body)
		}
	}
	if seen[personalA] != 1 || seen[personalB] != 1 || seen[realtime.InboxChannel] != 1 || seen[realtime.UnassignedChannel] != 0 {
		t.Fatalf("reassigned to A: %v", seen)
	}

	// Re-sync returns only what the caller may read, as conversation.updated.
	h.inbound("905550000012", "wamid.rv3", "other", time.Now())
	c2 := h.conversationOf("905550000012")
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + c2 + "/assignment", token: owner, body: map[string]any{"assignee_id": agentBID}}), 200, "")
	h.inbound("905550000013", "wamid.rv4", "pool", time.Now())
	c3 := h.conversationOf("905550000013")

	since := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	syncPath := func(channel, since string) string {
		return "/api/v1/realtime/sync?channel=" + url.QueryEscape(channel) + "&since=" + since
	}
	synced := func(token, channel string) []string {
		t.Helper()
		r := h.do(call{method: "GET", path: syncPath(channel, since), token: token})
		h.expect(r, 200, "")
		if r.data()["has_more"] != false {
			t.Fatalf("has_more: %s", r.raw)
		}
		out := []string{}
		for _, e := range r.data()["events"].([]any) {
			ev := e.(map[string]any)
			if ev["type"] != realtime.TypeConversationUpdated || ev["id"] == nil || ev["ts"] == nil {
				t.Fatalf("sync event %v", ev)
			}
			out = append(out, ev["conversation_id"].(string))
		}
		slices.Sort(out)
		return out
	}
	sorted := func(s ...string) []string { slices.Sort(s); return s }
	for _, tc := range []struct {
		who, token, channel string
		want                []string
	}{
		{"agent A", agentA, realtime.UnassignedChannel, []string{c3}},
		{"agent A", agentA, personalA, []string{c1}},
		{"agent A", agentA, "conversation:" + c1, []string{c1}},
		{"owner", owner, realtime.InboxChannel, sorted(c1, c2, c3)},
		{"owner", owner, realtime.UnassignedChannel, []string{c3}},
	} {
		if got := synced(tc.token, tc.channel); !slices.Equal(got, tc.want) {
			t.Errorf("%s %s: %v, want %v", tc.who, tc.channel, got, tc.want)
		}
	}
	for _, ch := range []string{realtime.InboxChannel, personalB, "conversation:" + c2, realtime.CampaignsChannel} {
		h.expect(h.do(call{method: "GET", path: syncPath(ch, since), token: agentA}), 403, "CHANNEL_FORBIDDEN")
	}
	// Nothing changed after now.
	r := h.do(call{method: "GET", path: syncPath(realtime.InboxChannel, url.QueryEscape(time.Now().Add(time.Minute).UTC().Format(time.RFC3339))), token: owner})
	h.expect(r, 200, "")
	if len(r.data()["events"].([]any)) != 0 {
		t.Fatalf("future since: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: syncPath(realtime.CampaignsChannel, since), token: owner}), 200, "")
	// Panel sessions only; both parameters are required.
	key := "msk_live_realtime-is-panel-only" // gitleaks:allow (test fixture)
	h.expect(h.do(call{method: "GET", path: syncPath(realtime.InboxChannel, since), token: key}), 403, "FORBIDDEN")
	if r := h.do(call{method: "GET", path: "/api/v1/realtime/sync?channel=inbox:all", token: owner}); r.status < 400 || r.status >= 500 {
		t.Fatalf("missing since: %d", r.status)
	}
}

// A campaign's delivery ticks reach only the message's conversation channel,
// never the inbox channels every panel listens to.
func TestRealtimeCampaignTicksStayInTheirThread(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(3, "+90552")
	id := h.createCampaign(token, tag, 10, map[string]any{"body": []string{"{{first_name}}", "x"}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	var msgID, wamid, phone string
	if err := h.pool.QueryRow(context.Background(), `SELECT m.id::text, m.wamid, c.phone_e164 FROM messages m JOIN contacts c ON c.id = m.contact_id
		WHERE m.campaign_id = $1 AND m.wamid IS NOT NULL LIMIT 1`, id).Scan(&msgID, &wamid, &phone); err != nil {
		t.Fatal(err)
	}
	from := len(h.rtEvents())
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "read", strings.TrimPrefix(phone, "+"), msgID, time.Now(), 0))
	var channels []string
	for _, e := range h.rtEvents()[from:] {
		if e.body["message_id"] == msgID && e.body["type"] == realtime.TypeMessageStatus {
			channels = append(channels, e.channel)
		}
	}
	if len(channels) != 1 || !strings.HasPrefix(channels[0], "conversation:") {
		t.Fatalf("campaign tick went to %v", channels)
	}
}
