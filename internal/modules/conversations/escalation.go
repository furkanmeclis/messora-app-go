package conversations

import (
	"slices"
	"strings"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
)

// Reason is why a conversation was handed to a person
// (conversation_handoffs.reason).
type Reason string

// Handoff reasons.
const (
	ReasonCustomerRequest    Reason = "customer_request"
	ReasonNoAnswer           Reason = "no_answer"
	ReasonLowConfidence      Reason = "low_confidence"
	ReasonCriticalIntent     Reason = "critical_intent"
	ReasonUnauthorizedAction Reason = "unauthorized_action"
	ReasonWindowClosed       Reason = "window_closed"
	ReasonAIError            Reason = "ai_error"
	ReasonManual             Reason = "manual"
)

// IntentHumanRequest is the intent the model uses for "I want a person".
const IntentHumanRequest = "human_request"

// humanWords are whole words that ask for a person (after
// consent.Normalize: lower case, Turkish letters folded). humanStems match
// any word starting with them (Turkish suffixes: temsilciye,
// temsilcinizle …); humanPhrases match as a sequence of words.
var (
	humanWords = []string{
		"insan", "insana", "insanla", "insanin", "insani", "insanlarla",
		"human", "agent", "agents", "operator", "operatore", "operatorle",
		"yetkili", "yetkiliye", "yetkiliyle",
	}
	humanStems   = []string{"temsilci"}
	humanPhrases = []string{
		"gercek biri", "gercek birisi", "gercek birine", "gercek biriyle", "gercek bir kisi", "gercek kisi", "gercek bir insan",
		"canli destek", "canli biri", "musteri hizmetleri",
		"real person", "real human", "live agent", "live person", "speak to someone", "talk to someone", "customer service",
	}
)

// maxHumanRequestScan bounds the messages scanned: a long text quoting the
// word is rarely a request for a person (the model still decides).
const maxHumanRequestScan = 500

// WantsHuman reports whether a customer message asks for a person, in
// Turkish or English, regardless of case, Turkish dotted/dotless i and
// missing Turkish marks. It is the deterministic half of the check; the
// model's intent human_request is the other.
func WantsHuman(text string) bool {
	n := consent.Normalize(text)
	if n == "" || len(n) > maxHumanRequestScan {
		return false
	}
	words := strings.Fields(n)
	for _, w := range words {
		if slices.Contains(humanWords, w) {
			return true
		}
		for _, s := range humanStems {
			if strings.HasPrefix(w, s) {
				return true
			}
		}
	}
	padded := " " + n + " "
	for _, p := range humanPhrases {
		if strings.Contains(padded, " "+p+" ") {
			return true
		}
	}
	return false
}

// Decision is whether an AI turn hands the conversation to a person.
type Decision struct {
	Escalate bool
	Reason   Reason
	Detail   string
	// Actions are the requested actions the profile allows (the others are
	// in Refused).
	Actions []string
	Refused []string
}

// Decide applies the handoff rules to an AI reply, in this order: the
// customer asked for a person (keyword or intent; always on — Meta
// requires it), a critical intent, the model's own escalate flag, an
// action the profile may not take (when the profile hands over for it),
// confidence below the threshold, and an empty reply.
func Decide(r ai.Reply, p ai.Profile, customerText string) Decision {
	d := Decision{}
	for _, a := range r.Actions {
		a = strings.TrimSpace(a)
		if a == "" || slices.Contains(d.Actions, a) || slices.Contains(d.Refused, a) {
			continue
		}
		if p.Allows(a) {
			d.Actions = append(d.Actions, a)
		} else {
			d.Refused = append(d.Refused, a)
		}
	}
	intent := strings.ToLower(strings.TrimSpace(r.Intent))
	esc := func(reason Reason, detail string) Decision {
		d.Escalate, d.Reason, d.Detail = true, reason, detail
		d.Actions = nil // nothing runs when a person takes over
		return d
	}
	switch {
	case WantsHuman(customerText) || intent == IntentHumanRequest:
		return esc(ReasonCustomerRequest, r.EscalateReason)
	case intent != "" && slices.Contains(p.Handoff.OnCriticalIntents, intent):
		return esc(ReasonCriticalIntent, intent)
	case r.Escalate:
		return esc(ReasonNoAnswer, r.EscalateReason)
	case len(d.Refused) > 0 && p.Handoff.OnUnauthorizedAction:
		return esc(ReasonUnauthorizedAction, strings.Join(d.Refused, ", "))
	case p.Handoff.OnLowConfidence != nil && r.Confidence < *p.Handoff.OnLowConfidence:
		return esc(ReasonLowConfidence, "")
	case strings.TrimSpace(r.Reply) == "":
		return esc(ReasonNoAnswer, "empty reply")
	}
	return d
}
