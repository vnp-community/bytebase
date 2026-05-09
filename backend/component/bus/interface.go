package bus

import (
	"context"
)

// EventBus defines the interface for all in-memory event communication within the server.
// This abstraction enables:
//   - Unit testing with mock implementations
//   - Future migration to durable messaging (PG LISTEN/NOTIFY, NATS)
//   - Clear documentation of the DCM workflow event flow
//
// Channel direction: producers call Send*() methods; consumers read from *Chan() methods.
type EventBus interface {
	// TicklePlanCheck signals the plan check scheduler to wake up and process pending checks.
	TicklePlanCheck()
	// TickleTaskRun signals the task run scheduler to wake up and process pending runs.
	TickleTaskRun()

	// RequestApprovalCheck requests approval template finding for an issue.
	RequestApprovalCheck(ref IssueRef)
	// RequestRolloutCreation requests automatic rollout creation for a plan.
	RequestRolloutCreation(ref PlanRef)
	// RequestPlanCompletionCheck requests checking if a plan is complete.
	RequestPlanCompletionCheck(ref PlanRef)

	// RegisterTaskRunCancel registers a cancel function for a running task run.
	RegisterTaskRunCancel(ref TaskRunRef, cancel context.CancelFunc)
	// CancelTaskRun cancels a running task run. Returns true if successfully cancelled.
	CancelTaskRun(ref TaskRunRef) bool
	// DeregisterTaskRunCancel removes the cancel function for a task run.
	DeregisterTaskRunCancel(ref TaskRunRef)

	// RegisterPlanCheckCancel registers a cancel function for a running plan check.
	RegisterPlanCheckCancel(ref PlanCheckRunRef, cancel context.CancelFunc)
	// CancelPlanCheck cancels a running plan check. Returns true if successfully cancelled.
	CancelPlanCheck(ref PlanCheckRunRef) bool
	// DeregisterPlanCheckCancel removes the cancel function for a plan check.
	DeregisterPlanCheckCancel(ref PlanCheckRunRef)

	// PlanCheckChan returns the channel for plan check tickles.
	PlanCheckChan() <-chan int
	// TaskRunChan returns the channel for task run tickles.
	TaskRunChan() <-chan int
	// ApprovalChan returns the channel for approval check events.
	ApprovalChan() <-chan IssueRef
	// RolloutCreationChan returns the channel for rollout creation events.
	RolloutCreationChan() <-chan PlanRef
	// PlanCompletionChan returns the channel for plan completion check events.
	PlanCompletionChan() <-chan PlanRef
}
