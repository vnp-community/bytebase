package leader

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestLeaderElector_FormatLockID verifies lock ID formatting.
func TestLeaderElector_FormatLockID(t *testing.T) {
	got := formatLockID(100001)
	if got == "" {
		t.Error("expected non-empty lock ID string")
	}
}

// TestLeaderElector_InitialState verifies elector starts as non-leader.
func TestLeaderElector_InitialState(t *testing.T) {
	le := NewLeaderElector(&sql.DB{}, LockIDTaskScheduler, 10*time.Second, "test")
	if le.IsLeader() {
		t.Error("expected initial state to be non-leader")
	}
}

// TestLeaderElector_Constants verifies lock ID uniqueness.
func TestLeaderElector_Constants(t *testing.T) {
	ids := map[int64]string{
		LockIDTaskScheduler: "TaskScheduler",
		LockIDPlanCheck:     "PlanCheck",
		LockIDSchemaSync:    "SchemaSync",
		LockIDApproval:      "Approval",
		LockIDDataCleaner:   "DataCleaner",
	}

	seen := make(map[int64]bool)
	for id, name := range ids {
		if seen[id] {
			t.Errorf("duplicate lock ID %d for %s", id, name)
		}
		seen[id] = true
	}

	if len(ids) != 5 {
		t.Errorf("expected 5 lock IDs, got %d", len(ids))
	}
}

// TestLeaderElector_Release verifies release sets non-leader state.
func TestLeaderElector_Release(t *testing.T) {
	le := NewLeaderElector(&sql.DB{}, LockIDTaskScheduler, 10*time.Second, "test")
	le.isLeader.Store(true)
	le.release()
	if le.IsLeader() {
		t.Error("expected non-leader after release")
	}
}

// TestLeaderElector_RunCancellation verifies Run exits on context cancel.
func TestLeaderElector_RunCancellation(t *testing.T) {
	// Use a minimal renewTick so the test is fast
	le := NewLeaderElector(&sql.DB{}, LockIDTaskScheduler, 50*time.Millisecond, "test")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		le.Run(ctx)
		close(done)
	}()

	// Cancel quickly
	cancel()

	select {
	case <-done:
		// OK — Run exited
	case <-time.After(2 * time.Second):
		t.Error("Run did not exit after context cancellation")
	}
}
