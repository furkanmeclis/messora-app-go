package realtime

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

const testSecret = "realtime-test-hmac-secret-0123456789" // gitleaks:allow (test fixture)

func user(perms ...string) *principal.Principal {
	slices.Sort(perms)
	return &principal.Principal{Kind: principal.KindUser, ID: uuid.Must(uuid.NewV7()), Permissions: perms}
}

// The permissions of the system roles (migrations/00002_identity.sql).
var (
	owner   = user("campaigns.read", "conversations.read", "conversations.read_all", "settings.read")
	manager = user("campaigns.read", "conversations.read", "conversations.read_all")
	agent   = user("contacts.read", "contacts.update", "conversations.read", "conversations.reply", "templates.read")
	viewer  = user("settings.read")
)

func TestAuthorizeMatrix(t *testing.T) {
	campaign := uuid.Must(uuid.NewV7())
	conv := uuid.Must(uuid.NewV7())
	apiKey := &principal.Principal{Kind: principal.KindAPIKey, ID: uuid.Must(uuid.NewV7()), Permissions: owner.Permissions}
	mfaPending := user("campaigns.read", "conversations.read_all")
	mfaPending.MFAPending = true

	cases := []struct {
		who     string
		p       *principal.Principal
		channel string
		ok      bool
	}{
		{"owner", owner, InboxChannel, true},
		{"owner", owner, UnassignedChannel, true},
		{"owner", owner, CampaignsChannel, true},
		{"owner", owner, CampaignChannel(campaign), true},
		{"owner", owner, ConversationChannel(conv), true},
		{"owner", owner, PersonalChannel(owner.ID), true},
		{"owner", owner, NotificationsChannel, true},
		{"owner", owner, PersonalChannel(agent.ID), false},
		{"manager", manager, InboxChannel, true},
		{"manager", manager, UnassignedChannel, true},
		{"manager", manager, CampaignChannel(campaign), true},
		// conversations.read alone: inbox:all would name conversations
		// assigned to others; the unassigned pool and the personal
		// channel carry what the agent may see.
		{"agent", agent, InboxChannel, false},
		{"agent", agent, UnassignedChannel, true},
		{"agent", agent, PersonalChannel(agent.ID), true},
		{"agent", agent, CampaignsChannel, false},
		{"agent", agent, CampaignChannel(campaign), false},
		{"agent", agent, ConversationChannel(conv), false},
		{"agent", agent, PersonalChannel(owner.ID), false},
		{"viewer", viewer, InboxChannel, false},
		{"viewer", viewer, UnassignedChannel, false},
		{"viewer", viewer, NotificationsChannel, true},
		{"api key", apiKey, InboxChannel, false},
		{"anonymous", nil, InboxChannel, false},
		{"mfa pending", mfaPending, InboxChannel, false},
		// Unknown and malformed channels.
		{"owner", owner, "inbox", false},
		{"owner", owner, "inbox:", false},
		{"owner", owner, "inbox:other", false},
		{"owner", owner, "inbox:Unassigned", false},
		{"owner", owner, "inbox:unassigned:x", false},
		{"owner", owner, "admin:all", false},
		{"owner", owner, "all", false},
		{"owner", owner, "campaign:not-a-uuid", false},
		{"owner", owner, "campaign:" + strings.ToUpper(campaign.String()), false},
		{"owner", owner, "campaign:" + campaign.String() + ":x", false},
		{"owner", owner, "personal:" + owner.ID.String(), false},
		{"owner", owner, "personal:#" + owner.ID.String() + ",x", false},
		{"owner", owner, "system:other", false},
		{"owner", owner, "", false},
	}
	for _, c := range cases {
		err := Authorize(c.p, c.channel)
		if (err == nil) != c.ok {
			t.Errorf("%s → %q: allowed=%v, want %v (err %v)", c.who, c.channel, err == nil, c.ok, err)
		}
		if err != nil && !errors.Is(err, ErrChannelForbidden) {
			t.Errorf("%s → %q: unexpected error %v", c.who, c.channel, err)
		}
	}
}

