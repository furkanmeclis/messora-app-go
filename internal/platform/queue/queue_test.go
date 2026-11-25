package queue

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBackoffGrowsAndCaps(t *testing.T) {
	within := func(d, want time.Duration) bool {
		return d >= time.Duration(float64(want)*0.8) && d <= time.Duration(float64(want)*1.2)
	}
	for n, want := range map[int]time.Duration{0: 2 * time.Second, 1: 4 * time.Second, 3: 16 * time.Second, 20: 10 * time.Minute} {
		for range 50 {
			if d := RetryDelay(n, errors.New("x"), nil); !within(d, want) {
				t.Fatalf("n=%d: %s not within 20%% of %s", n, d, want)
			}
		}
	}
}

func TestRetryAfterIsNotAFailure(t *testing.T) {
	ra := fmt.Errorf("wrapped: %w", &RetryAfterError{After: 7 * time.Second, Reason: "rate limited"})
	if IsFailure(ra) {
		t.Fatal("deferral must not count as failure")
	}
	if RetryDelay(5, ra, nil) != 7*time.Second {
		t.Fatal("requested delay must be honoured")
	}
	if !IsFailure(errors.New("boom")) {
		t.Fatal("plain errors are failures")
	}
}

func TestRouting(t *testing.T) {
	if Route(TypeOutboxCleanup) != Default || Route("unknown:type") != Default {
		t.Fatal("routing")
	}
}

func TestCampaignSendsUseTheCampaignsQueue(t *testing.T) {
	for _, tc := range []struct {
		name, taskType, payload, want string
	}{
		{"campaign send", "whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-000000000001","campaign_id":"0193a5a0-0000-7000-8000-000000000002"}`, Campaigns},
		{"inbox/API/automation send", "whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-000000000001"}`, Critical},
		{"empty campaign id", "whatsapp:send", `{"message_id":"x","campaign_id":""}`, Critical},
		{"undecodable payload", "whatsapp:send", `not json`, Critical},
		{"other types ignore campaign_id", "campaign:dispatch", `{"campaign_id":"c"}`, Campaigns},
		{"webhook stays critical", "whatsapp:webhook", `{"campaign_id":"c"}`, Critical},
	} {
		if got := RouteTask(tc.taskType, []byte(tc.payload)); got != tc.want {
			t.Errorf("%s: routed to %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPoliciesAreExplicitAndValid(t *testing.T) {
	known := map[string]bool{}
	for _, q := range AllQueues {
		known[q] = true
	}
	for typ, p := range Policies() {
		if !known[p.Queue] {
			t.Errorf("%s: unknown queue %q", typ, p.Queue)
		}
		if p.MaxRetry < 1 || p.MaxRetry > 25 {
			t.Errorf("%s: max retry %d out of range", typ, p.MaxRetry)
		}
	}
	if p := PolicyFor("unknown:type"); p.Queue != Default || p.MaxRetry != DefaultMaxRetry {
		t.Fatalf("fallback policy %+v", p)
	}
	if PolicyFor("whatsapp:send").MaxRetry != 10 {
		t.Fatal("send budget changed: update docs/ARCHITECTURE.md §11")
	}
}
