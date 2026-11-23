package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Request headers sent with every delivery.
const (
	HeaderSignature = "Messora-Signature"
	HeaderEventID   = "Messora-Event-Id"
	HeaderEventType = "Messora-Event-Type"
	HeaderDelivery  = "Messora-Delivery-Id"
	HeaderAttempt   = "Messora-Delivery-Attempt"
)

// Sign returns the Messora-Signature value for body sent at ts:
// "t=<unix seconds>,v1=<hex HMAC-SHA256(secret, "<t>.<body>")>".
func Sign(secret string, ts time.Time, body []byte) string {
	t := strconv.FormatInt(ts.Unix(), 10)
	return "t=" + t + ",v1=" + hex.EncodeToString(mac(secret, t, body))
}

func mac(secret, t string, body []byte) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(t))
	m.Write([]byte{'.'})
	m.Write(body)
	return m.Sum(nil)
}

// Verify checks a Messora-Signature header the way a receiver should: the
// HMAC must match (constant-time) and the timestamp must be within
// tolerance of now, which bounds replays. Several v1 values are accepted.
func Verify(secret, header string, body []byte, tolerance time.Duration, now time.Time) bool {
	var t string
	var sigs [][]byte
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			t = v
		case "v1":
			if b, err := hex.DecodeString(v); err == nil {
				sigs = append(sigs, b)
			}
		}
	}
	sec, err := strconv.ParseInt(t, 10, 64)
	if err != nil || len(sigs) == 0 {
		return false
	}
	if d := now.Sub(time.Unix(sec, 0)); d > tolerance || d < -tolerance {
		return false
	}
	want := mac(secret, t, body)
	for _, s := range sigs {
		if hmac.Equal(s, want) {
			return true
		}
	}
	return false
}

// MaxAttempts is how often a delivery is tried before it fails for good.
const MaxAttempts = 10

// retrySchedule is the wait after attempt n (index n-1); the nine waits add
// up to about 24 hours.
var retrySchedule = []time.Duration{
	time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 4 * time.Hour, 6 * time.Hour, 10 * time.Hour,
}

// maxRetryAfter caps a receiver's Retry-After.
const maxRetryAfter = 12 * time.Hour

// Backoff returns the wait after the given (1-based) failed attempt, with
// ±10% jitter so failing receivers are not hit in lockstep.
func Backoff(attempt int) time.Duration {
	i := min(max(attempt, 1), len(retrySchedule)) - 1
	jitter := 0.9 + rand.Float64()*0.2 //nolint:gosec // jitter, not security
	return time.Duration(float64(retrySchedule[i]) * jitter)
}

// retryAfter parses a Retry-After header (seconds or HTTP date), capped.
func retryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	var d time.Duration
	if n, err := strconv.Atoi(v); err == nil {
		d = time.Duration(n) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	} else {
		return 0, false
	}
	return min(max(d, 0), maxRetryAfter), true
}