func TestAuthorizeConversationAccess(t *testing.T) {
	mine, others, broken := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	var asked []uuid.UUID
	access := func(_ context.Context, p *principal.Principal, id uuid.UUID) (bool, error) {
		if p != agent {
			t.Fatalf("checker called for %v", p.Permissions)
		}
		asked = append(asked, id)
		if id == broken {
			return false, errors.New("database down")
		}
		return id == mine, nil
	}
	ctx := context.Background()
	readOnly := user("contacts.read")
	cases := []struct {
		who     string
		p       *principal.Principal
		channel string
		ok      bool
	}{
		{"agent", agent, ConversationChannel(mine), true},
		{"agent", agent, ConversationChannel(others), false},
		{"agent", agent, "conversation:" + strings.ToUpper(mine.String()), false},
		{"agent", agent, "conversation:not-a-uuid", false},
		{"owner", owner, ConversationChannel(others), true}, // read_all: no lookup
		{"viewer", viewer, ConversationChannel(mine), false},
		{"no inbox permission", readOnly, ConversationChannel(mine), false},
	}
	for _, c := range cases {
		err := AuthorizeContext(ctx, c.p, c.channel, access)
		if (err == nil) != c.ok {
			t.Errorf("%s → %q: allowed=%v, want %v (err %v)", c.who, c.channel, err == nil, c.ok, err)
		}
	}
	if len(asked) != 2 || asked[0] != mine || asked[1] != others {
		t.Fatalf("checker consulted for %v; only agents' well-formed channels need it", asked)
	}
	// A failing lookup is an error, not a grant.
	if err := AuthorizeContext(ctx, agent, ConversationChannel(broken), access); err == nil || errors.Is(err, ErrChannelForbidden) {
		t.Fatalf("lookup failure: %v", err)
	}
	// Without a checker agents keep the Phase 6 behaviour.
	if err := AuthorizeContext(ctx, agent, ConversationChannel(mine), nil); !errors.Is(err, ErrChannelForbidden) {
		t.Fatalf("no checker: %v", err)
	}

	iss, _ := NewIssuer(testSecret)
	iss.WithConversationAccess(access)
	tok, err := iss.SubscriptionContext(ctx, agent, ConversationChannel(mine))
	if err != nil {
		t.Fatal(err)
	}
	if c := verify(t, tok.Token, testSecret); c["channel"] != ConversationChannel(mine) {
		t.Fatalf("claims %v", c)
	}
	if _, err := iss.SubscriptionContext(ctx, agent, ConversationChannel(others)); !errors.Is(err, ErrChannelForbidden) {
		t.Fatalf("agent got a token for someone else's conversation: %v", err)
	}
}

func TestConversationEvent(t *testing.T) {
	assignee := uuid.New()
	c := db.Conversation{ID: uuid.New(), ContactID: uuid.New(), PhoneNumberID: uuid.New(), Status: "open", AssigneeID: &assignee,
		UnreadCount: 3, LastMessageAt: time.Now(), UpdatedAt: time.Now()}
	raw, _ := json.Marshal(NewConversationEvent(c))
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if got["type"] != TypeConversationUpdated || got["v"] != float64(EventVersion) || got["conversation_id"] != c.ID.String() ||
		got["assignee_id"] != assignee.String() || got["unread_count"] != float64(3) || got["status"] != "open" {
		t.Fatalf("event %s", raw)
	}
}

