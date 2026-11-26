package metacost

import (
	"testing"
	"time"
)

func TestChargedRules2026(t *testing.T) {
	sep := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	oct := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		category string
		at       time.Time
		window   bool
		service  int64
		want     bool
	}{
		{"marketing", sep, true, 0, true},
		{"MARKETING", oct, false, 0, true},
		{"authentication", oct, true, 0, true},
		{"utility", sep, true, 0, false}, // free inside the window until 2026-09-30
		{"utility", sep, false, 0, true},
		{"utility", oct, true, 0, true}, // charged again from 2026-10-01
		{"service", sep, true, 5000, false},
		{"service", oct, true, 999, false}, // within the 1,000 free tier
		{"service", oct, true, 1000, true},
		{"something_new", oct, true, 0, true}, // never free by guess
	}
	for _, c := range cases {
		if got := Charged(c.category, c.at, c.window, c.service); got != c.want {
			t.Errorf("%s at %s window=%v service=%d: got %v", c.category, c.at.Format(time.DateOnly), c.window, c.service, got)
		}
	}
}
