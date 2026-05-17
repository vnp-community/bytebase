package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/bytebase/bytebase/backend/common/log"
)

// Compile-time interface satisfaction check.
var _ EventBus = (*PGBus)(nil)

const (
	pgBusPollInterval     = 5 * time.Second
	pgBusNotifyChannel    = "bus_message"
	pgBusMaxRetries       = 5
	pgBusReconnectBackoff = 5 * time.Second
)

// PGBus implements EventBus backed by PostgreSQL for durable, HA-safe messaging.
// Messages are persisted to bus_queue, consumed via SELECT FOR UPDATE SKIP LOCKED,
// and signaled via PG LISTEN/NOTIFY for low-latency delivery.
type PGBus struct {
	db      *sql.DB
	metrics *BusMetrics

	// Embedded channel-based bus for local cancel registries and channel access.
	// PGBus delegates cancel/registration operations to the embedded Bus.
	local *Bus

	// Consumer handlers registered via Subscribe-like internal wiring.
	handlers map[string]func(ctx context.Context, payload json.RawMessage) error
	mu       sync.RWMutex
}

// NewPGBus creates a new PG-backed EventBus.
func NewPGBus(db *sql.DB, metrics *BusMetrics) *PGBus {
	local, _ := New()
	return &PGBus{
		db:       db,
		metrics:  metrics,
		local:    local,
		handlers: make(map[string]func(ctx context.Context, payload json.RawMessage) error),
	}
}

// --- EventBus interface: tickle/signal methods ---
// These persist to bus_queue for HA durability, then also fire locally for latency.

func (b *PGBus) TicklePlanCheck() {
	b.publishDurable(context.Background(), "plan.check.tickle", 0)
	b.local.TicklePlanCheck()
}

func (b *PGBus) TickleTaskRun() {
	b.publishDurable(context.Background(), "task.run.tickle", 0)
	b.local.TickleTaskRun()
}

func (b *PGBus) RequestApprovalCheck(ref IssueRef) {
	b.publishDurable(context.Background(), "approval.check", ref)
	b.local.RequestApprovalCheck(ref)
}

func (b *PGBus) RequestRolloutCreation(ref PlanRef) {
	b.publishDurable(context.Background(), "rollout.creation", ref)
	b.local.RequestRolloutCreation(ref)
}

func (b *PGBus) RequestPlanCompletionCheck(ref PlanRef) {
	b.publishDurable(context.Background(), "plan.completion.check", ref)
	b.local.RequestPlanCompletionCheck(ref)
}

// --- Cancel registry delegated to local Bus ---

func (b *PGBus) RegisterTaskRunCancel(ref TaskRunRef, cancel context.CancelFunc) {
	b.local.RegisterTaskRunCancel(ref, cancel)
}

func (b *PGBus) CancelTaskRun(ref TaskRunRef) bool {
	return b.local.CancelTaskRun(ref)
}

func (b *PGBus) DeregisterTaskRunCancel(ref TaskRunRef) {
	b.local.DeregisterTaskRunCancel(ref)
}

func (b *PGBus) RegisterPlanCheckCancel(ref PlanCheckRunRef, cancel context.CancelFunc) {
	b.local.RegisterPlanCheckCancel(ref, cancel)
}

func (b *PGBus) CancelPlanCheck(ref PlanCheckRunRef) bool {
	return b.local.CancelPlanCheck(ref)
}

func (b *PGBus) DeregisterPlanCheckCancel(ref PlanCheckRunRef) {
	b.local.DeregisterPlanCheckCancel(ref)
}

// --- Channel accessors (delegated to local Bus for backward compat) ---

func (b *PGBus) PlanCheckChan() <-chan int        { return b.local.PlanCheckChan() }
func (b *PGBus) TaskRunChan() <-chan int           { return b.local.TaskRunChan() }
func (b *PGBus) ApprovalChan() <-chan IssueRef     { return b.local.ApprovalChan() }
func (b *PGBus) RolloutCreationChan() <-chan PlanRef { return b.local.RolloutCreationChan() }
func (b *PGBus) PlanCompletionChan() <-chan PlanRef  { return b.local.PlanCompletionChan() }

// --- Durable publishing ---

func (b *PGBus) publishDurable(ctx context.Context, channel string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("pg_bus: failed to marshal payload", "channel", channel, log.BBError(err))
		return
	}

	_, err = b.db.ExecContext(ctx,
		`INSERT INTO bus_queue (channel, payload) VALUES ($1, $2)`,
		channel, data,
	)
	if err != nil {
		slog.Error("pg_bus: failed to persist message", "channel", channel, log.BBError(err))
		return
	}

	// Fire PG NOTIFY for immediate delivery to other replicas
	_, _ = b.db.ExecContext(ctx,
		"SELECT pg_notify($1, $2)", pgBusNotifyChannel, channel,
	)

	if b.metrics != nil {
		b.metrics.RecordPublish(channel)
	}
}

// StartConsumers starts the PG-backed consumer goroutines:
//  1. NOTIFY listener for immediate dispatch
//  2. Poll loop for catch-up of missed notifications
func (b *PGBus) StartConsumers(ctx context.Context, wg *sync.WaitGroup) {
	// Register default handlers for the 5 standard channels
	b.registerDefaultHandlers()

	// NOTIFY listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.runNotifyListener(ctx)
	}()

	// Poll-based consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.runPollConsumer(ctx)
	}()

	slog.Info("PGBus consumers started", "handlers", len(b.handlers))
}

