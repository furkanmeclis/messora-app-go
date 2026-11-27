//go:build integration

package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
	"github.com/furkanmeclis/messora-app-go/migrations"
)

// invite creates a user with the given role and returns its session and ID.
func (h *harness) invite(owner, email, role string) (token, id string) {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/invitations", token: owner, body: map[string]any{"email": email, "role": role}})
	h.expect(r, 201, "")
	link := r.data()["accept_url"].(string)
	r = h.do(call{method: "POST", path: "/api/v1/invitations/accept", body: map[string]any{
		"token": link[strings.Index(link, "token=")+6:], "name": "User " + role, "password": "Correct-horse-staple-9"}}) // gitleaks:allow (test fixture)
	h.expect(r, 201, "")
	return r.data()["session_token"].(string), r.data()["user"].(map[string]any)["id"].(string)
}

// inbound delivers one customer text message.
func (h *harness) inbound(from, wamid, body string, at time.Time) {
	h.t.Helper()
	if code := h.webhook(whatsapptest.InboundText(testPNID, from, wamid, body, "Customer "+from[len(from)-2:], at)); code != 200 {
		h.t.Fatalf("webhook: %d", code)
	}
}

// conversations lists conversations with the query string q.
func (h *harness) conversations(token, q string) []map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/conversations" + q, token: token})
	h.expect(r, 200, "")
	out := make([]map[string]any, len(r.list()))
	for i, c := range r.list() {
		out[i] = c.(map[string]any)
	}
	return out
}

// conversationOf returns the conversation ID of a customer.
func (h *harness) conversationOf(customer string) string {
	h.t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(h.ctx(),
		"SELECT v.id FROM conversations v JOIN contacts c ON c.id = v.contact_id WHERE c.phone_e164 = $1", "+"+customer).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	return id.String()
}

type download struct {
	StatusCode int
	Header     http.Header
}

// download fetches a file (see raw in contacts_integration_test.go).
func (h *harness) download(path, token string) (download, []byte) {
	h.t.Helper()
	status, body, header := h.raw(http.MethodGet, path, token, "", nil)
	return download{StatusCode: status, Header: header}, body
}

func ids(cs []map[string]any) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c["id"].(string)
	}
	return out
}

func TestInboxInboundCreatesConversationAndPublishes(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()

	h.inbound(testCustomer, "wamid.c1", "Merhaba", time.Now())
	cs := h.conversations(owner, "")
	if len(cs) != 1 {
		t.Fatalf("conversations: %v", cs)
	}
	c := cs[0]
	contact := c["contact"].(map[string]any)
	last := c["last_message"].(map[string]any)
	if c["status"] != "open" || c["unread_count"] != float64(1) || c["window_open"] != true || c["window_expires_at"] == nil ||
		contact["phone"] != "+"+testCustomer || contact["profile_name"] != "Customer 33" || c["assignee"] != nil ||
		last["direction"] != "inbound" || last["body"] != "Merhaba" || c["phone_number"].(map[string]any)["verified_name"] != "Acme" {
		t.Fatalf("conversation: %v", c)
	}
	id := c["id"].(string)

	// Announced in the same transaction: the conversation on the inbox and
	// its own channel, the message also on the conversation channel.
	var convInbox, convOwn, msgOwn int
	for _, e := range h.rtEvents() {
		switch {
		case e.body["type"] == realtime.TypeConversationUpdated && e.body["conversation_id"] == id && e.channel == realtime.InboxChannel:
			convInbox++
			if e.body["unread_count"] != float64(1) || e.body["status"] != "open" {
				t.Fatalf("conversation event %v", e.body)
			}
		case e.body["type"] == realtime.TypeConversationUpdated && e.channel == realtime.ConversationChannel(uuid.MustParse(id)):
			convOwn++
		case e.body["type"] == realtime.TypeMessageCreated && e.channel == realtime.ConversationChannel(uuid.MustParse(id)):
			msgOwn++
			if e.body["conversation_id"] != id {
				t.Fatalf("message event %v", e.body)
			}
		}
	}
	if convInbox != 1 || convOwn != 1 || msgOwn != 1 {
		t.Fatalf("events: inbox %d, conversation %d, message %d\n%v", convInbox, convOwn, msgOwn, h.rtEvents())
	}
	for _, e := range h.inboxMessageEvents() {
		if e.body["conversation_id"] != id {
			t.Fatalf("inbox message event without its conversation: %v", e.body)
		}
	}

	// Redelivery changes nothing; a second message counts as unread.
	before := len(h.rtEvents())
	h.inbound(testCustomer, "wamid.c1", "Merhaba", time.Now())
	if len(h.rtEvents()) != before || h.conversations(owner, "")[0]["unread_count"] != float64(1) {
		t.Fatal("duplicate delivery must not touch the conversation")
	}
	h.inbound(testCustomer, "wamid.c2", "Orada mısınız?", time.Now())
	if c := h.conversations(owner, "")[0]; c["unread_count"] != float64(2) || c["last_message"].(map[string]any)["body"] != "Orada mısınız?" {
		t.Fatalf("after second message: %v", c)
	}
	if h.count("SELECT count(*) FROM conversations WHERE id = $1 AND last_inbound_at IS NOT NULL", id) != 1 {
		t.Fatal("window not recorded")
	}

	// Mark read resets the team's counter and announces it.
	r := h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/read", token: owner})
	h.expect(r, 200, "")
	if r.data()["unread_count"] != float64(0) {
		t.Fatalf("read: %s", r.raw)
	}
	ev := h.rtEvents()
	if l := ev[len(ev)-1]; l.body["type"] != realtime.TypeConversationUpdated || l.body["unread_count"] != float64(0) {
		t.Fatalf("read not announced: %v", l)
	}
}

