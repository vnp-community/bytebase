package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements per-key token bucket rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens per second
	burst   int     // max tokens (bucket capacity)
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

// NewRateLimiter creates a rate limiter.
// rate is tokens per second, burst is the maximum burst size (bucket capacity).
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}
	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request is allowed for the given key.
// Returns true if the request is within the rate limit, false otherwise.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = &tokenBucket{
			tokens:   float64(rl.burst),
			lastFill: time.Now(),
		}
		rl.buckets[key] = bucket
	}

	// Refill tokens based on elapsed time
	elapsed := time.Since(bucket.lastFill).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastFill = time.Now()

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}

// Wait blocks until a request is allowed for the given key, or the context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context, key string) error {
	for {
		if rl.Allow(key) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("rate limiter: %w", ctx.Err())
		case <-time.After(10 * time.Millisecond):
			// Re-check after a short delay
		}
	}
}

// cleanupLoop removes stale buckets every 5 minutes.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for key, bucket := range rl.buckets {
			if time.Since(bucket.lastFill) > 5*time.Minute {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}
