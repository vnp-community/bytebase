// Package bus contains the message bus for synchronization within the server.
// NATSBus is a NATS-backed implementation of the EventBus interface.
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// Compile-time interface satisfaction check.
var _ EventBus = (*NATSBus)(nil)

// NATS subject constants for all event types.
const (
	SubjectPlanCheck       = "bytebase.plancheck.tickle"
	SubjectTaskRun         = "bytebase.taskrun.tickle"
	SubjectApprovalCheck   = "bytebase.approval.check"
	SubjectRolloutCreation = "bytebase.rollout.create"
	SubjectPlanCompletion  = "bytebase.plan.completion"
)

// NATSBus implements EventBus using an embedded NATS server.
// NATS subscribers bridge messages into Go channels consumed by existing runners.
type NATSBus struct {
	ns *natsserver.Server
	nc *nats.Conn

	// Channels consumed by runners — identical to Bus channels.
	planCheckCh       chan int
	taskRunCh         chan int
	approvalCh        chan IssueRef
	rolloutCh         chan PlanRef
	planCompletionCh  chan PlanRef

	// Cancel function registries (in-memory, same as Bus).
	runningTaskRunsCancelFunc      sync.Map // map[TaskRunRef]context.CancelFunc
	runningPlanCheckRunsCancelFunc sync.Map // map[PlanCheckRunRef]context.CancelFunc

	// Subscriptions for cleanup.
	subs []*nats.Subscription
}

// NewNATSBus creates a new NATSBus with an embedded NATS server.
func NewNATSBus() (*NATSBus, error) {
	opts := &natsserver.Options{
		Host:           "127.0.0.1",
		Port:           -1, // Random available port.
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS server: %w", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, fmt.Errorf("NATS server failed to become ready within 5s")
	}

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		return nil, fmt.Errorf("failed to connect to embedded NATS: %w", err)
	}

	b := &NATSBus{
		ns:               ns,
		nc:               nc,
		planCheckCh:      make(chan int, 1000),
		taskRunCh:        make(chan int, 1000),
		approvalCh:       make(chan IssueRef, 1000),
		rolloutCh:        make(chan PlanRef, 100),
		planCompletionCh: make(chan PlanRef, 1000),
	}

	// Subscribe NATS → Go channels (bridge for existing runners).
	if err := b.setupSubscriptions(); err != nil {
		nc.Close()
		ns.Shutdown()
		return nil, fmt.Errorf("failed to setup NATS subscriptions: %w", err)
	}

	slog.Info("NATSBus initialized", "url", ns.ClientURL())
	return b, nil
}

