package license

import (
	"time"
)

// Status is what the installation may do.
type Status string

// Statuses. Only Restricted blocks anything: sending messages, starting
// campaigns and running automations stop; data stays readable and
// exportable, and a valid key restores everything at once.
const (
	StatusTrial      Status = "trial"
	StatusActive     Status = "active"
	StatusExpiring   Status = "expiring" // last ExpiryWarning before expiry
	StatusGrace      Status = "grace"    // expired, revoked or unchecked, still working
	StatusRestricted Status = "restricted"
)

// Reasons for grace and restricted states.
const (
	ReasonTrialEnded     = "trial_ended"
	ReasonExpired        = "expired"
	ReasonRevoked        = "revoked"
	ReasonSuperseded     = "superseded"
	ReasonCheckInOverdue = "check_in_overdue"
)

// Policy durations (see docs/ROADMAP.md, Phase 16).
const (
	TrialPeriod    = 14 * 24 * time.Hour
	ExpiryWarning  = 30 * 24 * time.Hour
	ExpiryGrace    = 14 * 24 * time.Hour
	RevokedGrace   = 7 * 24 * time.Hour
	CheckInMaxAge  = 30 * 24 * time.Hour
	CheckInWarning = 14 * 24 * time.Hour
)

// Stored is what the installation remembers between restarts.
type Stored struct {
	Key         string     `json:"key,omitempty"`
	InstanceID  string     `json:"instance_id"`
	InstalledAt time.Time  `json:"installed_at"`
	LicensedAt  *time.Time `json:"licensed_at,omitempty"` // when Key was entered
	LastSeen    time.Time  `json:"last_seen"`             // latest wall clock seen
	LastCheckIn *time.Time `json:"last_check_in,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	Revocation  string     `json:"revocation,omitempty"` // revoked or superseded
	Latest      *Release   `json:"latest,omitempty"`     // newest release the gate offered
}

// Release is a published version.
type Release struct {
	Version    string    `json:"version"`
	ReleasedAt time.Time `json:"released_at"`
	Channel    string    `json:"channel"`
	Notes      string    `json:"notes,omitempty"`
}

// State is the evaluated licence situation.
type State struct {
	Status   Status
	Reason   string
	License  *Payload
	Until    *time.Time // when the current status ends (trial end, expiry, grace end)
	Now      time.Time  // the clock used (never earlier than LastSeen)
	KeyError error      // the stored key did not verify
}

// Allowed reports whether sending and automations may run.
func (s State) Allowed() bool { return s.Status != StatusRestricted }

func at(t time.Time) *time.Time { return &t }

// Evaluate decides the state at now. A clock set back cannot extend a
// licence: the latest time ever seen is used instead.
func Evaluate(now time.Time, st Stored, keys Keyring) State {
	if now.Before(st.LastSeen) {
		now = st.LastSeen
	}
	out := State{Now: now}
	var lic *Payload
	if st.Key != "" {
		p, err := Parse(st.Key, keys)
		if err != nil {
			out.KeyError = err
		} else {
			lic = &p
		}
	}
	if lic == nil {
		end := st.InstalledAt.Add(TrialPeriod)
		if now.Before(end) {
			out.Status, out.Until = StatusTrial, at(end)
		} else {
			out.Status, out.Reason, out.Until = StatusRestricted, ReasonTrialEnded, at(end)
		}
		return out
	}
	out.License = lic

	// Revoked or replaced: a short grace, then restricted.
	if st.RevokedAt != nil {
		reason := ReasonRevoked
		if st.Revocation == ReasonSuperseded {
			reason = ReasonSuperseded
		}
		end := st.RevokedAt.Add(RevokedGrace)
		if now.Before(end) {
			out.Status, out.Reason, out.Until = StatusGrace, reason, at(end)
		} else {
			out.Status, out.Reason, out.Until = StatusRestricted, reason, at(end)
		}
		return out
	}

	// Expiry.
	if lic.ExpiresAt != nil {
		exp := *lic.ExpiresAt
		switch {
		case !now.Before(exp.Add(ExpiryGrace)):
			out.Status, out.Reason, out.Until = StatusRestricted, ReasonExpired, at(exp.Add(ExpiryGrace))
			return out
		case !now.Before(exp):
			out.Status, out.Reason, out.Until = StatusGrace, ReasonExpired, at(exp.Add(ExpiryGrace))
			return out
		}
	}

	// Online licences must reach the gate now and then.
	if lic.CheckIn == CheckInOnline {
		since := st.InstalledAt
		if st.LicensedAt != nil {
			since = *st.LicensedAt
		}
		if st.LastCheckIn != nil && st.LastCheckIn.After(since) {
			since = *st.LastCheckIn
		}
		deadline := since.Add(CheckInMaxAge)
		if !now.Before(deadline) {
			out.Status, out.Reason, out.Until = StatusRestricted, ReasonCheckInOverdue, at(deadline)
			return out
		}
		if !now.Before(since.Add(CheckInWarning)) {
			out.Status, out.Reason, out.Until = StatusGrace, ReasonCheckInOverdue, at(deadline)
			return out
		}
	}

	if lic.ExpiresAt != nil && !now.Before(lic.ExpiresAt.Add(-ExpiryWarning)) {
		out.Status, out.Until = StatusExpiring, lic.ExpiresAt
		return out
	}
	out.Status = StatusActive
	out.Until = lic.ExpiresAt
	return out
}

// UpdatesAllowed reports whether a release may be installed under p.
func (p Payload) UpdatesAllowed(r Release) bool {
	return p.UpdatesUntil == nil || !r.ReleasedAt.After(*p.UpdatesUntil)
}

// CheckInRequest is what an installation sends to the gate.
type CheckInRequest struct {
	Key        string `json:"key"`
	InstanceID string `json:"instance_id"`
	Version    string `json:"version"`
}

// CheckInAnswer is the gate's signed reply ("msg1" envelope).
type CheckInAnswer struct {
	V          int       `json:"v"`
	KeyID      string    `json:"kid"`
	LicenseID  string    `json:"license_id"`
	InstanceID string    `json:"instance_id"`
	Status     string    `json:"status"` // ok, revoked, superseded, unknown
	Reason     string    `json:"reason,omitempty"`
	At         time.Time `json:"at"`
	Latest     *Release  `json:"latest,omitempty"`
}

// OpenAnswer verifies a gate reply.
func OpenAnswer(token string, keys Keyring) (CheckInAnswer, error) {
	var a CheckInAnswer
	err := Open(GatePrefix, token, keys, &a, func() string { return a.KeyID })
	return a, err
}
