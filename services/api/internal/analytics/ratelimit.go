package analytics

import (
	"sync"
	"time"
)

// maxEventsPerWindow caps ingestion per project+IP. MVP recommends
// ~100 events/minute/project/IP to prevent spam, accidental loops and
// database exhaustion. The tracking ID is public, so this is abuse
// prevention, not authentication.
const (
	maxEventsPerWindow  = 100
	rateLimitWindow     = time.Minute
	rateLimitSweepEvery = 1000
)

type rateBucket struct {
	count       int
	windowStart time.Time
}

// Limiter is a fixed-window rate limiter keyed by project+IP.
// It is in-memory and per-instance: sufficient for a single-replica MVP.
// A multi-replica deployment would need a shared store (e.g. Redis).
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	limit   int
	window  time.Duration
	now     func() time.Time
	ops     int
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{
		buckets: map[string]*rateBucket{},
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

// Allow reports whether an event from this project+IP fits in the
// current window. Expired windows reset lazily; stale keys are swept
// opportunistically to bound memory.
func (limiter *Limiter) Allow(projectID, ip string) bool {
	now := limiter.now().UTC()
	key := projectID + "|" + ip

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	bucket, ok := limiter.buckets[key]
	if !ok || now.Sub(bucket.windowStart) >= limiter.window {
		limiter.buckets[key] = &rateBucket{count: 1, windowStart: now}
	} else {
		bucket.count++
	}
	limiter.ops++
	if limiter.ops%rateLimitSweepEvery == 0 {
		for k, b := range limiter.buckets {
			if now.Sub(b.windowStart) >= limiter.window {
				delete(limiter.buckets, k)
			}
		}
	}
	return limiter.buckets[key].count <= limiter.limit
}
