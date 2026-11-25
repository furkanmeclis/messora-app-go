package campaigns

import (
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

func TestTierLimit(t *testing.T) {
	for tier, want := range map[string]int64{
		"TIER_250": 250, "TIER_2K": 2000, "TIER_10K": 10_000, "TIER_100K": 100_000, "TIER_1K": 1000, "tier_50": 50,
		"TIER_UNLIMITED": 0, "": 0, "garbage": 0, "TIER_": 0,
	} {
		if got := TierLimit(tier); got != want {
			t.Errorf("%q: %d, want %d", tier, got, want)
		}
	}
}

func TestPreviousCampaignClasses(t *testing.T) {
	id := uuid.New()
	ok := []string{ClassTemporary, ClassEcosystemLimit, ClassNeverSent}
	if err := (&PreviousCampaign{CampaignID: id, Classes: ok}).validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{nil, {ClassUserStoppedMarketing}, {ClassBlockedByConsent}, {"reached"}, {ClassTemporary, ClassTemporary}} {
		if err := (&PreviousCampaign{CampaignID: id, Classes: bad}).validate(); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if err := (&PreviousCampaign{Classes: ok}).validate(); err == nil {
		t.Error("missing campaign accepted")
	}
	// An audience is exactly one kind.
	if err := (Audience{All: true, PreviousCampaign: &PreviousCampaign{CampaignID: id, Classes: ok}}).validate(); err == nil {
		t.Error("two audiences accepted")
	}
}

func TestOutcome(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, tc := range []struct {
		state  string
		status *string
		want   string
	}{
		{"pending", nil, "pending"}, {"retry", s("failed"), "retry_scheduled"}, {"skipped", nil, "skipped"},
		{"queued", s("held"), "in_flight"}, {"queued", s("sending"), "in_flight"}, {"queued", s("delivered"), "delivered"},
		{"queued", s("failed"), "failed"}, {"queued", s("deferred"), "deferred"}, {"cancelled", nil, "cancelled"},
	} {
		if got := Outcome(tc.state, tc.status); got != tc.want {
			t.Errorf("%s/%v: %s, want %s", tc.state, tc.status, got, tc.want)
		}
	}
}

func TestETAAndRemaining(t *testing.T) {
	rate := 4.0
	c := db.Campaign{Status: "running", SendRate: &rate}
	counts := map[string]int64{"recipients_pending": 30, "recipients_retry": 2, "pending": 5, "sending": 1, "held": 2, "sent": 100}
	if remaining(counts) != 40 {
		t.Fatalf("remaining %d", remaining(counts))
	}
	if e := eta(c, counts); e == nil || *e != 10 {
		t.Fatalf("eta %v", e)
	}
	c.Status = "paused"
	if eta(c, counts) != nil {
		t.Fatal("paused campaigns have no ETA")
	}
}

func TestSettingsValidation(t *testing.T) {
	p := func(s string) *string { return &s }
	f := func(v float64) *float64 { return &v }
	if (Settings{Priority: p("high"), MaxMPS: f(12.5)}).validate() != nil {
		t.Fatal("valid settings refused")
	}
	for _, st := range []Settings{{Priority: p("urgent")}, {MaxMPS: f(0)}, {MaxMPS: f(5000)}, {Name: p("  ")}} {
		if st.validate() == nil {
			t.Errorf("%+v accepted", st)
		}
	}
	if !IsPaused("paused_limit") || !IsPaused("paused") || IsPaused("running") {
		t.Fatal("IsPaused")
	}
}
