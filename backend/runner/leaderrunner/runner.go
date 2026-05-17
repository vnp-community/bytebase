// Package leaderrunner wraps a runner so it only executes when this replica
// is the elected leader for the given advisory lock.
package leaderrunner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bytebase/bytebase/backend/component/leader"
)

// Runner is the interface that background runners must implement.
type Runner interface {
	Run(ctx context.Context, wg *sync.WaitGroup)
}

// LeaderRunner wraps an inner Runner and only starts it when the associated
// LeaderElector confirms this replica is the leader.
type LeaderRunner struct {
	inner   Runner
	elector *leader.LeaderElector
	name    string
}

// NewLeaderRunner creates a new leader-gated runner.
func NewLeaderRunner(inner Runner, elector *leader.LeaderElector, name string) *LeaderRunner {
	return &LeaderRunner{
		inner:   inner,
		elector: elector,
		name:    name,
	}
}

// Run starts the elector in a goroutine and polls IsLeader() until
// elected. Once elected, starts the inner runner. If leadership is lost
// (elector marks non-leader), the inner runner is stopped via context cancel
// and restarted when re-elected.
func (lr *LeaderRunner) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Start elector in background
	electorCtx, electorCancel := context.WithCancel(ctx)
	defer electorCancel()

	go lr.elector.Run(electorCtx)

	slog.Info("LeaderRunner waiting for election", "runner", lr.name)

	var innerCancel context.CancelFunc
	var innerWG sync.WaitGroup
	running := false

	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			if innerCancel != nil {
				innerCancel()
				innerWG.Wait()
			}
			return

		case <-pollTicker.C:
			isLeader := lr.elector.IsLeader()

			if isLeader && !running {
				// Became leader — start inner runner
				slog.Info("LeaderRunner: elected, starting runner", "runner", lr.name)
				innerCtx, cancel := context.WithCancel(ctx)
				innerCancel = cancel
				innerWG.Add(1)
				go lr.inner.Run(innerCtx, &innerWG)
				running = true
			} else if !isLeader && running {
				// Lost leadership — stop inner runner
				slog.Info("LeaderRunner: lost leadership, stopping runner", "runner", lr.name)
				if innerCancel != nil {
					innerCancel()
					innerWG.Wait()
				}
				running = false
			}
		}
	}
}
