package consent

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

func TestNormalizeTurkish(t *testing.T) {
	cases := map[string]string{
		"İPTAL":                 "iptal",
		"iptal":                 "iptal",
		"IPTAL":                 "iptal",
		"ıptal":                 "iptal",
		"  İptal!!! ":           "iptal",
		"ABONELİKTEN ÇIK":       "abonelikten cik",
		"abonelikten çık.":      "abonelikten cik",
		"Abonelikten   cik":     "abonelikten cik",
		"BAŞLAT":                "baslat",
		"başlat 🙂":              "baslat",
		"STOP.":                 "stop",
		"Bana tekrar yazmayın!": "bana tekrar yazmayin",
		"ŞÜĞÖÇ şüğöç":           "sugoc sugoc",
		"İPTAL":                "iptal", // İ written decomposed (I + combining dot)
		"UNSUBSCRIBE":           "unsubscribe",
		"un-subscribe":          "un subscribe",
		"":                      "",
		" ?! ":                  "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeywordMatch(t *testing.T) {
	k := DefaultKeywords()
	cases := map[string]string{
		"İPTAL":           KindStop,
		"iptal":           KindStop,
		"Stop":            KindStop,
		"stop!":           KindStop,
		"dur":             KindStop,
		"Abonelikten çık": KindStop,
		"unsubscribe":     KindStop,
		"BAŞLAT":          KindResume,
		"baslat":          KindResume,
		"start":           KindResume,
		"abone ol":        KindResume,
		"Artık mesaj almak istemiyorum, teşekkürler": KindRequest,
		"lütfen bana tekrar yazmayın":                KindRequest,
		"Mesaj istemiyorum":                          KindRequest,
		"abonelikten çıkmak istiyorum":               KindRequest,
		// Not keywords: a stop word inside a sentence, look-alikes.
		"stop the order please":         "",
		"durum nedir":                   "",
		"iptal edilen siparişim nerede": "",
		"mesaj istemiyorumdur":          "",
		"merhaba":                       "",
		"":                              "",
	}
	for in, want := range cases {
		if got := k.Match(in); got != want {
			t.Errorf("Match(%q) = %q, want %q", in, got, want)
		}
	}
	// A long message quoting a phrase is not a request.
	if got := k.Match(strings.Repeat("uzun ", 80) + "mesaj istemiyorum"); got != "" {
		t.Errorf("long message matched %q", got)
	}
}

func TestSettingsValidate(t *testing.T) {
	s := DefaultSettings()
	if err := s.Validate(); err != nil {
		t.Fatalf("defaults invalid: %v", err)
	}
	s.Keywords.Stop = append(s.Keywords.Stop, " stop ", "STOP!")
	if err := s.Validate(); err != nil || len(s.Keywords.Stop) != len(DefaultKeywords().Stop) {
		t.Fatalf("duplicates not folded: %v %v", s.Keywords.Stop, err)
	}
	bad := []func(*Settings){
		func(s *Settings) { s.UnknownMarketing = "maybe" },
		func(s *Settings) { s.Keywords.Stop = []string{"!!"} },
		func(s *Settings) { s.Keywords.Resume = []string{"İptal"} }, // also a stop word
		func(s *Settings) { s.StopMessage = Messages{} },            // confirmations on, no text
		func(s *Settings) { s.Keywords.Request = make([]string, 51) },
	}
	for i, f := range bad {
		s := DefaultSettings()
		f(&s)
		if s.Validate() == nil {
			t.Errorf("case %d: invalid settings accepted", i)
		}
	}
	s = DefaultSettings()
	s.Confirm, s.StopMessage = false, Messages{}
	if err := s.Validate(); err != nil {
		t.Fatalf("no text needed without confirmations: %v", err)
	}
	if (Messages{TR: "tr", EN: "en"}).For("tr") != "tr" || (Messages{TR: "tr", EN: "en"}).For("de") != "en" || (Messages{TR: "tr"}).For("en") != "tr" {
		t.Fatal("message language fallback")
	}
}

func TestStateOfLegacyOptOut(t *testing.T) {
	at := time.Now()
	s := StateOf(db.Contact{MarketingConsent: "unknown", ServiceConsent: "opted_in", OptedOutAt: &at})
	if s.Marketing.Status != OptedOut || s.Service.Status != OptedOut {
		t.Fatalf("legacy opt-out must cover both categories: %+v", s)
	}
	s = StateOf(db.Contact{MarketingConsent: "garbage", ServiceConsent: ""})
	if s.Marketing.Status != Unknown || s.Service.Status != Unknown {
		t.Fatalf("invalid values read as unknown: %+v", s)
	}
}

func TestDecideMatrix(t *testing.T) {
	now := time.Now()
	before, after := now.Add(-time.Hour), now.Add(time.Hour)
	st := func(m, s Status) State {
		return State{Marketing: CategoryStatus{Status: m, UpdatedAt: &now}, Service: CategoryStatus{Status: s, UpdatedAt: &now}}
	}
	allowUnknown, blockUnknown := Policy{}, Policy{BlockUnknownMarketing: true}
	type want struct {
		v      Verdict
		reason string
		hard   bool
	}
	cases := []struct {
		name     string
		state    State
		category string
		policy   Policy
		want     want
	}{
		// Marketing.
		{"marketing opted in", st(OptedIn, Unknown), Marketing, allowUnknown, want{Allow, "", false}},
		{"marketing resubscribed", st(Resubscribed, Unknown), Marketing, blockUnknown, want{Allow, "", false}},
		{"marketing unknown, default policy", st(Unknown, Unknown), Marketing, allowUnknown, want{Warn, ReasonConsentUnknown, false}},
		{"marketing unknown, company blocks", st(Unknown, Unknown), Marketing, blockUnknown, want{Block, ReasonConsentUnknown, false}},
		{"marketing opt-out requested", st(OptOutRequested, OptedIn), Marketing, allowUnknown, want{Block, ReasonOptOutRequested, false}},
		{"marketing opted out", st(OptedOut, OptedIn), Marketing, allowUnknown, want{Block, ReasonOptedOut, true}},
		{"marketing stopped in WhatsApp", func() State { s := st(OptedIn, OptedIn); s.MetaMarketingStoppedAt = &now; return s }(),
			Marketing, allowUnknown, want{Block, ReasonMetaMarketingStopped, true}},
		{"unrecognised category is marketing", st(OptedOut, OptedIn), "promo", allowUnknown, want{Block, ReasonOptedOut, true}},
		// Utility / authentication need service consent.
		{"utility with marketing opt-out", st(OptedOut, Unknown), Utility, blockUnknown, want{Allow, "", false}},
		{"utility stopped in WhatsApp", func() State { s := st(Unknown, Unknown); s.MetaMarketingStoppedAt = &now; return s }(),
			Utility, allowUnknown, want{Allow, "", false}},
		{"utility fully opted out", st(OptedOut, OptedOut), Utility, allowUnknown, want{Block, ReasonOptedOut, true}},
		{"authentication service opted out", st(OptedIn, OptedOut), Authentication, allowUnknown, want{Block, ReasonOptedOut, true}},
		{"utility service opt-out requested", st(OptedIn, OptOutRequested), Utility, allowUnknown, want{Warn, ReasonOptOutRequested, false}},
		// Service replies inside the window.
		{"reply after marketing opt-out", st(OptedOut, Unknown), Service, blockUnknown, want{Allow, "", false}},
		{"reply, service opted out, no new message", func() State { s := st(OptedOut, OptedOut); s.LastInboundAt = &before; return s }(),
			Service, allowUnknown, want{Block, ReasonOptedOut, true}},
		{"reply, service opted out, customer wrote again", func() State { s := st(OptedOut, OptedOut); s.LastInboundAt = &after; return s }(),
			Service, allowUnknown, want{Warn, ReasonRepliedAfterOptOut, false}},
		{"reply, service opt-out requested", st(Unknown, OptOutRequested), Service, allowUnknown, want{Warn, ReasonOptOutRequested, false}},
		// Undeliverable blocks everything.
		{"undeliverable marketing", func() State { s := st(OptedIn, OptedIn); s.UndeliverableAt = &now; return s }(),
			Marketing, allowUnknown, want{Block, ReasonUndeliverable, true}},
		{"undeliverable reply", func() State { s := st(OptedIn, OptedIn); s.UndeliverableAt = &now; s.LastInboundAt = &before; return s }(),
			Service, allowUnknown, want{Block, ReasonUndeliverable, true}},
		{"undeliverable utility", func() State { s := st(OptedIn, OptedIn); s.UndeliverableAt = &now; return s }(),
			Utility, allowUnknown, want{Block, ReasonUndeliverable, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.state, tc.category, tc.policy)
			if d.Verdict != tc.want.v || d.Reason != tc.want.reason || d.Hard != tc.want.hard {
				t.Fatalf("got %+v, want %+v", d, tc.want)
			}
			if (d.Err() == nil) != d.Allowed() {
				t.Fatal("Err must be set exactly for blocks")
			}
		})
	}
	// Hard blocks hold under every policy.
	for _, p := range []Policy{allowUnknown, blockUnknown} {
		for _, cat := range []string{Marketing, Utility, Authentication, Service} {
			d := Decide(st(OptedOut, OptedOut), cat, p)
			if d.Verdict != Block || !d.Hard {
				t.Errorf("%s/%v: full opt-out not a hard block: %+v", cat, p, d)
			}
		}
	}
}

func TestMessageCategory(t *testing.T) {
	for in, want := range map[string]string{"MARKETING": Marketing, "UTILITY": Utility, "AUTHENTICATION": Authentication,
		"utility": Utility, "": Marketing, "SOMETHING_NEW": Marketing} {
		if got := MessageCategory(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
	for _, r := range []string{ReasonOptedOut, ReasonMetaMarketingStopped, ReasonUndeliverable, ReasonOptOutRequested, ReasonConsentUnknown} {
		if ErrorCode(r) == "BLOCKED_BY_CONSENT" || Explain(r) == Explain("x") {
			t.Errorf("reason %s has no specific code or text", r)
		}
	}
}
