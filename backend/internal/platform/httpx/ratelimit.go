package httpx

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// MaxBuckets bounds the limiter's memory (SECURITY M1): when full, idle buckets are
// dropped first, then arbitrary ones (a dropped key just starts with a full bucket).
const MaxBuckets = 100_000

// RateLimiter is an in-memory token bucket per client key (works without Redis).
type RateLimiter struct {
	max     int
	rate    float64 // tokens per second
	burst   float64
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{rate: rps, burst: float64(burst), buckets: map[string]*bucket{}, now: time.Now, max: MaxBuckets}
}

// Allow consumes a token for key.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	b, ok := rl.buckets[key]
	if !ok {
		if len(rl.buckets) >= rl.max {
			rl.evict(now)
		}
		b = &bucket{tokens: rl.burst, last: now}
		rl.buckets[key] = b
	}
	b.tokens = min(rl.burst, b.tokens+now.Sub(b.last).Seconds()*rl.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evict frees room: buckets that are full again first, then any 10% (caller holds mu).
func (rl *RateLimiter) evict(now time.Time) {
	for k, b := range rl.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*rl.rate >= rl.burst {
			delete(rl.buckets, k)
		}
	}
	for k := range rl.buckets {
		if len(rl.buckets) < rl.max*9/10 {
			break
		}
		delete(rl.buckets, k)
	}
}

// Len is the number of tracked keys.
func (rl *RateLimiter) Len() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.buckets)
}

// Cleanup drops buckets idle longer than maxIdle.
func (rl *RateLimiter) Cleanup(maxIdle time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cut := rl.now().Add(-maxIdle)
	for k, b := range rl.buckets {
		if b.last.Before(cut) {
			delete(rl.buckets, k)
		}
	}
}

// Middleware rejects with 429 when the client IP exceeds the rate.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			key = host
		}
		if !rl.Allow(key) {
			w.Header().Set("Retry-After", strconv.Itoa(1))
			http.Error(w, `{"errors":[{"message":"rate limit exceeded","extensions":{"code":"RATE_LIMITED"}}]}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
