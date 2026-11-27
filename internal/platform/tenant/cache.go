package tenant

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Cache keeps one short-lived value per tenant, so an in-process cache of
// settings can never hand one tenant's values to another. Work without a
// tenant is never cached.
type Cache[T any] struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[uuid.UUID]cacheEntry[T]
}

type cacheEntry[T any] struct {
	v       T
	expires time.Time
}

// NewCache returns a cache whose entries live for ttl.
func NewCache[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{ttl: ttl, entries: map[uuid.UUID]cacheEntry[T]{}}
}

// Get returns the cached value of ctx's tenant.
func (c *Cache[T]) Get(ctx context.Context) (T, bool) {
	var zero T
	id, ok := FromCtx(ctx)
	if !ok {
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[id]
	if !ok || !time.Now().Before(e.expires) {
		return zero, false
	}
	return e.v, true
}

// Set stores v for ctx's tenant.
func (c *Cache[T]) Set(ctx context.Context, v T) {
	id, ok := FromCtx(ctx)
	if !ok {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	// Drop expired entries now and then, so tenants that stopped calling
	// do not stay in memory.
	if len(c.entries) > 64 {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
	}
	c.entries[id] = cacheEntry[T]{v: v, expires: now.Add(c.ttl)}
}

// Invalidate forgets ctx's tenant's value (every value without a tenant).
func (c *Cache[T]) Invalidate(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := FromCtx(ctx); ok {
		delete(c.entries, id)
		return
	}
	clear(c.entries)
}

// Clear forgets every tenant's value.
func (c *Cache[T]) Clear() {
	c.mu.Lock()
	clear(c.entries)
	c.mu.Unlock()
}
