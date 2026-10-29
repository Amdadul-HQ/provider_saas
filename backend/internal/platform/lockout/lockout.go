// Package lockout counts failed attempts per key (login identifier, client IP, invite)
// and locks a key for a while after too many failures (SECURITY H6). The in-memory
// implementation is bounded (LRU); with REDIS_URL set, state is shared across API
// instances through Redis.
package lockout

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// Policy: MaxFailures failures within Window lock the key for LockFor.
type Policy struct {
	MaxFailures int
	Window      time.Duration
	LockFor     time.Duration
}

// DefaultLogin is the login policy: 5 failures -> 15 minute lock.
var DefaultLogin = Policy{MaxFailures: 5, Window: 15 * time.Minute, LockFor: 15 * time.Minute}

// Limiter tracks failures per key.
type Limiter interface {
	// Locked reports whether key is locked and for how long.
	Locked(ctx context.Context, key string) (bool, time.Duration, error)
	// Fail records one failure and reports whether the key is now locked.
	Fail(ctx context.Context, key string) (bool, error)
	// Reset clears the key (successful login).
	Reset(ctx context.Context, key string) error
}

// Memory is a bounded in-process Limiter (least recently used keys are evicted).
type Memory struct {
	p     Policy
	max   int
	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
	now   func() time.Time
}

type entry struct {
	key         string
	fails       int
	windowStart time.Time
	lockedUntil time.Time
}

// NewMemory builds an in-memory limiter keeping at most maxKeys keys.
func NewMemory(p Policy, maxKeys int) *Memory {
	if maxKeys <= 0 {
		maxKeys = 100_000
	}
	return &Memory{p: p, max: maxKeys, ll: list.New(), items: map[string]*list.Element{}, now: time.Now}
}

// SetClock replaces the time source (tests).
func (m *Memory) SetClock(now func() time.Time) { m.now = now }

// Len is the number of tracked keys.
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ll.Len()
}

func (m *Memory) get(key string, create bool) *entry {
	if el, ok := m.items[key]; ok {
		m.ll.MoveToFront(el)
		return el.Value.(*entry)
	}
	if !create {
		return nil
	}
	e := &entry{key: key}
	m.items[key] = m.ll.PushFront(e)
	for m.ll.Len() > m.max {
		old := m.ll.Back()
		m.ll.Remove(old)
		delete(m.items, old.Value.(*entry).key)
	}
	return e
}

func (m *Memory) Locked(_ context.Context, key string) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.get(key, false)
	if e == nil {
		return false, 0, nil
	}
	if d := e.lockedUntil.Sub(m.now()); d > 0 {
		return true, d, nil
	}
	return false, 0, nil
}

func (m *Memory) Fail(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	e := m.get(key, true)
	if now.Sub(e.windowStart) > m.p.Window {
		e.fails, e.windowStart = 0, now
	}
	e.fails++
	if e.fails >= m.p.MaxFailures {
		e.lockedUntil = now.Add(m.p.LockFor)
		e.fails, e.windowStart = 0, now
		return true, nil
	}
	return false, nil
}

func (m *Memory) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if el, ok := m.items[key]; ok {
		m.ll.Remove(el)
		delete(m.items, key)
	}
	return nil
}
