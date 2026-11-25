package offers

import (
	"testing"
	"time"
)

func TestNextStatus(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	h := time.Hour
	cases := []struct {
		cur          string
		starts, ends time.Duration // relative to now
		want         string
	}{
		{StatusDraft, -h, h, StatusDraft},            // drafts ignore the clock
		{StatusDraft, -2 * h, -h, StatusDraft},       // even after their end
		{StatusScheduled, h, 2 * h, StatusScheduled}, // not started yet
		{StatusScheduled, 0, h, StatusActive},        // starts exactly now
		{StatusScheduled, -h, h, StatusActive},
		{StatusScheduled, -2 * h, -h, StatusExpired}, // missed its whole window
		{StatusActive, -h, h, StatusActive},
		{StatusActive, -h, 0, StatusExpired}, // ends exactly now
		{StatusActive, -2 * h, -h, StatusExpired},
		{StatusActive, h, 2 * h, StatusScheduled}, // start moved into the future
		{StatusExpired, -h, h, StatusExpired},     // terminal
	}
	for _, c := range cases {
		got := NextStatus(c.cur, now.Add(c.starts), now.Add(c.ends), now)
		if got != c.want {
			t.Errorf("%s starts %v ends %v: got %s, want %s", c.cur, c.starts, c.ends, got, c.want)
		}
	}
}