// setupSubscriptions creates NATS subscriptions that bridge messages to Go channels.
func (b *NATSBus) setupSubscriptions() error {
	sub, err := b.nc.Subscribe(SubjectPlanCheck, func(_ *nats.Msg) {
		select {
		case b.planCheckCh <- 0:
		default:
		}
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)

	sub, err = b.nc.Subscribe(SubjectTaskRun, func(_ *nats.Msg) {
		select {
		case b.taskRunCh <- 0:
		default:
		}
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)

	sub, err = b.nc.Subscribe(SubjectApprovalCheck, func(msg *nats.Msg) {
		var ref IssueRef
		if err := json.Unmarshal(msg.Data, &ref); err != nil {
			slog.Warn("NATSBus: failed to unmarshal IssueRef", "err", err)
			return
		}
		select {
		case b.approvalCh <- ref:
		default:
		}
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)

	sub, err = b.nc.Subscribe(SubjectRolloutCreation, func(msg *nats.Msg) {
		var ref PlanRef
		if err := json.Unmarshal(msg.Data, &ref); err != nil {
			slog.Warn("NATSBus: failed to unmarshal PlanRef", "err", err)
			return
		}
		select {
		case b.rolloutCh <- ref:
		default:
		}
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)

	sub, err = b.nc.Subscribe(SubjectPlanCompletion, func(msg *nats.Msg) {
		var ref PlanRef
		if err := json.Unmarshal(msg.Data, &ref); err != nil {
			slog.Warn("NATSBus: failed to unmarshal PlanRef", "err", err)
			return
		}
		select {
		case b.planCompletionCh <- ref:
		default:
		}
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)

	return nil
}

// --- Producer methods (publish to NATS) ---

// TicklePlanCheck signals the plan check scheduler to wake up.
func (b *NATSBus) TicklePlanCheck() {
	if err := b.nc.Publish(SubjectPlanCheck, nil); err != nil {
		slog.Warn("NATSBus: failed to publish", "subject", SubjectPlanCheck, "err", err)
	}
}

// TickleTaskRun signals the task run scheduler to wake up.
func (b *NATSBus) TickleTaskRun() {
	if err := b.nc.Publish(SubjectTaskRun, nil); err != nil {
		slog.Warn("NATSBus: failed to publish", "subject", SubjectTaskRun, "err", err)
	}
}

// RequestApprovalCheck requests approval template finding for an issue.
func (b *NATSBus) RequestApprovalCheck(ref IssueRef) {
	data, _ := json.Marshal(ref)
	if err := b.nc.Publish(SubjectApprovalCheck, data); err != nil {
		slog.Warn("NATSBus: failed to publish", "subject", SubjectApprovalCheck, "err", err)
	}
}

// RequestRolloutCreation requests automatic rollout creation for a plan.
func (b *NATSBus) RequestRolloutCreation(ref PlanRef) {
	data, _ := json.Marshal(ref)
	if err := b.nc.Publish(SubjectRolloutCreation, data); err != nil {
		slog.Warn("NATSBus: failed to publish", "subject", SubjectRolloutCreation, "err", err)
	}
}

// RequestPlanCompletionCheck requests checking if a plan is complete.
func (b *NATSBus) RequestPlanCompletionCheck(ref PlanRef) {
	data, _ := json.Marshal(ref)
	if err := b.nc.Publish(SubjectPlanCompletion, data); err != nil {
		slog.Warn("NATSBus: failed to publish", "subject", SubjectPlanCompletion, "err", err)
	}
}

// --- Cancel function registry (in-memory, same as Bus) ---

// RegisterTaskRunCancel registers a cancel function for a running task run.
func (b *NATSBus) RegisterTaskRunCancel(ref TaskRunRef, cancel context.CancelFunc) {
	b.runningTaskRunsCancelFunc.Store(ref, cancel)
}

// CancelTaskRun cancels a running task run. Returns true if successfully cancelled.
func (b *NATSBus) CancelTaskRun(ref TaskRunRef) bool {
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
func (b *NATSBus) DeregisterTaskRunCancel(ref TaskRunRef) {
	b.runningTaskRunsCancelFunc.Delete(ref)
}

// RegisterPlanCheckCancel registers a cancel function for a running plan check.
func (b *NATSBus) RegisterPlanCheckCancel(ref PlanCheckRunRef, cancel context.CancelFunc) {
	b.runningPlanCheckRunsCancelFunc.Store(ref, cancel)
}

// CancelPlanCheck cancels a running plan check. Returns true if successfully cancelled.
func (b *NATSBus) CancelPlanCheck(ref PlanCheckRunRef) bool {
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
func (b *NATSBus) DeregisterPlanCheckCancel(ref PlanCheckRunRef) {
	b.runningPlanCheckRunsCancelFunc.Delete(ref)
}

// --- Consumer channels (read by runners, unchanged) ---

// PlanCheckChan returns the channel for plan check tickles.
func (b *NATSBus) PlanCheckChan() <-chan int {
	return b.planCheckCh
}

// TaskRunChan returns the channel for task run tickles.
func (b *NATSBus) TaskRunChan() <-chan int {
	return b.taskRunCh
}

// ApprovalChan returns the channel for approval check events.
func (b *NATSBus) ApprovalChan() <-chan IssueRef {
	return b.approvalCh
}

// RolloutCreationChan returns the channel for rollout creation events.
func (b *NATSBus) RolloutCreationChan() <-chan PlanRef {
	return b.rolloutCh
}

// PlanCompletionChan returns the channel for plan completion check events.
func (b *NATSBus) PlanCompletionChan() <-chan PlanRef {
	return b.planCompletionCh
}

// --- Lifecycle & Diagnostics ---

// NATSConn returns the underlying NATS connection for advanced use.
func (b *NATSBus) NATSConn() *nats.Conn {
	return b.nc
}

// Shutdown gracefully shuts down NATS connections and the embedded server.
func (b *NATSBus) Shutdown() {
	for _, sub := range b.subs {
		_ = sub.Unsubscribe()
	}
	b.nc.Close()
	b.ns.Shutdown()
	slog.Info("NATSBus shutdown complete")
}
