package whatsapp

import (
	"errors"
	"testing"
	"time"
)

func TestClassifyConsentRelatedCodes(t *testing.T) {
	cases := []struct {
		code     int
		outcome  Outcome
		deferred bool
		class    string
	}{
		// Ecosystem limit: at least a day later, deferred (no window slot).
		{131049, Retryable, true, FailureEcosystemLimit},
		// The customer stopped marketing in WhatsApp / undeliverable: final.
		{131050, Permanent, false, FailureUserStoppedMarketing},
		{131026, Permanent, false, FailureUndeliverable},
		// Ordinary temporary and rejected errors.
		{130429, Retryable, false, FailureTemporary},
		{131056, Retryable, false, FailureTemporary},
		{131047, Permanent, false, FailureRejectedByMeta},
		{132000, Permanent, false, FailureRejectedByMeta},
	}
	for _, tc := range cases {
		se := Classify(&APIError{Code: tc.code, HTTPStatus: 400}, 1)
		if se.Outcome != tc.outcome || se.Deferred != tc.deferred {
			t.Errorf("%d: outcome %s deferred %v, want %s %v", tc.code, se.Outcome, se.Deferred, tc.outcome, tc.deferred)
		}
		if got := se.FailureClass(); got != tc.class {
			t.Errorf("%d: failure class %q, want %q", tc.code, got, tc.class)
		}
		if got := FailureClassOf(tc.code); got != tc.class {
			t.Errorf("%d: FailureClassOf %q, want %q", tc.code, got, tc.class)
		}
	}
	if se := Classify(&APIError{Code: 131049}, 0); se.RetryAfter < 24*time.Hour {
		t.Fatalf("131049 retried after %s; Meta asks for at least 24 hours", se.RetryAfter)
	}
	transport := &SendError{Outcome: Ambiguous, Err: errors.New("eof")}
	if got := transport.FailureClass(); got != FailurePermanent {
		t.Fatalf("transport failure class %q", got)
	}
}