func TestInboxVisibility(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	agentA, _ := h.invite(owner, "a@example.com", "agent")
	agentB, agentBID := h.invite(owner, "b@example.com", "agent")

	h.inbound("905550000001", "wamid.v1", "one", time.Now().Add(-time.Minute))
	h.inbound("905550000002", "wamid.v2", "two", time.Now())
	c1, c2 := h.conversationOf("905550000001"), h.conversationOf("905550000002")

	// Agents cannot assign (conversations.assign); the owner can.
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + c1 + "/assignment", token: agentA, body: map[string]any{"assignee_id": agentBID}}), 403, "FORBIDDEN")
	r := h.do(call{method: "PUT", path: "/api/v1/conversations/" + c1 + "/assignment", token: owner, body: map[string]any{"assignee_id": agentBID}})
	h.expect(r, 200, "")
	if a := r.data()["assignee"].(map[string]any); a["id"] != agentBID || a["name"] != "User agent" {
		t.Fatalf("assignee %v", a)
	}

	// Owner (read_all) sees both; agent B sees its own and the unassigned
	// one; agent A only the unassigned one.
	if got := ids(h.conversations(owner, "")); !slices.Equal(got, []string{c2, c1}) {
		t.Fatalf("owner sees %v", got)
	}
	if got := ids(h.conversations(agentB, "")); !slices.Equal(got, []string{c2, c1}) {
		t.Fatalf("agent B sees %v", got)
	}
	if got := ids(h.conversations(agentA, "")); !slices.Equal(got, []string{c2}) {
		t.Fatalf("agent A sees %v", got)
	}
	// Filters cannot widen visibility.
	if got := h.conversations(agentA, "?assignee="+agentBID); len(got) != 0 {
		t.Fatalf("agent A filtered by B: %v", got)
	}
	if got := ids(h.conversations(owner, "?assignee="+agentBID)); !slices.Equal(got, []string{c1}) {
		t.Fatalf("owner filtered by B: %v", got)
	}
	if got := ids(h.conversations(owner, "?assignee=unassigned")); !slices.Equal(got, []string{c2}) {
		t.Fatalf("unassigned: %v", got)
	}
	if got := ids(h.conversations(agentB, "?assignee=me")); !slices.Equal(got, []string{c1}) {
		t.Fatalf("B's own: %v", got)
	}
	if got := h.conversations(owner, "?assignee=me"); len(got) != 0 {
		t.Fatalf("owner's own: %v", got)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations?assignee=someone", token: owner}), 422, "VALIDATION_ERROR")

	// Agent A cannot read B's conversation through any endpoint, not even
	// by guessing its ID; the answer is indistinguishable from "not found".
	var mediaID string
	if err := h.pool.QueryRow(h.ctx(), `INSERT INTO media_files (message_id, mime_type, status)
		SELECT id, 'image/jpeg', 'pending' FROM messages WHERE wamid = 'wamid.v1' RETURNING id`).Scan(&mediaID); err != nil {
		t.Fatal(err)
	}
	for _, c := range []call{
		{method: "GET", path: "/api/v1/conversations/" + c1},
		{method: "GET", path: "/api/v1/conversations/" + c1 + "/messages"},
		{method: "GET", path: "/api/v1/conversations/" + c1 + "/events"},
		{method: "POST", path: "/api/v1/conversations/" + c1 + "/read"},
		{method: "POST", path: "/api/v1/conversations/" + c1 + "/messages", body: map[string]any{"type": "text", "text": map[string]any{"body": "hi"}}},
	} {
		c.token = agentA
		h.expect(h.do(c), 404, "NOT_FOUND")
	}
	if res, body := h.download("/api/v1/conversations/"+c1+"/media/"+mediaID, agentA); res.StatusCode != 404 {
		t.Fatalf("agent A downloaded B's media: %d %s", res.StatusCode, body)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations/" + c1, token: agentB}), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations/" + c2, token: agentA}), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations/" + uuid.Must(uuid.NewV7()).String(), token: owner}), 404, "NOT_FOUND")

	// Realtime: agents may subscribe to the conversations they can see.
	sub := func(token, id string) resp {
		return h.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: token, body: map[string]any{"channel": h.wire("conversation:" + id)}})
	}
	h.expect(sub(agentA, c1), 403, "CHANNEL_FORBIDDEN")
	h.expect(sub(agentA, c2), 200, "")
	h.expect(sub(agentB, c1), 200, "")
	h.expect(sub(owner, c1), 200, "")
	h.expect(sub(agentA, uuid.Must(uuid.NewV7()).String()), 403, "CHANNEL_FORBIDDEN")

	// Assignment rules: unknown and disabled users are refused.
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + c2 + "/assignment", token: owner,
		body: map[string]any{"assignee_id": uuid.Must(uuid.NewV7()).String()}}), 422, "VALIDATION_ERROR")
	r = h.do(call{method: "GET", path: "/api/v1/users", token: owner})
	var agentAID string
	for _, u := range r.list() {
		if m := u.(map[string]any); m["email"] == "a@example.com" {
			agentAID = m["id"].(string)
		}
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/users/" + agentAID, token: owner, body: map[string]any{"status": "disabled"}}), 200, "")
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + c2 + "/assignment", token: owner, body: map[string]any{"assignee_id": agentAID}}), 422, "VALIDATION_ERROR")

	// Unassigning makes it visible to everyone again; history and audit
	// record both changes.
	r = h.do(call{method: "PUT", path: "/api/v1/conversations/" + c1 + "/assignment", token: owner, body: map[string]any{"assignee_id": nil}})
	h.expect(r, 200, "")
	if r.data()["assignee"] != nil {
		t.Fatalf("unassign: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/conversations/" + c1 + "/events", token: owner})
	h.expect(r, 200, "")
	ev := r.list()
	if len(ev) != 2 || ev[0].(map[string]any)["kind"] != "unassigned" || ev[1].(map[string]any)["kind"] != "assigned" ||
		ev[1].(map[string]any)["actor_name"] != "Ayşe Owner" || ev[1].(map[string]any)["assignee"].(map[string]any)["id"] != agentBID {
		t.Fatalf("events %s", r.raw)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action IN ('conversation.assigned', 'conversation.unassigned') AND entity_id = $1", c1) != 2 {
		t.Fatal("assignment not audited")
	}
	// Re-assigning the same user is a no-op.
	h.do(call{method: "PUT", path: "/api/v1/conversations/" + c1 + "/assignment", token: owner, body: map[string]any{"assignee_id": nil}})
	if h.count("SELECT count(*) FROM conversation_events WHERE conversation_id = $1", c1) != 2 {
		t.Fatal("no-op assignment recorded")
	}
}

