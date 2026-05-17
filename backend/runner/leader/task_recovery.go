package leader

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/store"
)

// performTaskRecovery finds orphaned RUNNING tasks on dead nodes and resets them to PENDING.
func (r *Runner) performTaskRecovery(ctx context.Context) {
	// 1. Get active nodes list
	activeNodes, err := r.store.ListActiveReplicas(ctx, 30*time.Second)
	if err != nil {
		slog.Error("Leader failed to list active replicas for task recovery", log.BBError(err))
		return
	}

	var activeIDs []string
	for _, node := range activeNodes {
		activeIDs = append(activeIDs, node.ReplicaID)
	}

	// 2. Find orphaned task runs
	orphans, err := r.store.FindOrphanedTaskRuns(ctx, activeIDs)
	if err != nil {
		slog.Error("Leader failed to find orphaned task runs", log.BBError(err))
		return
	}

	if len(orphans) == 0 {
		return
	}

	slog.Info("Leader found orphaned task runs", slog.Int("count", len(orphans)))

	// 3. Reset orphans to PENDING
	for _, orphan := range orphans {
		patch := &store.TaskRunStatusPatch{
			ID:        orphan.ID,
			ProjectID: orphan.ProjectID,
			Status:    storepb.TaskRun_PENDING,
			ResultProto: &storepb.TaskRunResult{
				Detail: "Task run recovered by leader: previous node died.",
			},
		}

		if _, err := r.store.UpdateTaskRunStatus(ctx, patch); err != nil {
			slog.Error("Leader failed to recover orphaned task run",
				slog.Int64("taskRunID", orphan.ID),
				log.BBError(err),
			)
			continue
		}

		// Clear assigned node so it can be picked up by any healthy node
		if err := r.store.UpdateTaskRunAssignedNode(ctx, orphan.ID, ""); err != nil {
			slog.Error("Leader failed to clear assigned node for orphaned task run",
				slog.Int64("taskRunID", orphan.ID),
				log.BBError(err),
			)
		}
	}
}
