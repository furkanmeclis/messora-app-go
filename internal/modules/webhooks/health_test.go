package webhooks

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

// During a rotation's overlap the header carries one v1 per secret; a
// receiver holding either secret accepts it, one holding neither does not,
// and the replay window applies to both.
func TestSignWithPreviousSecret(t *testing.T) {
	body := []byte(`{"id":"e1"}`)
	now := time.Unix(1_800_000_000, 0)
	h := Sign("whsec_new", now, body, "whsec_old")
	if strings.Count(h, "v1=") != 2 || !strings.HasPrefix(h, Sign("whsec_new", now, body)+",v1=") {
		t.Fatalf("header: %s", h)
	}
	for secret, ok := range map[string]bool{"whsec_new": true, "whsec_old": true, "whsec_other": false} {
		if Verify(secret, h, body, 5*time.Minute, now) != ok {
			t.Errorf("%s: want %v", secret, ok)
		}
	}
	if Verify("whsec_old", h, body, 5*time.Minute, now.Add(10*time.Minute)) {
		t.Error("replayed outside the window")
	}
}

// The switch-off rule needs both: enough failures in a row and a run of
// failures old enough, so a busy endpoint's short outage is survived.
func TestShouldDisable(t *testing.T) {
	s := New(nil, nil, Options{Client: NewClient(false)})
	now := time.Now()
	since := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	for _, c := range []struct {
		streak int64
		since  *time.Time
		want   bool
	}{
		{DefaultDisableAfterFailures - 1, since(48 * time.Hour), false},
		{DefaultDisableAfterFailures, since(time.Hour), false}, // many failures in an hour: an outage
		{DefaultDisableAfterFailures, since(25 * time.Hour), true},
		{500, nil, false},
	} {
		if got := s.shouldDisable(db.RecordWebhookFailureRow{FailureStreak: c.streak, FailingSince: c.since}, now); got != c.want {
			t.Errorf("streak %d since %v: %v", c.streak, c.since, got)
		}
	}
	s.SetDisableRule(3, 0)
	if !s.shouldDisable(db.RecordWebhookFailureRow{FailureStreak: 3, FailingSince: since(0)}, now) {
		t.Error("custom rule")
	}
}