// verify checks an HS256 JWT the way Centrifugo does and returns its claims.
func verify(t *testing.T, tok, secret string) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", tok)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		t.Fatal("signature does not verify")
	}
	var header, claims map[string]any
	h, _ := base64.RawURLEncoding.DecodeString(parts[0])
	c, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(h, &header); err != nil || header["alg"] != "HS256" {
		t.Fatalf("header %s", h)
	}
	if err := json.Unmarshal(c, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestConnectionToken(t *testing.T) {
	iss, err := NewIssuer(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	iss.now = func() time.Time { return now }

	tok, err := iss.Connection(agent)
	if err != nil {
		t.Fatal(err)
	}
	if !tok.ExpiresAt.Equal(now.Add(TokenTTL)) {
		t.Fatalf("expires %v", tok.ExpiresAt)
	}
	c := verify(t, tok.Token, testSecret)
	if c["sub"] != agent.ID.String() || c["exp"] != float64(now.Add(TokenTTL).Unix()) || c["iat"] != float64(now.Unix()) {
		t.Fatalf("claims %v", c)
	}
	subs, _ := c["subs"].(map[string]any)
	if len(subs) != 2 || subs[PersonalChannel(agent.ID)] == nil || subs[NotificationsChannel] == nil {
		t.Fatalf("server-side subscriptions %v", c["subs"])
	}

	// A different secret does not verify: Centrifugo would refuse it.
	other, _ := NewIssuer("another-secret-0123456789abcdef") // gitleaks:allow (test fixture)
	tok2, _ := other.Connection(agent)
	parts := strings.Split(tok2.Token, ".")
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) == parts[2] {
		t.Fatal("tokens of different secrets must differ")
	}

	if _, err := iss.Connection(&principal.Principal{Kind: principal.KindAPIKey, ID: uuid.New()}); err == nil {
		t.Fatal("API keys must not get connection tokens")
	}
	if _, err := NewIssuer("short"); err == nil {
		t.Fatal("a short secret must be refused")
	}
}

func TestSubscriptionToken(t *testing.T) {
	iss, _ := NewIssuer(testSecret)
	tok, err := iss.Subscription(agent, UnassignedChannel)
	if err != nil {
		t.Fatal(err)
	}
	c := verify(t, tok.Token, testSecret)
	if c["sub"] != agent.ID.String() || c["channel"] != UnassignedChannel {
		t.Fatalf("claims %v", c)
	}
	if exp := int64(c["exp"].(float64)); time.Until(time.Unix(exp, 0)) > TokenTTL || time.Until(time.Unix(exp, 0)) < TokenTTL-time.Minute {
		t.Fatalf("exp %d", exp)
	}
	if _, err := iss.Subscription(agent, CampaignsChannel); !errors.Is(err, ErrChannelForbidden) {
		t.Fatalf("agent got a campaigns token: %v", err)
	}
}

type fakeCentrifugo struct {
	channel, key string
	data         []byte
	err          error
}

func (f *fakeCentrifugo) Publish(_ context.Context, channel string, data []byte, key string) error {
	f.channel, f.data, f.key = channel, data, key
	return f.err
}

func TestPublisher(t *testing.T) {
	f := &fakeCentrifugo{}
	pub := Publisher(f)
	ev := NewMessageEvent(TypeMessageCreated, db.Message{ID: uuid.New(), ContactID: uuid.New(), PhoneNumberID: uuid.New(),
		Direction: "inbound", Type: "text", Status: "received", Body: "secret body", UpdatedAt: time.Now()})
	payload, _ := json.Marshal(ev)
	if err := pub.Publish(context.Background(), outbox.Event{ID: 42, Topic: TopicPrefix + InboxChannel, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if f.channel != InboxChannel || f.key != "outbox:42" || string(f.data) != string(payload) {
		t.Fatalf("published %q %q %s", f.channel, f.key, f.data)
	}
	var got map[string]any
	_ = json.Unmarshal(f.data, &got)
	if got["type"] != TypeMessageCreated || got["v"] != float64(EventVersion) || got["direction"] != "inbound" {
		t.Fatalf("event %v", got)
	}
	if strings.Contains(string(f.data), "secret body") {
		t.Fatal("message bodies must never be published")
	}

	f.err = errors.New("down")
	if err := pub.Publish(context.Background(), outbox.Event{ID: 43, Topic: TopicPrefix + InboxChannel, Payload: payload}); err == nil {
		t.Fatal("a failed publish must be retried by the relay")
	}
	if err := pub.Publish(context.Background(), outbox.Event{ID: 44, Topic: TopicPrefix, Payload: payload}); err == nil {
		t.Fatal("an event without a channel must fail")
	}
}
