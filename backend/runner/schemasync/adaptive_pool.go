package schemasync

import (
	"os"
	"runtime"
	"strconv"

	"github.com/sourcegraph/conc/pool"
)

const (
	defaultMinConcurrency = 10
	defaultMaxConcurrency = 300
)

// newAdaptivePool returns a goroutine pool whose concurrency scales with the
// available CPU cores. The limit can be overridden via SYNC_MAX_CONCURRENCY.
func (s *Syncer) newAdaptivePool() *pool.Pool {
	return pool.New().WithMaxGoroutines(calculateAdaptiveConcurrency())
}

// calculateAdaptiveConcurrency computes a concurrency limit as 10×NumCPU,
// clamped to [defaultMinConcurrency, defaultMaxConcurrency]. An explicit
// SYNC_MAX_CONCURRENCY env var takes priority.
func calculateAdaptiveConcurrency() int {
	if v := os.Getenv("SYNC_MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	target := runtime.NumCPU() * 10
	if target < defaultMinConcurrency {
		return defaultMinConcurrency
	}
	if target > defaultMaxConcurrency {
		return defaultMaxConcurrency
	}
	return target
}
