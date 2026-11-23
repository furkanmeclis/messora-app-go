package messaging

import (
	"testing"
	"time"
)

func TestServiceWindow(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	cases := []struct {
		last *time.Time
		open bool
	}{
		{nil, false},
		{at(time.Minute), true},
		{at(ServiceWindow), true},
		{at(ServiceWindow + time.Second), false},
	}
	for _, c := range cases {
		if got := WindowOpen(c.last, now); got != c.open {
			t.Errorf("last %v: open=%v, want %v", c.last, got, c.open)
		}
	}
	if WindowExpiresAt(nil) != nil || !WindowExpiresAt(at(time.Hour)).Equal(now.Add(23*time.Hour)) {
		t.Fatal("expiry")
	}
	for typ, want := range map[string]bool{"text": true, "image": true, "button": true, "reaction": false, "system": false, "unsupported": false} {
		if opensWindow(typ) != want {
			t.Errorf("%s opens window: %v", typ, !want)
		}
	}
}
