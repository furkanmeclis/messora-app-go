package channels

import (
	"strings"
	"testing"
)

func TestWebhookKeys(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		k, err := newKey()
		if err != nil {
			t.Fatal(err)
		}
		if !WellFormedKey(k) || len(k) != WebhookKeyLength {
			t.Fatalf("generated key %q is not well formed", k)
		}
		if seen[k] {
			t.Fatal("duplicate key")
		}
		seen[k] = true
		if len(KeyHash(k)) != 32 {
			t.Fatal("hash is not SHA-256")
		}
	}
	for _, bad := range []string{"", "short", strings.Repeat("a", 51), strings.Repeat("a", 53),
		strings.Repeat("A", 52), strings.Repeat("1", 52), strings.Repeat("a", 51) + "/", strings.Repeat("a", 51) + "="} {
		if WellFormedKey(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	if string(KeyHash("x")) == string(KeyHash("y")) {
		t.Fatal("hashes collide")
	}
}