func TestInboxReplyRules(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	agent, agentID := h.invite(owner, "agent@example.com", "agent")
	h.inbound(testCustomer, "wamid.r1", "Siparişim?", time.Now())
	id := h.conversationOf(testCustomer)
	reply := func(token string, body map[string]any) resp {
		return h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/messages", token: token, body: body})
	}
	text := map[string]any{"type": "text", "text": map[string]any{"body": "Kargoda"}}
	template := map[string]any{"type": "template", "template": map[string]any{"name": "order_update", "language": "en"}}

	// Inside the window an agent can reply with text, from the
	// conversation's number, attributed to them.
	r := reply(agent, text)
	h.expect(r, 202, "")
	msgID := r.data()["id"].(string)
	if r.data()["status"] != "pending" || r.data()["source"] != "panel" || r.data()["direction"] != "outbound" {
		t.Fatalf("reply %s", r.raw)
	}
	if h.count("SELECT count(*) FROM messages WHERE id = $1 AND created_by = $2", msgID, agentID) != 1 {
		t.Fatal("reply not attributed")
	}
	if err := h.deliver(msgID); err != nil {
		t.Fatal(err)
	}
	wamid := h.message(owner, msgID)["wamid"].(string)
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "delivered", testCustomer, msgID, time.Now(), 0))
	r = h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/messages", token: agent})
	h.expect(r, 200, "")
	newest := r.list()[0].(map[string]any)
	if newest["id"] != msgID || newest["status"] != "delivered" || newest["delivered_at"] == nil {
		t.Fatalf("history %s", r.raw)
	}
	// The delivery tick reaches the conversation channel.
	found := false
	for _, e := range h.rtEvents() {
		if e.channel == "conversation:"+id && e.body["type"] == realtime.TypeMessageStatus && e.body["status"] == "delivered" {
			found = true
		}
	}
	if !found {
		t.Fatal("delivery update not published on the conversation channel")
	}
	// The reply does not reopen the window or count as unread.
	if c := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: owner}).data(); c["unread_count"] != float64(1) ||
		c["last_message"].(map[string]any)["direction"] != "outbound" {
		t.Fatalf("after reply: %v", c)
	}

	// The window closes 24 hours after the customer's last message: only
	// templates then.
	if _, err := h.pool.Exec(h.ctx(), "UPDATE conversations SET last_inbound_at = now() - interval '25 hours' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if c := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: agent}).data(); c["window_open"] != false {
		t.Fatalf("window still open: %v", c)
	}
	h.expect(reply(agent, text), 422, "OUTSIDE_SERVICE_WINDOW")
	h.expect(reply(agent, map[string]any{"type": "image", "media": map[string]any{"link": "https://example.com/a.jpg"}}), 422, "OUTSIDE_SERVICE_WINDOW")
	h.expect(reply(agent, template), 202, "")
	// Unapproved templates and malformed content are messaging's validation.
	h.expect(reply(agent, map[string]any{"type": "template", "template": map[string]any{"name": "promo", "language": "en"}}), 422, "VALIDATION_ERROR")
	h.expect(reply(agent, map[string]any{"type": "location"}), 422, "VALIDATION_ERROR")

	// Opted-out contacts get no templates; inside a fresh window a reply is
	// still allowed.
	contactID := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: owner}).data()["contact"].(map[string]any)["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + contactID + "/consent", token: owner, body: map[string]any{"kind": "opt_out"}}), 200, "")
	h.expect(reply(agent, template), 422, "CONTACT_OPTED_OUT")
	if c := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: agent}).data(); c["contact"].(map[string]any)["opted_out"] != true {
		t.Fatalf("opt-out not shown: %v", c)
	}
	h.inbound(testCustomer, "wamid.r2", "Bir sorum daha var", time.Now())
	h.expect(reply(agent, text), 202, "")

	// Replying needs conversations.reply: an API key with only read scopes
	// is refused.
	h.do(call{method: "POST", path: "/api/v1/auth/reauth", token: owner, body: map[string]any{"password": ownerPassword}})
	r = h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: map[string]any{"name": "crm", "scopes": []string{"conversations.read"}}})
	h.expect(r, 201, "")
	key := r.data()["secret"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/messages", token: key, noBFF: true, body: text}), 403, "FORBIDDEN")
	// An API key with conversations.read sees unassigned conversations.
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: key, noBFF: true}), 200, "")
}

