package heartbeat

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/store/model"
)

const (
	heartbeatInterval = 10 * time.Second
)

// Runner sends periodic heartbeats to indicate this replica is alive.
type Runner struct {
	store   *store.Store
	profile *config.Profile
	node    *model.ReplicaNode
}

// NewRunner creates a new heartbeat runner.
func NewRunner(store *store.Store, profile *config.Profile) *Runner {
	return &Runner{
		store:   store,
		profile: profile,
		node: &model.ReplicaNode{
			ReplicaID:    profile.ReplicaID,
			EndpointURL:  profile.ExternalURL,
			Version:      profile.Version,
			Status:       "STARTING",
			Capabilities: []string{"API", "RUNNER"}, // Default capabilities
			Metadata:     "{}",
		},
	}
}

// Run starts the heartbeat runner.
func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Mark stale replicas on startup
	if _, err := r.store.MarkStaleReplicas(ctx, 30*time.Second); err != nil {
		slog.Error("Failed to mark stale replicas on startup", log.BBError(err))
	}

	r.SetStatus("READY")

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	slog.Debug("Heartbeat runner started", slog.String("replicaID", r.profile.ReplicaID))

	// Send heartbeat immediately on startup
	r.SendHeartbeat(ctx)

	for {
		select {
		case <-ticker.C:
			r.SendHeartbeat(ctx)
		case <-ctx.Done():
			r.SetStatus("STOPPED")
			r.SendHeartbeat(context.Background())
			return
		}
	}
}

// SetStatus updates the status of the current replica node.
func (r *Runner) SetStatus(status string) {
	r.node.Status = status
}

// SendHeartbeat upserts the replica node heartbeat into the database.
func (r *Runner) SendHeartbeat(ctx context.Context) {
	if err := r.store.UpsertReplicaHeartbeat(ctx, r.node); err != nil {
		slog.Error("Failed to send heartbeat", log.BBError(err))
	}
}
