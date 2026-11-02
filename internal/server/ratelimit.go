package server

import (
	"sync"
	"time"
)

// tokenBucket is a tiny, dependency-free rate limiter. The organism is a
// localhost app, so this is not an internet quota system: it protects the
// kernel from an accidental request loop, a runaway browser tab, or a plugin
// hammering its own API.
type tokenBucket struct {
	tokens float64
	last   time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens per second
	burst   float64
	now     func() time.Time
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	return &rateLimiter{
		buckets: map[string]*tokenBucket{},
		rate:    rate,
		burst:   burst,
		now:     time.Now,
	}
}

// allow consumes one token, or returns how long the caller should wait.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.buckets[key]
	if b == nil {
		b = &tokenBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
		if len(l.buckets) > 1024 {
			for k, old := range l.buckets {
				if now.Sub(old.last) > 10*time.Minute {
					delete(l.buckets, k)
				}
			}
		}
	} else if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1-b.tokens)/l.rate*float64(time.Second)) + time.Millisecond
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}
