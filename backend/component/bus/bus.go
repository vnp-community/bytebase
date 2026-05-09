// Package bus contains the message bus for synchronization within the server.
package bus

import (
	"context"
	"sync"
)

// Compile-time interface satisfaction check.
var _ EventBus = (*Bus)(nil)

// PlanRef identifies a plan by project and UID.
type PlanRef struct {
	ProjectID string
	PlanID    int64
}

// TaskRunRef identifies a task run by project and ID.
type TaskRunRef struct {
	ProjectID string
	ID        int64
}

// IssueRef identifies an issue by project and UID.
type IssueRef struct {
	ProjectID string
	UID       int64
}

// PlanCheckRunRef identifies a plan check run by project and UID.
type PlanCheckRunRef struct {
	ProjectID string
	UID       int64
}

// Bus is the message bus for all in-memory communication within the server.
// It implements the EventBus interface.
type Bus struct {
	// approvalCheckCh signals when an issue needs approval template finding.
	approvalCheckCh chan IssueRef

	// runningTaskRunsCancelFunc stores cancel functions for running task runs.
	runningTaskRunsCancelFunc sync.Map // map[TaskRunRef]context.CancelFunc

	// runningPlanCheckRunsCancelFunc stores cancel functions for running plan checks.
	runningPlanCheckRunsCancelFunc sync.Map // map[PlanCheckRunRef]context.CancelFunc

	// planCheckTickleCh is the tickler for plan check scheduler.
	planCheckTickleCh chan int
	// taskRunTickleCh is the tickler for task run scheduler.
	taskRunTickleCh chan int

	// rolloutCreationCh is the channel for automatic rollout creation.
	rolloutCreationCh chan PlanRef

	// planCompletionCheckCh signals when a plan might be complete.
	planCompletionCheckCh chan PlanRef
}

// New creates a new Bus instance with buffered channels.
func New() (*Bus, error) {
	return &Bus{
		approvalCheckCh:       make(chan IssueRef, 1000),
		planCheckTickleCh:     make(chan int, 1000),
		taskRunTickleCh:       make(chan int, 1000),
		rolloutCreationCh:     make(chan PlanRef, 100),
		planCompletionCheckCh: make(chan PlanRef, 1000),
	}, nil
}

// TicklePlanCheck signals the plan check scheduler to wake up.
func (b *Bus) TicklePlanCheck() {
	select {
	case b.planCheckTickleCh <- 0:
	default:
	}
}

// TickleTaskRun signals the task run scheduler to wake up.
func (b *Bus) TickleTaskRun() {
	select {
	case b.taskRunTickleCh <- 0:
	default:
	}
}

// RequestApprovalCheck requests approval template finding for an issue.
func (b *Bus) RequestApprovalCheck(ref IssueRef) {
	select {
	case b.approvalCheckCh <- ref:
	default:
	}
}

// RequestRolloutCreation requests automatic rollout creation for a plan.
func (b *Bus) RequestRolloutCreation(ref PlanRef) {
	select {
	case b.rolloutCreationCh <- ref:
	default:
	}
}

// RequestPlanCompletionCheck requests checking if a plan is complete.
func (b *Bus) RequestPlanCompletionCheck(ref PlanRef) {
	select {
	case b.planCompletionCheckCh <- ref:
	default:
	}
}

// RegisterTaskRunCancel registers a cancel function for a running task run.
func (b *Bus) RegisterTaskRunCancel(ref TaskRunRef, cancel context.CancelFunc) {
	b.runningTaskRunsCancelFunc.Store(ref, cancel)
}

// CancelTaskRun cancels a running task run. Returns true if successfully cancelled.
func (b *Bus) CancelTaskRun(ref TaskRunRef) bool {
	v, ok := b.runningTaskRunsCancelFunc.LoadAndDelete(ref)
	if !ok {
		return false
	}
	if cancel, ok := v.(context.CancelFunc); ok {
		cancel()
		return true
	}
	return false
}

// DeregisterTaskRunCancel removes the cancel function for a task run.
func (b *Bus) DeregisterTaskRunCancel(ref TaskRunRef) {
	b.runningTaskRunsCancelFunc.Delete(ref)
}

// RegisterPlanCheckCancel registers a cancel function for a running plan check.
func (b *Bus) RegisterPlanCheckCancel(ref PlanCheckRunRef, cancel context.CancelFunc) {
	b.runningPlanCheckRunsCancelFunc.Store(ref, cancel)
}

// CancelPlanCheck cancels a running plan check. Returns true if successfully cancelled.
func (b *Bus) CancelPlanCheck(ref PlanCheckRunRef) bool {
	v, ok := b.runningPlanCheckRunsCancelFunc.LoadAndDelete(ref)
	if !ok {
		return false
	}
	if cancel, ok := v.(context.CancelFunc); ok {
		cancel()
		return true
	}
	return false
}

// DeregisterPlanCheckCancel removes the cancel function for a plan check.
func (b *Bus) DeregisterPlanCheckCancel(ref PlanCheckRunRef) {
	b.runningPlanCheckRunsCancelFunc.Delete(ref)
}

// PlanCheckChan returns the channel for plan check tickles.
func (b *Bus) PlanCheckChan() <-chan int {
	return b.planCheckTickleCh
}

// TaskRunChan returns the channel for task run tickles.
func (b *Bus) TaskRunChan() <-chan int {
	return b.taskRunTickleCh
}

// ApprovalChan returns the channel for approval check events.
func (b *Bus) ApprovalChan() <-chan IssueRef {
	return b.approvalCheckCh
}

// RolloutCreationChan returns the channel for rollout creation events.
func (b *Bus) RolloutCreationChan() <-chan PlanRef {
	return b.rolloutCreationCh
}

// PlanCompletionChan returns the channel for plan completion check events.
func (b *Bus) PlanCompletionChan() <-chan PlanRef {
	return b.planCompletionCheckCh
}
