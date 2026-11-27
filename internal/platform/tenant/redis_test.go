package tenant

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRedisKeys(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ka := RedisKey(WithID(context.Background(), a), "rl:api:user:x")
	kb := RedisKey(WithID(context.Background(), b), "rl:api:user:x")
	if ka == kb || ka != "messora:t:{"+a.String()+"}:rl:api:user:x" {
		t.Fatalf("keys %q %q", ka, kb)
	}
	// The tenant is the hash tag: every key of a tenant is in one slot.
	if !strings.Contains(ka, "{"+a.String()+"}") || strings.Count(ka, "{") != 1 {
		t.Fatalf("hash tag of %q", ka)
	}
	// Work without a tenant never shares a tenant's keys.
	none := RedisKey(System(context.Background()), "x")
	if none != "messora:t:{-}:x" || none == RedisKey(WithID(context.Background(), a), "x") {
		t.Fatalf("tenant-less key %q", none)
	}
	if GlobalRedisKey("mfa:abc") != "messora:mfa:abc" || strings.HasPrefix(GlobalRedisKey("x"), "messora:t:") {
		t.Fatal("global keys must stay outside every tenant")
	}
}
