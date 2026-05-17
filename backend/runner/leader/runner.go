package leader

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/store"
)

const (
	leaderElectionInterval = 5 * time.Second
)

// Runner manages cluster leader election and performs leader duties.
type Runner struct {
	store    *store.Store
	profile  *config.Profile
	isLeader bool
	lock     *store.AdvisoryLock
	mu       sync.RWMutex
}

// NewRunner creates a new leader runner.
func NewRunner(store *store.Store, profile *config.Profile) *Runner {
	return &Runner{
		store:   store,
		profile: profile,
	}
}

// IsLeader returns true if the current replica is the cluster leader.
func (r *Runner) IsLeader() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isLeader
}

// Run starts the leader election process and performs duties if elected.
func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer r.releaseLeadership()

	ticker := time.NewTicker(leaderElectionInterval)
	defer ticker.Stop()

	// Try immediately
	r.tryAcquireLeadership(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tryAcquireLeadership(ctx)
			if r.IsLeader() {
				r.performLeaderDuties(ctx)
			}
		}
	}
}

func (r *Runner) tryAcquireLeadership(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If we are already the leader, ensure our lock is still valid (connection alive)
	if r.isLeader && r.lock != nil {
		return
	}

	lock, acquired, err := store.TryAdvisoryLock(ctx, r.store.GetDB(), store.AdvisoryLockKeyLeader)
	if err != nil {
		slog.Error("Failed to attempt advisory lock for leadership", log.BBError(err))
		return
	}

	if acquired {
		slog.Info("Acquired cluster leadership", slog.String("replicaID", r.profile.ReplicaID))
		r.isLeader = true
		r.lock = lock
	}
}

func (r *Runner) releaseLeadership() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isLeader && r.lock != nil {
		slog.Info("Releasing cluster leadership", slog.String("replicaID", r.profile.ReplicaID))
		if err := r.lock.Release(); err != nil {
			slog.Error("Failed to release advisory lock", log.BBError(err))
		}
		r.lock = nil
		r.isLeader = false
	}
}

func (r *Runner) performLeaderDuties(ctx context.Context) {
	// 1. Cleanup stale replicas
	if _, err := r.store.MarkStaleReplicas(ctx, 30*time.Second); err != nil {
		slog.Error("Leader failed to mark stale replicas", log.BBError(err))
	}
	
	// 2. Recover orphaned tasks
	r.performTaskRecovery(ctx)
}
