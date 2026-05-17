package bus

import (
	"context"
	"testing"
	"time"
)

func TestNATSBus_ImplementsEventBus(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	// Compile-time check is already in nats_bus.go, but verify at runtime.
	var _ EventBus = nb
}

func TestNATSBus_TicklePlanCheck(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	nb.TicklePlanCheck()

	select {
	case <-nb.PlanCheckChan():
		// OK — received tickle.
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for plan check tickle")
	}
}

func TestNATSBus_TickleTaskRun(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	nb.TickleTaskRun()

	select {
	case <-nb.TaskRunChan():
		// OK.
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task run tickle")
	}
}

func TestNATSBus_RequestApprovalCheck(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	ref := IssueRef{ProjectID: "proj-1", UID: 42}
	nb.RequestApprovalCheck(ref)

	select {
	case got := <-nb.ApprovalChan():
		if got.ProjectID != ref.ProjectID || got.UID != ref.UID {
			t.Fatalf("got %+v, want %+v", got, ref)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for approval check")
	}
}

func TestNATSBus_RequestRolloutCreation(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	ref := PlanRef{ProjectID: "proj-1", PlanID: 99}
	nb.RequestRolloutCreation(ref)

	select {
	case got := <-nb.RolloutCreationChan():
		if got.ProjectID != ref.ProjectID || got.PlanID != ref.PlanID {
			t.Fatalf("got %+v, want %+v", got, ref)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for rollout creation")
	}
}

func TestNATSBus_RequestPlanCompletionCheck(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	ref := PlanRef{ProjectID: "proj-2", PlanID: 7}
	nb.RequestPlanCompletionCheck(ref)

	select {
	case got := <-nb.PlanCompletionChan():
		if got.ProjectID != ref.ProjectID || got.PlanID != ref.PlanID {
			t.Fatalf("got %+v, want %+v", got, ref)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for plan completion check")
	}
}

func TestNATSBus_CancelTaskRun(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	ref := TaskRunRef{ProjectID: "proj-1", ID: 10}
	ctx, cancel := context.WithCancel(context.Background())
	nb.RegisterTaskRunCancel(ref, cancel)

	if !nb.CancelTaskRun(ref) {
		t.Fatal("CancelTaskRun should return true")
	}
	if ctx.Err() == nil {
		t.Fatal("context should be cancelled")
	}

	// Should return false on second cancel.
	if nb.CancelTaskRun(ref) {
		t.Fatal("CancelTaskRun should return false after deregister")
	}
}

func TestNATSBus_CancelPlanCheck(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	ref := PlanCheckRunRef{ProjectID: "proj-1", UID: 20}
	ctx, cancel := context.WithCancel(context.Background())
	nb.RegisterPlanCheckCancel(ref, cancel)

	if !nb.CancelPlanCheck(ref) {
		t.Fatal("CancelPlanCheck should return true")
	}
	if ctx.Err() == nil {
		t.Fatal("context should be cancelled")
	}
}

func TestNATSBus_DeregisterTaskRunCancel(t *testing.T) {
	nb, err := NewNATSBus()
	if err != nil {
		t.Fatalf("NewNATSBus() error: %v", err)
	}
	defer nb.Shutdown()

	ref := TaskRunRef{ProjectID: "proj-1", ID: 30}
	_, cancel := context.WithCancel(context.Background())
	nb.RegisterTaskRunCancel(ref, cancel)
	nb.DeregisterTaskRunCancel(ref)

	if nb.CancelTaskRun(ref) {
		t.Fatal("CancelTaskRun should return false after deregister")
	}
}