func TestInboxCloseAndReopen(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	agent, _ := h.invite(owner, "agent@example.com", "agent")
	h.inbound(testCustomer, "wamid.o1", "Merhaba", time.Now())
	id := h.conversationOf(testCustomer)

	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/close", token: agent}), 403, "FORBIDDEN")
	r := h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/close", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "closed" || r.data()["closed_at"] == nil {
		t.Fatalf("close %s", r.raw)
	}
	if got := ids(h.conversations(owner, "?status=closed")); !slices.Equal(got, []string{id}) {
		t.Fatalf("closed filter %v", got)
	}
	if got := h.conversations(owner, "?status=open"); len(got) != 0 {
		t.Fatalf("open filter %v", got)
	}
	// Closing twice changes nothing.
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/close", token: owner}), 200, "")

	// The next customer message reopens it automatically.
	h.inbound(testCustomer, "wamid.o2", "Bir şey daha", time.Now())
	c := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: owner}).data()
	if c["status"] != "open" || c["closed_at"] != nil || c["unread_count"] != float64(2) {
		t.Fatalf("after inbound: %v", c)
	}
	// An outbound message does not reopen a closed conversation.
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/close", token: owner}), 200, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/messages", token: agent,
		body: map[string]any{"type": "text", "text": map[string]any{"body": "Kapatıyoruz"}}}), 202, "")
	if h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: owner}).data()["status"] != "closed" {
		t.Fatal("outbound reopened the conversation")
	}
	r = h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/reopen", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "open" {
		t.Fatalf("reopen %s", r.raw)
	}

	r = h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/events", token: agent})
	h.expect(r, 200, "")
	var kinds []string
	for _, e := range r.list() {
		m := e.(map[string]any)
		kinds = append(kinds, m["kind"].(string)+"/"+m["actor_type"].(string))
	}
	want := []string{"reopened/user", "closed/user", "reopened/system", "closed/user"}
	if !slices.Equal(kinds, want) {
		t.Fatalf("history %v, want %v", kinds, want)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action IN ('conversation.close', 'conversation.reopen')") != 3 {
		t.Fatal("status changes not audited")
	}
}

