package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// fakeTx records outbox inserts (topic, payload) instead of writing them.
type fakeTx struct {
	rows []struct {
		channel string
		body    map[string]any
	}
}

func (f *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	var body map[string]any
	_ = json.Unmarshal(args[1].([]byte), &body)
	topic := args[0].(string)
	f.rows = append(f.rows, struct {
		channel string
		body    map[string]any
	}{topic[len(TopicPrefix):], body})
	return idRow(len(f.rows))
}

type idRow int

func (r idRow) Scan(dest ...any) error {
	*(dest[0].(*int64)) = int64(r)
	return nil
}

func (f *fakeTx) channels() []string {
	out := make([]string, len(f.rows))
	for i, r := range f.rows {
		out[i] = r.channel
	}
	return out
}

func conversation(assignee *uuid.UUID) db.Conversation {
	return db.Conversation{ID: uuid.Must(uuid.NewV7()), ContactID: uuid.New(), PhoneNumberID: uuid.New(), Status: "open",
		AssigneeID: assignee, LastMessageAt: time.Now(), UpdatedAt: time.Now()}
}

func TestConversationRouting(t *testing.T) {
	ctx := context.Background()
	a, b := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	unassigned, mine := conversation(nil), conversation(&a)
	reassigned := mine
	reassigned.AssigneeID = &b
	released := mine
	released.AssigneeID = nil
	taken := unassigned
	taken.AssigneeID = &a

	for _, tc := range []struct {
		name          string
		before, after db.Conversation
		want          []string
	}{
		{"unassigned", unassigned, unassigned, []string{InboxChannel, UnassignedChannel, ConversationChannel(unassigned.ID)}},
		{"assigned", mine, mine, []string{InboxChannel, PersonalChannel(a), ConversationChannel(mine.ID)}},
		{"reassigned: the previous assignee hears it", mine, reassigned,
			[]string{InboxChannel, PersonalChannel(b), ConversationChannel(mine.ID), PersonalChannel(a)}},
		{"released to the pool", mine, released,
			[]string{InboxChannel, UnassignedChannel, ConversationChannel(mine.ID), PersonalChannel(a)}},
		{"taken from the pool", unassigned, taken,
			[]string{InboxChannel, PersonalChannel(a), ConversationChannel(unassigned.ID), UnassignedChannel}},
	} {
		tx := &fakeTx{}
		if err := PublishConversationChange(ctx, tx, tc.before, tc.after); err != nil {
			t.Fatal(err)
		}
		if got := tx.channels(); !slices.Equal(got, tc.want) {
			t.Errorf("%s: channels %v, want %v", tc.name, got, tc.want)
		}
		// One event: the same ID on every channel, with the new state.
		id := tx.rows[0].body["id"]
		for _, r := range tx.rows {
			if r.body["id"] != id || r.body["type"] != TypeConversationUpdated || r.body["v"] != float64(EventVersion) || r.body["ts"] == nil {
				t.Errorf("%s: %s got %v", tc.name, r.channel, r.body)
			}
			want := any(nil)
			if tc.after.AssigneeID != nil {
				want = tc.after.AssigneeID.String()
			}
			if r.body["assignee_id"] != want {
				t.Errorf("%s: %s assignee %v", tc.name, r.channel, r.body["assignee_id"])
			}
		}
	}
}

func TestMessageRouting(t *testing.T) {
	ctx := context.Background()
	a := uuid.Must(uuid.NewV7())
	m := db.Message{ID: uuid.New(), ContactID: uuid.New(), PhoneNumberID: uuid.New(), Direction: "inbound", Type: "text", Status: "received", UpdatedAt: time.Now()}

	tx := &fakeTx{}
	c := conversation(&a)
	if err := PublishMessage(ctx, tx, NewMessageEvent(TypeMessageStatus, m), &c); err != nil {
		t.Fatal(err)
	}
	if want := []string{InboxChannel, PersonalChannel(a), ConversationChannel(c.ID)}; !slices.Equal(tx.channels(), want) {
		t.Fatalf("assigned: %v", tx.channels())
	}
	if tx.rows[0].body["conversation_id"] != c.ID.String() || tx.rows[0].body["id"] != tx.rows[2].body["id"] {
		t.Fatalf("event %v", tx.rows[0].body)
	}

	// Without a conversation only read_all users hear of it.
	tx = &fakeTx{}
	if err := PublishMessage(ctx, tx, NewMessageEvent(TypeMessageCreated, m), nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tx.channels(), []string{InboxChannel}) || tx.rows[0].body["conversation_id"] != nil {
		t.Fatalf("no conversation: %v %v", tx.channels(), tx.rows)
	}

	// Every channel an agent can subscribe to carries only visible
	// conversations.
	other := uuid.Must(uuid.NewV7())
	for _, assignee := range []*uuid.UUID{nil, &agent.ID, &other} {
		for _, ch := range ConversationChannels(conversation(assignee)) {
			visible := assignee == nil || *assignee == agent.ID
			if strings.HasPrefix(ch, "conversation:") {
				continue // checked per conversation (TestAuthorizeConversationAccess)
			}
			if err := Authorize(agent, ch); (err == nil) && !visible {
				t.Errorf("agent may read %s, which announces a conversation assigned to someone else", ch)
			}
		}
	}
}

