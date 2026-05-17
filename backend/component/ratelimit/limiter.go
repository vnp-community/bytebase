// Package ratelimit provides per-workspace API rate limiting using token bucket.
package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// Config defines rate limits for read and write operations.
type Config struct {
	ReadRatePerSecond  float64
	WriteRatePerSecond float64
	ReadBurst          int
	WriteBurst         int
}

// DefaultConfig provides sensible defaults for multi-tenant deployments.
var DefaultConfig = Config{
	ReadRatePerSecond:  1000,
	WriteRatePerSecond: 100,
	ReadBurst:          2000,
	WriteBurst:         200,
}

// WorkspaceLimiter enforces per-workspace rate limits using token bucket
// algorithm. Limiters are lazily created per workspace+operation type.
type WorkspaceLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	config   Config
}

// New creates a WorkspaceLimiter with the given configuration.
func New(config Config) *WorkspaceLimiter {
	return &WorkspaceLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}
}

// Allow checks if a request from the given workspace is within rate limits.
// isWrite differentiates between read and write operations which have
// different rate limits.
func (wl *WorkspaceLimiter) Allow(workspace string, isWrite bool) bool {
	return wl.getOrCreateLimiter(workspace, isWrite).Allow()
}

func (wl *WorkspaceLimiter) getOrCreateLimiter(workspace string, isWrite bool) *rate.Limiter {
	key := workspace + ":read"
	r := rate.Limit(wl.config.ReadRatePerSecond)
	b := wl.config.ReadBurst
	if isWrite {
		key = workspace + ":write"
		r = rate.Limit(wl.config.WriteRatePerSecond)
		b = wl.config.WriteBurst
	}

	wl.mu.RLock()
	if l, ok := wl.limiters[key]; ok {
		wl.mu.RUnlock()
		return l
	}
	wl.mu.RUnlock()

	wl.mu.Lock()
	defer wl.mu.Unlock()
	// Double-check after acquiring write lock.
	if l, ok := wl.limiters[key]; ok {
		return l
	}
	l := rate.NewLimiter(r, b)
	wl.limiters[key] = l
	return l
}