func TestInboxPagination(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	base := time.Now().Add(-time.Hour)
	for i := range 5 {
		h.inbound(fmt.Sprintf("90555000010%d", i), fmt.Sprintf("wamid.p%d", i), "hi", base.Add(time.Duration(i)*time.Minute))
	}
	// Conversations: most recent activity first, across pages.
	var seen []string
	cursor := ""
	for page := 0; ; page++ {
		path := "/api/v1/conversations?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := h.do(call{method: "GET", path: path, token: owner})
		h.expect(r, 200, "")
		for _, c := range r.list() {
			seen = append(seen, c.(map[string]any)["contact"].(map[string]any)["phone"].(string))
		}
		cursor, _ = r.body["meta"].(map[string]any)["next_cursor"].(string)
		if cursor == "" || page > 5 {
			break
		}
	}
	want := []string{"+905550000104", "+905550000103", "+905550000102", "+905550000101", "+905550000100"}
	if !slices.Equal(seen, want) {
		t.Fatalf("pages %v, want %v", seen, want)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations?cursor=nope", token: owner}), 422, "VALIDATION_ERROR")
	if got := h.conversations(owner, "?q=000103"); len(got) != 1 {
		t.Fatalf("search by phone %v", got)
	}
	if got := h.conversations(owner, "?q=Customer%2003"); len(got) != 1 {
		t.Fatalf("search by name %v", got)
	}
	if got := h.conversations(owner, "?q=%25"); len(got) != 0 {
		t.Fatalf("wildcards must be literal: %v", got)
	}

	// History: newest first, keyset by message ID.
	for i := range 4 {
		h.inbound("905550000100", fmt.Sprintf("wamid.h%d", i), fmt.Sprintf("m%d", i), time.Now())
	}
	id := h.conversationOf("905550000100")
	var bodies []string
	cursor = ""
	for page := 0; ; page++ {
		path := "/api/v1/conversations/" + id + "/messages?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := h.do(call{method: "GET", path: path, token: owner})
		h.expect(r, 200, "")
		for _, m := range r.list() {
			bodies = append(bodies, m.(map[string]any)["body"].(string))
		}
		cursor, _ = r.body["meta"].(map[string]any)["next_cursor"].(string)
		if cursor == "" || page > 5 {
			break
		}
	}
	if want := []string{"m3", "m2", "m1", "m0", "hi"}; !slices.Equal(bodies, want) {
		t.Fatalf("history %v, want %v", bodies, want)
	}
	// The cursor stays inside the conversation.
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + h.conversationOf("905550000104") + "/messages", token: owner})
	if len(r.list()) != 1 {
		t.Fatalf("other conversation %s", r.raw)
	}
}

