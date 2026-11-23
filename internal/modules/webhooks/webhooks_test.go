package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/platform/safehttp"
)

const testSecret = "whsec_test-secret-for-signatures" // gitleaks:allow (test fixture)

func TestSignAndVerify(t *testing.T) {
	body := []byte(`{"id":"e1","type":"message.received"}`)
	now := time.Unix(1_800_000_000, 0)
	h := Sign(testSecret, now, body)
	if !strings.HasPrefix(h, "t=1800000000,v1=") || len(h) != len("t=1800000000,v1=")+64 {
		t.Fatalf("header format: %s", h)
	}
	// Independent computation of HMAC-SHA256(secret, "1800000000."+body).
	m := hmac.New(sha256.New, []byte(testSecret))
	m.Write([]byte("1800000000." + string(body)))
	if want := "t=1800000000,v1=" + hex.EncodeToString(m.Sum(nil)); h != want {
		t.Fatalf("signature %s, want %s", h, want)
	}
	cases := []struct {
		name   string
		secret string
		header string
		body   []byte
		now    time.Time
		ok     bool
	}{
		{"valid", testSecret, h, body, now, true},
		{"valid within tolerance", testSecret, h, body, now.Add(4 * time.Minute), true},
		{"too old (replay)", testSecret, h, body, now.Add(6 * time.Minute), false},
		{"from the future", testSecret, h, body, now.Add(-6 * time.Minute), false},
		{"wrong secret", "whsec_other", h, body, now, false},
		{"tampered body", testSecret, h, []byte(`{"id":"e2"}`), now, false},
		{"tampered timestamp", testSecret, strings.Replace(h, "t=1800000000", "t=1800000001", 1), body, now, false},
		{"second signature matches", testSecret, "t=1800000000,v1=deadbeef," + h[len("t=1800000000,"):], body, now, true},
		{"missing v1", testSecret, "t=1800000000", body, now, false},
		{"garbage", testSecret, "nonsense", body, now, false},
	}
	for _, tc := range cases {
		if got := Verify(tc.secret, tc.header, tc.body, 5*time.Minute, tc.now); got != tc.ok {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestBackoffSchedule(t *testing.T) {
	var total time.Duration
	for a := 1; a < MaxAttempts; a++ {
		d := Backoff(a)
		base := retrySchedule[a-1]
		if d < base*9/10 || d > base*11/10 {
			t.Errorf("attempt %d: %s outside ±10%% of %s", a, d, base)
		}
		if a > 1 && base <= retrySchedule[a-2] {
			t.Errorf("schedule must grow at attempt %d", a)
		}
		total += base
	}
	if total < 20*time.Hour || total > 26*time.Hour {
		t.Fatalf("retries should span about a day, got %s", total)
	}
	if Backoff(0) < 50*time.Second || Backoff(99) > 11*time.Hour {
		t.Fatal("out-of-range attempts are clamped")
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	h := http.Header{}
	if _, ok := retryAfter(h, now); ok {
		t.Fatal("absent header")
	}
	h.Set("Retry-After", "120")
	if d, ok := retryAfter(h, now); !ok || d != 2*time.Minute {
		t.Fatalf("seconds: %s %v", d, ok)
	}
	h.Set("Retry-After", now.Add(90*time.Second).Format(http.TimeFormat))
	if d, ok := retryAfter(h, now); !ok || d != 90*time.Second {
		t.Fatalf("date: %s %v", d, ok)
	}
	h.Set("Retry-After", "999999")
	if d, _ := retryAfter(h, now); d != maxRetryAfter {
		t.Fatalf("cap: %s", d)
	}
	h.Set("Retry-After", "soon")
	if _, ok := retryAfter(h, now); ok {
		t.Fatal("garbage")
	}
}

func TestClassify(t *testing.T) {
	now := time.Now()
	res := func(code int, hdr ...string) *safehttp.Response {
		h := http.Header{}
		for i := 0; i+1 < len(hdr); i += 2 {
			h.Set(hdr[i], hdr[i+1])
		}
		return &safehttp.Response{StatusCode: code, Header: h, Body: []byte(strings.Repeat("é", 1000)), Duration: 42 * time.Millisecond}
	}
	blocked := &safehttp.BlockedError{Host: "x", Addr: netip.MustParseAddr("10.0.0.1"), Reason: "private address"}
	cases := []struct {
		name    string
		res     *safehttp.Response
		err     error
		attempt int
		status  string
		disable bool
	}{
		{"200", res(200), nil, 1, "succeeded", false},
		{"204", res(204), nil, 3, "succeeded", false},
		{"500 retries", res(500), nil, 1, "retrying", false},
		{"503 retries", res(503), nil, 1, "retrying", false},
		{"429 retries", res(429), nil, 1, "retrying", false},
		{"408 retries", res(408), nil, 1, "retrying", false},
		{"network error retries", nil, errors.New("connection refused"), 1, "retrying", false},
		{"last attempt gives up", res(500), nil, MaxAttempts, "failed", false},
		{"410 disables", res(410), nil, 1, "failed", true},
		{"400 fails at once", res(400), nil, 1, "failed", false},
		{"401 fails at once", res(401), nil, 1, "failed", false},
		{"302 fails at once", res(302), nil, 1, "failed", false},
		{"blocked destination fails at once", nil, blocked, 1, "failed", false},
	}
	for _, tc := range cases {
		o := classify(tc.res, tc.err, tc.attempt, now)
		if o.status != tc.status || (o.disable != "") != tc.disable {
			t.Errorf("%s: status %s disable %q", tc.name, o.status, o.disable)
		}
		if o.status == "retrying" && o.delay <= 0 {
			t.Errorf("%s: retry without delay", tc.name)
		}
		if tc.res != nil && (o.code == nil || *o.code != int32(tc.res.StatusCode) || o.duration == nil || *o.duration != 42) {
			t.Errorf("%s: response not recorded", tc.name)
		}
		if o.body != nil && len(*o.body) > storedBodyBytes+4 {
			t.Errorf("%s: stored body %d bytes", tc.name, len(*o.body))
		}
	}
	o := classify(res(429, "Retry-After", "7"), nil, 1, now)
	if o.delay != 7*time.Second {
		t.Fatalf("Retry-After honoured: %s", o.delay)
	}
	if o := classify(nil, blocked, 1, now); o.errMsg == nil || !strings.Contains(*o.errMsg, "private address") {
		t.Fatalf("blocked reason: %v", o.errMsg)
	}
}

func TestCatalogue(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Catalogue {
		if seen[e.Type] || e.Description == "" || !Known(e.Type) {
			t.Fatalf("bad catalogue entry %+v", e)
		}
		seen[e.Type] = true
	}
	if Known(TypeTest) || Known("message.deleted") {
		t.Fatal("test and unknown types are not subscribable")
	}
	if _, err := eventTypes([]string{TypeContactCreated, TypeMessageReceived, TypeContactCreated}); err != nil {
		t.Fatal(err)
	}
	if got, _ := eventTypes([]string{TypeMessageStatus, TypeMessageReceived, TypeMessageStatus}); len(got) != 2 || got[0] != TypeMessageReceived {
		t.Fatalf("types are sorted and de-duplicated: %v", got)
	}
	if _, err := eventTypes(nil); err == nil {
		t.Fatal("empty subscription")
	}
	if _, err := eventTypes([]string{"webhook.test"}); err == nil {
		t.Fatal("test type is not subscribable")
	}
}

func TestCheckURLRules(t *testing.T) {
	s := &Service{client: safehttp.New(safehttp.Options{Resolver: staticResolver{}})}
	ctx := context.Background()
	for raw, ok := range map[string]bool{
		"https://hooks.example.com/messora":         true,
		"http://hooks.example.com/messora":          false,
		"https://user:pass@hooks.example.com/":      false, // gitleaks:allow (test fixture)
		"https://hooks.example.com/#frag":           false,
		"https://127.0.0.1/":                        false,
		"https://169.254.169.254/latest/meta-data/": false,
		"https://internal.example.com/":             false, // resolves to 10.0.0.1
	} {
		_, err := s.checkURL(ctx, raw)
		if ok != (err == nil) {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

type staticResolver struct{}

func (staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if host == "internal.example.com" {
		return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
}
