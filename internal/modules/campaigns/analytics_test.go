package campaigns

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

func TestAttributionWindow(t *testing.T) {
	sent := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	number, other := uuid.New(), uuid.New()
	a := attribution{RecipientID: uuid.New(), CampaignID: uuid.New(), PhoneNumberID: number, SentAt: sent}
	week := 7 * 24 * time.Hour
	cases := []struct {
		name   string
		at     time.Time
		number *uuid.UUID
		window time.Duration
		want   bool
	}{
		{"reply right after", sent.Add(time.Minute), &number, week, true},
		{"at the send", sent, &number, week, true},
		{"before the send", sent.Add(-time.Second), &number, week, false},
		{"last moment of the window", sent.Add(week), &number, week, true},
		{"after the window", sent.Add(week + time.Second), &number, week, false},
		{"shorter window", sent.Add(2 * 24 * time.Hour), &number, 24 * time.Hour, false},
		{"other number", sent.Add(time.Minute), &other, week, false},
		{"opt-out: any number", sent.Add(time.Hour), nil, week, true},
	}
	for _, c := range cases {
		if got := attributable(a, c.at, c.number, c.window); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestReactionMapping(t *testing.T) {
	m := []byte(`{"YES_PAYLOAD":"positive","Hayır":"negative","Belki":"neutral","bad":"maybe"}`)
	cases := []struct{ payload, text, want string }{
		{"YES_PAYLOAD", "Evet", ReactionPositive},
		{"", "Hayır", ReactionNegative},
		{"OTHER", "hayır", ReactionNegative}, // text, case-insensitive
		{"", "Belki", ReactionNeutral},
		{"", "bad", ""}, // invalid label is ignored
		{"", "Unmapped", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := ReactionFor(m, c.payload, c.text); got != c.want {
			t.Errorf("%q/%q: got %q want %q", c.payload, c.text, got, c.want)
		}
	}
	if got := ReactionFor(nil, "x", "y"); got != "" {
		t.Errorf("nil map: %q", got)
	}
}

func TestQuickReplyFromInbound(t *testing.T) {
	btn, _ := json.Marshal(map[string]any{"type": "button", "button": map[string]string{"payload": "P1", "text": "Evet"}})
	p, tx := quickReply(db.Message{Type: "button", Payload: btn})
	if p != "P1" || tx != "Evet" {
		t.Fatalf("button: %q %q", p, tx)
	}
	inter, _ := json.Marshal(map[string]any{"type": "interactive", "interactive": map[string]any{"type": "button_reply",
		"button_reply": map[string]string{"id": "id1", "title": "Hayır"}}})
	p, tx = quickReply(db.Message{Type: "interactive", Payload: inter})
	if p != "id1" || tx != "Hayır" {
		t.Fatalf("interactive: %q %q", p, tx)
	}
	if p, tx = quickReply(db.Message{Type: "text", Payload: []byte(`{"text":{"body":"hi"}}`)}); p != "" || tx != "" {
		t.Fatal("text is no quick reply")
	}
}

func TestCountsOfKeepsLegacyKeys(t *testing.T) {
	k := Counters{Recipients: 5, RPending: 1, RQueued: 3, RSkipped: 1, MSent: 2, MDelivered: 1, Sent: 3, Delivered: 1,
		Replied: 1, Positive: 1, OBlocked: 1, Cost: "0.120000"}
	c := countsOf(k)
	if c["recipients"] != 5 || c["recipients_queued"] != 3 || c["sent"] != 2 || c["delivered"] != 1 {
		t.Fatalf("legacy keys: %v", c)
	}
	if _, ok := c["failed"]; ok {
		t.Fatal("zero message statuses are omitted as before")
	}
	if c["a_sent"] != 3 || c["a_replied"] != 1 || c["a_positive"] != 1 || c["a_blocked"] != 1 || c["a_cost_micros"] != 120000 {
		t.Fatalf("analytics keys: %v", c)
	}
	if _, ok := c["a_failed"]; !ok {
		t.Fatal("analytics keys are always present")
	}
}

func TestRatesDefinitions(t *testing.T) {
	r := RatesOf(Counters{Recipients: 110, OBlocked: 10, Sent: 100, Delivered: 80, Read: 40, Replied: 20, Positive: 8, Negative: 4, OptedOut: 2})
	check := func(name string, got *float64, want float64) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s: %v want %v", name, got, want)
		}
	}
	check("sent", r.Sent, 1)
	check("delivery", r.Delivery, 0.8)
	check("read", r.Read, 0.5)
	check("reply", r.Reply, 0.25)
	check("positive", r.Positive, 0.1)
	check("negative", r.Negative, 0.05)
	check("opt-out", r.OptOut, 0.025)
	if z := RatesOf(Counters{}); z.Sent != nil || z.Delivery != nil {
		t.Fatal("no denominator: nil rates")
	}
}

func TestAttributionWholeSecondTimestamps(t *testing.T) {
	sent := time.Date(2026, 11, 20, 10, 0, 0, 700_000_000, time.UTC)
	a := attribution{SentAt: sent}
	if !attributable(a, sent.Truncate(time.Second), nil, time.Hour) {
		t.Fatal("a reply stamped in the send's second is after it")
	}
}