func TestInboxMediaDownload(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	agentA, _ := h.invite(owner, "a@example.com", "agent")
	_, agentBID := h.invite(owner, "b@example.com", "agent")
	jpeg := []byte("\xff\xd8\xff fake jpeg")
	h.meta.AddMedia("media-1", "image/jpeg", jpeg)
	h.meta.AddMedia("media-2", "application/pdf", []byte("%PDF-1.7"))
	h.webhook(whatsapptest.InboundMedia(testPNID, testCustomer, "wamid.img", "image", "media-1", "image/jpeg", "fatura", time.Now()))
	h.webhook(whatsapptest.InboundMedia(testPNID, testCustomer, "wamid.doc", "document", "media-2", "application/pdf", "", time.Now()))
	id := h.conversationOf(testCustomer)

	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/messages", token: owner})
	h.expect(r, 200, "")
	media := map[string]string{}
	for _, m := range r.list() {
		md := m.(map[string]any)["media"].(map[string]any)
		if md["status"] != "pending" {
			t.Fatalf("media before download %v", md)
		}
		media[m.(map[string]any)["type"].(string)] = md["id"].(string)
	}
	path := "/api/v1/conversations/" + id + "/media/"
	res, body := h.download(path+media["image"], owner)
	if res.StatusCode != 409 || !strings.Contains(string(body), "MEDIA_NOT_STORED") {
		t.Fatalf("pending media: %d %s", res.StatusCode, body)
	}
	for _, m := range media {
		if err := h.svc.messaging.DownloadMedia(h.ctx(), uuid.MustParse(m)); err != nil {
			t.Fatal(err)
		}
	}

	res, body = h.download(path+media["image"], agentA)
	if res.StatusCode != 200 || string(body) != string(jpeg) {
		t.Fatalf("download: %d %q", res.StatusCode, body)
	}
	for k, want := range map[string]string{
		"Content-Type": "image/jpeg", "X-Content-Type-Options": "nosniff", "Content-Length": fmt.Sprint(len(jpeg)),
		"Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if got := res.Header.Get(k); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
	if d := res.Header.Get("Content-Disposition"); !strings.HasPrefix(d, "inline; filename=media-") || !strings.HasSuffix(d, ".jpg") {
		t.Errorf("image disposition %q", d)
	}
	res, body = h.download(path+media["document"], owner)
	if res.StatusCode != 200 || string(body) != "%PDF-1.7" || res.Header.Get("Content-Type") != "application/pdf" ||
		!strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment;") {
		t.Fatalf("document: %d %v", res.StatusCode, res.Header)
	}

	// Media of another conversation, unknown IDs and invisible
	// conversations are all "not found".
	h.inbound("905550000009", "wamid.other", "x", time.Now())
	other := h.conversationOf("905550000009")
	if res, _ := h.download("/api/v1/conversations/"+other+"/media/"+media["image"], owner); res.StatusCode != 404 {
		t.Fatalf("cross-conversation media: %d", res.StatusCode)
	}
	if res, _ := h.download(path+uuid.Must(uuid.NewV7()).String(), owner); res.StatusCode != 404 {
		t.Fatalf("unknown media: %d", res.StatusCode)
	}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/assignment", token: owner, body: map[string]any{"assignee_id": agentBID}}), 200, "")
	if res, _ := h.download(path+media["image"], agentA); res.StatusCode != 404 {
		t.Fatalf("agent downloaded media of someone else's conversation: %d", res.StatusCode)
	}
	if res, _ := h.download(path+media["image"], ""); res.StatusCode != 401 {
		t.Fatalf("anonymous download: %d", res.StatusCode)
	}
}

