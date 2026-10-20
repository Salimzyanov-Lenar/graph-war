package api

import (
	"sync"
	"time"
)

// RateLimiter is a fixed-window limiter per key (client IP), sized for
// one process: B08 requires a cap on guest creation and a bounded map.
type RateLimiter struct {
	mu      sync.Mutex
	hits    map[string]*window
	limit   int
	per     time.Duration
	maxKeys int
}

type window struct {
	count int
	start time.Time
}

// NewRateLimiter allows `limit` hits per `per` per key. maxKeys bounds
// the memory: the oldest windows are dropped when exceeded.
func NewRateLimiter(limit int, per time.Duration, maxKeys int) *RateLimiter {
	return &RateLimiter{
		hits:    make(map[string]*window),
		limit:   limit,
		per:     per,
		maxKeys: maxKeys,
	}
}

// Allow reports whether the key may proceed now.
func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.hits) >= l.maxKeys {
		l.evict(now)
	}
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= l.per {
		if !ok && len(l.hits) >= l.maxKeys {
			// Still full after evicting expired windows: refuse new keys.
			return false
		}
		l.hits[key] = &window{count: 1, start: now}
		return true
	}
	w.count++
	return w.count <= l.limit
}

// evict drops expired windows; if none expired (flood of fresh keys),
// it drops everything beyond half the budget.
func (l *RateLimiter) evict(now time.Time) {
	for k, w := range l.hits {
		if now.Sub(w.start) >= l.per {
			delete(l.hits, k)
		}
	}
	for k := range l.hits {
		if len(l.hits) < l.maxKeys/2 {
			return
		}
		delete(l.hits, k)
	}
}