func TestPublishNeedsEnvelope(t *testing.T) {
	tx := &fakeTx{}
	if err := Publish(context.Background(), tx, InboxChannel, MessageEvent{}); err == nil {
		t.Fatal("an event without an envelope must be refused")
	}
	if err := Publish(context.Background(), tx, "", NewConversationEvent(conversation(nil))); err == nil {
		t.Fatal("an empty channel must be refused")
	}
	e1, e2 := NewEnvelope(TypeAIStatus), NewEnvelope(TypeAIStatus)
	if e1.ID == e2.ID || e1.V != EventVersion || e1.TS.Location() != time.UTC {
		t.Fatalf("envelopes %v %v", e1, e2)
	}
}

func TestSync(t *testing.T) {
	ctx := context.Background()
	since := time.Now().Add(-time.Minute)
	var scopes []ConversationScope
	n := 1
	s := Syncer{
		Access: func(_ context.Context, _ *principal.Principal, id uuid.UUID) (bool, error) { return false, nil },
		Conversations: func(_ context.Context, _ *principal.Principal, scope ConversationScope, at time.Time, limit int) ([]db.Conversation, error) {
			if !at.Equal(since) || limit != SyncConversationLimit+1 {
				t.Fatalf("since %v limit %d", at, limit)
			}
			scopes = append(scopes, scope)
			out := make([]db.Conversation, n)
			for i := range out {
				out[i] = conversation(nil)
			}
			return out, nil
		},
		Campaigns: func(_ context.Context, id *uuid.UUID, _ time.Time, limit int) ([]CampaignEvent, error) {
			return []CampaignEvent{NewCampaignEvent(db.Campaign{ID: uuid.New(), Status: "running"}, nil)}, nil
		},
	}
	evs, more, err := s.Sync(ctx, agent, UnassignedChannel, since)
	if err != nil || more || len(evs) != 1 || evs[0].envelope().Type != TypeConversationUpdated {
		t.Fatalf("unassigned: %v %v %v", evs, more, err)
	}
	if _, _, err := s.Sync(ctx, agent, PersonalChannel(agent.ID), since); err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 2 || !scopes[0].Unassigned || scopes[1].Assignee == nil || *scopes[1].Assignee != agent.ID {
		t.Fatalf("scopes %+v", scopes)
	}
	// The channel rules are those of the subscription token.
	for _, ch := range []string{InboxChannel, PersonalChannel(owner.ID), CampaignsChannel, ConversationChannel(uuid.New())} {
		if _, _, err := s.Sync(ctx, agent, ch, since); !errors.Is(err, ErrChannelForbidden) {
			t.Errorf("agent synced %s: %v", ch, err)
		}
	}
	if len(scopes) != 2 {
		t.Fatal("a refused channel must not be queried")
	}
	n = SyncConversationLimit + 1
	evs, more, err = s.Sync(ctx, owner, InboxChannel, since)
	if err != nil || !more || len(evs) != SyncConversationLimit {
		t.Fatalf("overflow: %d %v %v", len(evs), more, err)
	}
	evs, _, err = s.Sync(ctx, owner, CampaignsChannel, since)
	if err != nil || len(evs) != 1 || evs[0].envelope().Type != TypeCampaignUpdated {
		t.Fatalf("campaigns: %v %v", evs, err)
	}
	if evs, _, err := s.Sync(ctx, owner, NotificationsChannel, since); err != nil || len(evs) != 0 {
		t.Fatalf("notifications: %v %v", evs, err)
	}
}