// registerDefaultHandlers registers handlers for the 5 standard bus channels.
func (b *PGBus) registerDefaultHandlers() {
	// Tickle channels just trigger local channel sends (already done in Publish).
	// The handlers here process messages from OTHER replicas via the outbox.
	b.handlers["plan.check.tickle"] = func(_ context.Context, _ json.RawMessage) error {
		b.local.TicklePlanCheck()
		return nil
	}
	b.handlers["task.run.tickle"] = func(_ context.Context, _ json.RawMessage) error {
		b.local.TickleTaskRun()
		return nil
	}
	b.handlers["approval.check"] = func(_ context.Context, payload json.RawMessage) error {
		var ref IssueRef
		if err := json.Unmarshal(payload, &ref); err != nil {
			return err
		}
		b.local.RequestApprovalCheck(ref)
		return nil
	}
	b.handlers["rollout.creation"] = func(_ context.Context, payload json.RawMessage) error {
		var ref PlanRef
		if err := json.Unmarshal(payload, &ref); err != nil {
			return err
		}
		b.local.RequestRolloutCreation(ref)
		return nil
	}
	b.handlers["plan.completion.check"] = func(_ context.Context, payload json.RawMessage) error {
		var ref PlanRef
		if err := json.Unmarshal(payload, &ref); err != nil {
			return err
		}
		b.local.RequestPlanCompletionCheck(ref)
		return nil
	}
}

// runNotifyListener listens for PG NOTIFY on bus_message channel
// for low-latency cross-replica dispatch.
func (b *PGBus) runNotifyListener(ctx context.Context) {
	slog.Info("PGBus NOTIFY listener started")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := b.listenNotify(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("PGBus NOTIFY listener error, reconnecting",
					log.BBError(err),
					"backoff", pgBusReconnectBackoff,
				)
				time.Sleep(pgBusReconnectBackoff)
			}
		}
	}
}

func (b *PGBus) listenNotify(ctx context.Context) error {
	conn, err := b.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()

		_, err := pgxConn.Exec(ctx, "LISTEN "+pgBusNotifyChannel)
		if err != nil {
			return err
		}

		for {
			notification, err := pgxConn.WaitForNotification(ctx)
			if err != nil {
				return err
			}
			// notification.Payload is the channel name (e.g., "approval.check")
			b.processChannel(ctx, notification.Payload)
		}
	})
}

// runPollConsumer periodically polls bus_queue for PENDING messages.
func (b *PGBus) runPollConsumer(ctx context.Context) {
	ticker := time.NewTicker(pgBusPollInterval)
	defer ticker.Stop()

	slog.Info("PGBus poll consumer started", "interval", pgBusPollInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.pollAll(ctx)
		}
	}
}

func (b *PGBus) pollAll(ctx context.Context) {
	b.mu.RLock()
	channels := make([]string, 0, len(b.handlers))
	for ch := range b.handlers {
		channels = append(channels, ch)
	}
	b.mu.RUnlock()

	for _, ch := range channels {
		b.processChannel(ctx, ch)
	}
}

// processChannel claims and processes one pending message from a channel.
func (b *PGBus) processChannel(ctx context.Context, channel string) {
	b.mu.RLock()
	handler, ok := b.handlers[channel]
	b.mu.RUnlock()
	if !ok {
		return
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Debug("PGBus: begin tx error", "channel", channel, log.BBError(err))
		return
	}
	defer tx.Rollback() //nolint:errcheck

	// Claim one pending message
	row := tx.QueryRowContext(ctx,
		`SELECT id, payload, attempts
		 FROM bus_queue
		 WHERE channel = $1 AND status = 'pending'
		 ORDER BY priority DESC, id ASC
		 LIMIT 1
		 FOR UPDATE SKIP LOCKED`,
		channel,
	)

	var id int64
	var payload json.RawMessage
	var attempts int
	if err := row.Scan(&id, &payload, &attempts); err != nil {
		if err == sql.ErrNoRows {
			return // No pending messages
		}
		slog.Debug("PGBus: scan error", "channel", channel, log.BBError(err))
		return
	}

	// Mark as processing
	_, err = tx.ExecContext(ctx,
		`UPDATE bus_queue SET status = 'processing', claimed_at = NOW(), attempts = attempts + 1 WHERE id = $1`,
		id,
	)
	if err != nil {
		slog.Error("PGBus: claim failed", "id", id, log.BBError(err))
		return
	}

	// Dispatch to handler
	if handlerErr := handler(ctx, payload); handlerErr != nil {
		if attempts+1 >= pgBusMaxRetries {
			// Move to DLQ (failed)
			_, _ = tx.ExecContext(ctx,
				`UPDATE bus_queue SET status = 'failed', error_msg = $1, updated_at = NOW() WHERE id = $2`,
				handlerErr.Error(), id,
			)
			if b.metrics != nil {
				b.metrics.failedGauge.WithLabelValues(channel).Inc()
			}
		} else {
			// Return to pending for retry
			_, _ = tx.ExecContext(ctx,
				`UPDATE bus_queue SET status = 'pending', error_msg = $1, updated_at = NOW() WHERE id = $2`,
				handlerErr.Error(), id,
			)
		}
		if err := tx.Commit(); err != nil {
			slog.Error("PGBus: commit after handler error", log.BBError(err))
		}
		return
	}

	// Mark as done
	_, err = tx.ExecContext(ctx,
		`UPDATE bus_queue SET status = 'done', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		slog.Error("PGBus: mark done failed", "id", id, log.BBError(err))
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("PGBus: commit failed", "id", id, log.BBError(err))
		return
	}

	if b.metrics != nil {
		b.metrics.RecordConsume(channel)
	}
}
