package selfheal

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/health"
	"github.com/bytebase/bytebase/backend/store"
)

// Runner is the self-healing runner.
type Runner struct {
	store         *store.Store
	profile       *config.Profile
	healthChecker *health.Checker
}

// NewRunner creates a new self-healing runner.
func NewRunner(store *store.Store, profile *config.Profile, healthChecker *health.Checker) *Runner {
	return &Runner{
		store:         store,
		profile:       profile,
		healthChecker: healthChecker,
	}
}

// Run starts the runner.
func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Only run in HA mode
	if !r.profile.HA {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkAndHeal(ctx)
		}
	}
}

func (r *Runner) checkAndHeal(ctx context.Context) {
	overall, checks := r.healthChecker.RunAll(ctx)
	if overall != health.StatusUnhealthy && overall != health.StatusDegraded {
		return
	}

	for _, check := range checks {
		if check.Name == "PostgreSQL" && (check.Status == health.StatusDegraded || check.Status == health.StatusUnhealthy) {
			zap.L().Warn("selfheal: PG pool degraded/unhealthy, purging cache")
			r.store.DeleteCache()
		}

		if check.Name == "Memory" && (check.Status == health.StatusDegraded || check.Status == health.StatusUnhealthy) {
			zap.L().Warn("selfheal: memory pressure detected, purging cache and forcing GC")
			r.store.DeleteCache()
			runtime.GC()
			debug.FreeOSMemory()
		}
	}
}
