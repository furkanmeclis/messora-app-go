package conversations

import (
	"errors"
	"fmt"
)

// Owner is who answers a conversation.
type Owner string

// Owners.
const (
	OwnerNone  Owner = "none"
	OwnerHuman Owner = "human"
	OwnerAI    Owner = "ai"
)

// Mode is how the AI works in a conversation.
type Mode string

// Modes. Off and Suggest belong to conversations a person (or nobody)
// owns; AutoReply and AwaitingApproval to conversations the AI owns.
const (
	ModeOff              Mode = "off"
	ModeSuggest          Mode = "suggest"
	ModeAutoReply        Mode = "auto_reply"
	ModeAwaitingApproval Mode = "awaiting_approval"
)

// AIOwned reports whether m is a mode in which the AI owns the conversation.
func (m Mode) AIOwned() bool { return m == ModeAutoReply || m == ModeAwaitingApproval }

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool {
	return m == ModeOff || m == ModeSuggest || m.AIOwned()
}

// State is a conversation's ownership. PendingHandoff is set while an
// escalation waits for a person to take it.
type State struct {
	Owner          Owner
	Mode           Mode
	PendingHandoff bool
}

// Action is what happens to a conversation's ownership.
type Action string

// Actions of the ownership state machine.
const (
	// HandToAI gives the conversation to the AI (Event.Mode auto_reply or
	// awaiting_approval; empty = auto_reply).
	HandToAI Action = "hand_to_ai"
	// HandToHuman gives it to a person or the pool, by a person's choice.
	HandToHuman Action = "hand_to_human"
	// Escalate is the AI handing over (it asked for a person, was unsure,
	// failed …): the conversation waits for a person.
	Escalate Action = "escalate"
	// Takeover is a person taking the conversation: they pressed "take
	// over", accepted an escalation or replied while the AI owned it.
	Takeover Action = "takeover"
	// SetMode sets the AI mode; an AI-owned mode hands to the AI, off and
	// suggest give an AI-owned conversation to a person.
	SetMode Action = "set_mode"
	// AIUnavailable: the AI was disabled for the contact or the company.
	AIUnavailable Action = "ai_unavailable"
)

// Event is one input of the state machine.
type Event struct {
	Action Action
	// Mode for HandToAI and SetMode.
	Mode Mode
	// TakeoverMode is what the AI does once a person owns the
	// conversation: ModeOff or ModeSuggest (the profile's
	// on_human_takeover). Anything else means off.
	TakeoverMode Mode
	// AIAvailable: AI is on for the company, the number has an active
	// profile and AI is allowed for the contact. Needed to hand to the AI
	// or to suggest.
	AIAvailable bool
}

// ErrAIUnavailable refuses giving work to the AI when it cannot do it.
var ErrAIUnavailable = errors.New("conversations: the AI is not available for this conversation")

// ErrTransition is an action that does not apply to the current state.
var ErrTransition = errors.New("conversations: invalid ownership transition")

// Next returns the state after e. The result equals s when nothing
// changes. It is pure; the service persists the result and records the
// handoff, activity and audit entries.
func Next(s State, e Event) (State, error) {
	takeover := e.TakeoverMode
	if takeover != ModeSuggest || !e.AIAvailable {
		takeover = ModeOff
	}
	human := func(mode Mode) State { return State{Owner: OwnerHuman, Mode: mode} }
	switch e.Action {
	case HandToAI:
		mode := e.Mode
		if mode == "" {
			mode = ModeAutoReply
		}
		if !mode.AIOwned() {
			return s, fmt.Errorf("%w: the AI owns a conversation in auto_reply or awaiting_approval", ErrTransition)
		}
		if !e.AIAvailable {
			return s, ErrAIUnavailable
		}
		return State{Owner: OwnerAI, Mode: mode}, nil
	case HandToHuman:
		if s.Owner == OwnerAI {
			return human(takeover), nil
		}
		mode := s.Mode
		if mode.AIOwned() || (mode == ModeSuggest && !e.AIAvailable) {
			mode = ModeOff
		}
		return human(mode), nil
	case Escalate:
		if s.Owner != OwnerAI {
			return s, fmt.Errorf("%w: only the AI escalates", ErrTransition)
		}
		st := human(takeover)
		st.PendingHandoff = true
		return st, nil
	case Takeover:
		if s.Owner == OwnerAI {
			return human(takeover), nil
		}
		mode := s.Mode
		if mode == ModeSuggest && !e.AIAvailable {
			mode = ModeOff
		}
		return human(mode), nil
	case SetMode:
		switch {
		case e.Mode.AIOwned():
			return Next(s, Event{Action: HandToAI, Mode: e.Mode, AIAvailable: e.AIAvailable})
		case e.Mode == ModeSuggest:
			if !e.AIAvailable {
				return s, ErrAIUnavailable
			}
			owner := s.Owner
			if owner == OwnerAI {
				owner = OwnerHuman
			}
			return State{Owner: owner, Mode: ModeSuggest, PendingHandoff: s.PendingHandoff && owner == OwnerHuman}, nil
		case e.Mode == ModeOff:
			owner := s.Owner
			if owner == OwnerAI {
				owner = OwnerHuman
			}
			return State{Owner: owner, Mode: ModeOff, PendingHandoff: s.PendingHandoff && owner == OwnerHuman}, nil
		}
		return s, fmt.Errorf("%w: unknown mode %q", ErrTransition, e.Mode)
	case AIUnavailable:
		switch {
		case s.Owner == OwnerAI:
			return human(ModeOff), nil
		case s.Mode == ModeSuggest:
			return State{Owner: s.Owner, Mode: ModeOff, PendingHandoff: s.PendingHandoff}, nil
		}
		return s, nil
	}
	return s, fmt.Errorf("%w: unknown action %q", ErrTransition, e.Action)
}
