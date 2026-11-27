package tenant

import (
	"context"

	"github.com/google/uuid"
)

// Redis keys (docs/ARCHITECTURE.md §10.2).
//
// Everything a tenant's work keeps in Redis — API rate limits per session
// and API key, the send scheduler's buckets and shares, AIMD rates, caches
// and locks of tenant work — lives under the tenant's prefix:
//
//	messora:t:{<tenant_id>}:<rest>
//
// The tenant ID is a hash tag, so on a Redis Cluster all keys of a tenant
// share a slot and multi-key Lua scripts (the send scheduler) keep working.
// Keys of work without a tenant use "messora:t:{-}:", which no tenant can
// produce.
//
// Person-global keys stay outside any tenant (GlobalRedisKey): a person
// signs in to several tenants with one account (WP2), so login lockout per
// e-mail address, the 2FA challenge and TOTP replay protection must count
// across tenants. Platform state (the scheduler's leader lease) is global
// too.
//
// Identity (WP2) builds its keys here too: rate limits (rl:login:*,
// rl:reset:*, rl:reauth:*, rl:password:*, rl:signup:*, rl:resend:*), TOTP
// replay protection (totp:used:*) and workspace hand-over tokens
// (handoff:*) are global; a workspace sign-in's second-factor challenge
// and an e-mail change confirmation live under the workspace's prefix, the
// admin panel's challenge under mfa:platform:*.

// RedisPrefix returns the key prefix of tenant id's Redis keys.
func RedisPrefix(id uuid.UUID) string {
	if id == uuid.Nil {
		return "messora:t:{-}:"
	}
	return "messora:t:{" + id.String() + "}:"
}

// RedisKey returns key namespaced to ctx's tenant.
func RedisKey(ctx context.Context, key string) string { return RedisPrefix(ID(ctx)) + key }

// GlobalRedisKey returns a person-global or platform key, deliberately
// shared by every tenant.
func GlobalRedisKey(key string) string { return "messora:" + key }
