package conversations

import (
	"errors"
	"testing"
)

func TestOwnershipStateMachine(t *testing.T) {
	none := State{Owner: OwnerNone, Mode: ModeOff}
	human := State{Owner: OwnerHuman, Mode: ModeOff}
	humanSuggest := State{Owner: OwnerHuman, Mode: ModeSuggest}
	aiAuto := State{Owner: OwnerAI, Mode: ModeAutoReply}
	aiApproval := State{Owner: OwnerAI, Mode: ModeAwaitingApproval}
	waiting := State{Owner: OwnerHuman, Mode: ModeOff, PendingHandoff: true}
	waitingSuggest := State{Owner: OwnerHuman, Mode: ModeSuggest, PendingHandoff: true}

	cases := []struct {
		name string
		from State
		ev   Event
		want State
		err  error
	}{
		{"hand to ai defaults to auto reply", none, Event{Action: HandToAI, AIAvailable: true}, aiAuto, nil},
		{"hand to ai for approval", human, Event{Action: HandToAI, Mode: ModeAwaitingApproval, AIAvailable: true}, aiApproval, nil},
		{"hand to ai refused when unavailable", human, Event{Action: HandToAI}, human, ErrAIUnavailable},
		{"hand to ai with a human mode", human, Event{Action: HandToAI, Mode: ModeSuggest, AIAvailable: true}, human, ErrTransition},
		{"hand to human from ai", aiAuto, Event{Action: HandToHuman, AIAvailable: true}, human, nil},
		{"hand to human keeps suggestions per profile", aiAuto, Event{Action: HandToHuman, TakeoverMode: ModeSuggest, AIAvailable: true}, humanSuggest, nil},
		{"takeover suggestion needs the ai", aiAuto, Event{Action: HandToHuman, TakeoverMode: ModeSuggest}, human, nil},
		{"hand to human between people keeps suggest", humanSuggest, Event{Action: HandToHuman, AIAvailable: true}, humanSuggest, nil},
		{"escalate from ai waits for a person", aiAuto, Event{Action: Escalate}, waiting, nil},
		{"escalate keeps drafting when the profile says so", aiApproval, Event{Action: Escalate, TakeoverMode: ModeSuggest, AIAvailable: true}, waitingSuggest, nil},
		{"only the ai escalates", human, Event{Action: Escalate}, human, ErrTransition},
		{"agent takes over from ai", aiAuto, Event{Action: Takeover, AIAvailable: true}, human, nil},
		{"agent accepts an escalation", waiting, Event{Action: Takeover, AIAvailable: true}, human, nil},
		{"takeover of an unowned conversation", none, Event{Action: Takeover}, human, nil},
		{"takeover drops suggest without ai", humanSuggest, Event{Action: Takeover}, human, nil},
		{"set mode auto reply hands to ai", humanSuggest, Event{Action: SetMode, Mode: ModeAutoReply, AIAvailable: true}, aiAuto, nil},
		{"set mode suggest on ai gives it to a person", aiAuto, Event{Action: SetMode, Mode: ModeSuggest, AIAvailable: true}, humanSuggest, nil},
		{"set mode suggest keeps nobody as owner", none, Event{Action: SetMode, Mode: ModeSuggest, AIAvailable: true}, State{Owner: OwnerNone, Mode: ModeSuggest}, nil},
		{"set mode suggest needs the ai", human, Event{Action: SetMode, Mode: ModeSuggest}, human, ErrAIUnavailable},
		{"set mode off on ai", aiApproval, Event{Action: SetMode, Mode: ModeOff}, human, nil},
		{"set mode off keeps a pending escalation", waitingSuggest, Event{Action: SetMode, Mode: ModeOff}, waiting, nil},
		{"unknown mode", human, Event{Action: SetMode, Mode: "turbo"}, human, ErrTransition},
		{"ai disabled takes it from the ai", aiAuto, Event{Action: AIUnavailable}, human, nil},
		{"ai disabled stops suggestions", humanSuggest, Event{Action: AIUnavailable}, human, nil},
		{"ai disabled leaves a plain conversation", human, Event{Action: AIUnavailable}, human, nil},
		{"unknown action", human, Event{Action: "teleport"}, human, ErrTransition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Next(c.from, c.ev)
			if c.err != nil {
				if !errors.Is(err, c.err) {
					t.Fatalf("error %v, want %v", err, c.err)
				}
				if got != c.from {
					t.Fatalf("state changed on error: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
			// Invariant: the AI owns exactly the AI modes.
			if (got.Owner == OwnerAI) != got.Mode.AIOwned() {
				t.Fatalf("owner %s with mode %s", got.Owner, got.Mode)
			}
		})
	}
}

func TestModes(t *testing.T) {
	for _, m := range []Mode{ModeOff, ModeSuggest, ModeAutoReply, ModeAwaitingApproval} {
		if !m.Valid() {
			t.Fatalf("%s invalid", m)
		}
	}
	if Mode("active").Valid() || ModeSuggest.AIOwned() || !ModeAwaitingApproval.AIOwned() {
		t.Fatal("mode classification")
	}
}
