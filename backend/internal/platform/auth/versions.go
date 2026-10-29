package auth

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// VersionCache caches users.token_version for the access-token revocation check
// (SECURITY L2). A bump in this process invalidates the entry immediately; other
// processes (worker, a second API replica) see it after at most TTL.
type VersionCache struct {
	load func(ctx context.Context, userID uuid.UUID) (int, error)
	ttl  time.Duration
	max  int
	now  func() time.Time

	mu      sync.Mutex
	entries map[uuid.UUID]versionEntry
}

type versionEntry struct {
	v  int
	at time.Time
}

// DefaultVersionTTL bounds how long another process may accept a revoked access token.
const DefaultVersionTTL = 5 * time.Second

// NewVersionCache caches load results for ttl (<=0: DefaultVersionTTL), at most 100k users.
func NewVersionCache(load func(ctx context.Context, userID uuid.UUID) (int, error), ttl time.Duration) *VersionCache {
	if ttl <= 0 {
		ttl = DefaultVersionTTL
	}
	return &VersionCache{load: load, ttl: ttl, max: 100_000, now: time.Now, entries: map[uuid.UUID]versionEntry{}}
}

// TokenVersion implements VersionSource.
func (c *VersionCache) TokenVersion(ctx context.Context, userID uuid.UUID) (int, error) {
	now := c.now()
	c.mu.Lock()
	e, ok := c.entries[userID]
	c.mu.Unlock()
	if ok && now.Sub(e.at) < c.ttl {
		return e.v, nil
	}
	v, err := c.load(ctx, userID)
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	if len(c.entries) >= c.max {
		clear(c.entries) // bounded; a dropped entry is just reloaded
	}
	c.entries[userID] = versionEntry{v: v, at: now}
	c.mu.Unlock()
	return v, nil
}

// Invalidate drops the cached version of userID (call after bumping it).
func (c *VersionCache) Invalidate(userID uuid.UUID) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()
}
