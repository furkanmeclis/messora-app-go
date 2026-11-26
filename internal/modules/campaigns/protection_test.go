package campaigns

import "testing"

func TestSpamThresholds(t *testing.T) {
	st := DefaultAnalyticsSettings()
	cases := []struct {
		name string
		sg   Signals
		rule string
	}{
		{"quiet", Signals{Attempts: 200, Ecosystem: 10, OptedOut: 2, StateRules: true, NumberQuality: "GREEN"}, ""},
		{"131048 at once", Signals{Saw131048: true}, RuleSpamRateLimit},
		{"restriction", Signals{AccountRestriction: "ACCOUNT_RESTRICTION"}, RuleAccountRestriction},
		{"ecosystem above 10%", Signals{Attempts: 200, Ecosystem: 21}, RuleEcosystemRate},
		{"ecosystem at 10%", Signals{Attempts: 200, Ecosystem: 20}, ""},
		{"too few attempts", Signals{Attempts: 49, Ecosystem: 49}, ""},
		{"opt-out above 2%", Signals{Attempts: 100, OptedOut: 3}, RuleOptOutRate},
		{"quality red", Signals{StateRules: true, NumberQuality: "RED"}, RuleNumberQuality},
		{"quality yellow ignored at red", Signals{StateRules: true, NumberQuality: "YELLOW"}, ""},
		{"flagged counts as red", Signals{StateRules: true, NumberQuality: "flagged"}, RuleNumberQuality},
		{"state rules off after resume", Signals{StateRules: false, NumberQuality: "RED"}, ""},
		{"template paused", Signals{StateRules: true, TemplateStatus: "PAUSED"}, RuleTemplatePaused},
		{"template disabled", Signals{StateRules: true, TemplateStatus: "DISABLED"}, RuleTemplatePaused},
		{"template quality red", Signals{StateRules: true, TemplateQuality: "RED"}, RuleTemplateQuality},
	}
	for _, c := range cases {
		trip := Evaluate(st, c.sg)
		got := ""
		if trip != nil {
			got = trip.Rule
		}
		if got != c.rule {
			t.Errorf("%s: got %q want %q", c.name, got, c.rule)
		}
	}
	trip := Evaluate(st, Signals{Attempts: 200, Ecosystem: 30})
	if trip.Detail["rate_pct"] != 15.0 || trip.Detail["threshold_pct"] != 10.0 || trip.Detail["count"] != int64(30) {
		t.Fatalf("detail %v", trip.Detail)
	}

	yellow := st
	yellow.QualityStop = QualityStopYellow
	if Evaluate(yellow, Signals{StateRules: true, NumberQuality: "YELLOW"}) == nil {
		t.Fatal("yellow stop level")
	}
	off := st
	off.Enabled = false
	if Evaluate(off, Signals{Saw131048: true}) != nil {
		t.Fatal("disabled protection never trips")
	}
	no := st
	no.StopOn131048 = false
	if Evaluate(no, Signals{Saw131048: true}) != nil {
		t.Fatal("131048 rule off")
	}
}

func TestProtectionSettingsValidate(t *testing.T) {
	s := DefaultAnalyticsSettings()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mut := range []func(*AnalyticsSettings){
		func(s *AnalyticsSettings) { s.Window = 10 },
		func(s *AnalyticsSettings) { s.MinAttempts = 500 },
		func(s *AnalyticsSettings) { s.EcosystemRatePct = 0 },
		func(s *AnalyticsSettings) { s.QualityStop = "green" },
		func(s *AnalyticsSettings) { s.AttributionDays = 31 },
	} {
		c := DefaultAnalyticsSettings()
		mut(&c)
		if c.Validate() == nil {
			t.Fatalf("invalid settings accepted: %+v", c)
		}
	}
}