// The window and the inbox list must follow the thread (customer and
// sending number): replies through POST /messages use the same rule.
func TestInboxWindowAppliesToTheDeveloperAPI(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	h.inbound(testCustomer, "wamid.w1", "hi", time.Now())
	h.expect(h.send(owner, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "hello"}}), 202, "")
	if _, err := h.pool.Exec(h.ctx(), "UPDATE conversations SET last_inbound_at = now() - interval '24 hours 1 minute'"); err != nil {
		t.Fatal(err)
	}
	h.expect(h.send(owner, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "hello"}}), 422, "OUTSIDE_SERVICE_WINDOW")
	// Business-initiated messages create the conversation too.
	h.expect(h.send(owner, map[string]any{"to": "+905550000777", "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}}), 202, "")
	c := h.do(call{method: "GET", path: "/api/v1/conversations/" + h.conversationOf("905550000777"), token: owner}).data()
	if c["unread_count"] != float64(0) || c["window_open"] != false || c["last_inbound_at"] != nil {
		t.Fatalf("outbound-only conversation %v", c)
	}
}

// Upgrading a database that already has messages creates their
// conversations.
func TestInboxMigrationBackfill(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Postgres(t)
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 4); err != nil {
		t.Fatal(err)
	}
	seed := `
WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc) VALUES ('a', '1', '\x00') RETURNING id),
     n AS (INSERT INTO phone_numbers (account_id, phone_number_id) SELECT id, '2' FROM a RETURNING id),
     c AS (INSERT INTO contacts (phone_e164) VALUES ('+905550000001'), ('+905550000002') RETURNING id, phone_e164)
INSERT INTO messages (phone_number_id, contact_id, direction, type, body, status, occurred_at)
SELECT n.id, c.id, d.direction, d.type, d.body, d.status, now() - d.age
FROM n, c, (VALUES ('inbound', 'text', 'first', 'received', interval '3 hours'),
                   ('inbound', 'reaction', '👍', 'received', interval '2 hours'),
                   ('outbound', 'text', 'last', 'sent', interval '1 hour')) AS d(direction, type, body, status, age)
WHERE c.phone_e164 = '+905550000001' OR d.direction = 'outbound'`
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT ct.phone_e164, v.status, v.last_inbound_at IS NOT NULL,
		now() - v.last_inbound_at > interval '170 minutes', m.body
		FROM conversations v JOIN contacts ct ON ct.id = v.contact_id JOIN messages m ON m.id = v.last_message_id ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var phone, status, last string
		var hasInbound bool
		var old *bool
		if err := rows.Scan(&phone, &status, &hasInbound, &old, &last); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s %v %v %s", phone, status, hasInbound, old != nil && *old, last))
	}
	rows.Close()
	// The reaction does not count as a window-opening message; the last
	// message is the newest one.
	want := []string{"+905550000001 open true true last", "+905550000002 open false false last"}
	if !slices.Equal(got, want) {
		t.Fatalf("backfill %v, want %v", got, want)
	}
}

// Roadmap done criterion: listing 10k conversations stays under 200 ms
// (p95) for both visibility modes.
func TestInboxListPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("performance test")
	}
	h := newHarness(t)
	owner, _ := h.connect()
	agent, agentID := h.invite(owner, "agent@example.com", "agent")
	seed := `
WITH c AS (
  INSERT INTO contacts (phone_e164, name)
  SELECT '+90555' || lpad(g::text, 7, '0'), 'Customer ' || g FROM generate_series(1, 10000) g
  RETURNING id, phone_e164)
INSERT INTO conversations (contact_id, phone_number_id, status, assignee_id, last_message_at, last_inbound_at, unread_count)
SELECT c.id, (SELECT id FROM phone_numbers LIMIT 1),
       CASE WHEN right(c.phone_e164, 1) IN ('0', '1') THEN 'closed' ELSE 'open' END,
       CASE WHEN right(c.phone_e164, 1) IN ('2', '3', '4') THEN $1::uuid END,
       now() - (right(c.phone_e164, 5)::int * interval '1 minute'), now() - interval '1 hour', right(c.phone_e164, 1)::int
FROM c`
	if _, err := h.pool.Exec(h.ctx(), seed, uuid.MustParse(agentID)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx(), "ANALYZE conversations; ANALYZE contacts"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ who, token, q string }{
		{"owner", owner, ""},
		{"owner open", owner, "?status=open"},
		{"owner search", owner, "?q=Customer%2099"},
		{"agent", agent, ""},
		{"agent mine", agent, "?assignee=me&status=open"},
		{"agent unread", agent, "?unread=true"},
	} {
		// One validated call, then timed calls without the (slow) response
		// validation of the test harness.
		r := h.do(call{method: "GET", path: "/api/v1/conversations" + tc.q, token: tc.token})
		h.expect(r, 200, "")
		if len(r.list()) == 0 {
			t.Fatalf("%s: empty page", tc.who)
		}
		var d []time.Duration
		for range 40 {
			start := time.Now()
			status, _, _ := h.raw(http.MethodGet, "/api/v1/conversations"+tc.q, tc.token, "", nil)
			d = append(d, time.Since(start))
			if status != 200 {
				t.Fatalf("%s: %d", tc.who, status)
			}
		}
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		p95 := d[len(d)*95/100-1]
		t.Logf("%s: p50 %v, p95 %v", tc.who, d[len(d)/2], p95)
		if p95 > 200*time.Millisecond {
			t.Errorf("%s: p95 %v exceeds 200 ms", tc.who, p95)
		}
	}
}
