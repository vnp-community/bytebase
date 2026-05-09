package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// DurablePublisher provides persistent message publishing via PostgreSQL.
// It bridges the volatile in-memory Bus with a PG-backed queue for HA deployments.
//
// Messages are stored in the bus_queue table and consumed via SELECT FOR UPDATE SKIP LOCKED,
// which allows safe multi-instance consumption without duplicate delivery.
type DurablePublisher struct {
	db *sql.DB
}

// NewDurablePublisher creates a publisher backed by the given database connection.
func NewDurablePublisher(db *sql.DB) *DurablePublisher {
	return &DurablePublisher{db: db}
}

// Publish persists a message to the bus_queue table.
func (p *DurablePublisher) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("durable bus: marshal payload: %w", err)
	}
	_, err = p.db.ExecContext(ctx,
		`INSERT INTO bus_queue (channel, payload) VALUES ($1, $2)`,
		channel, data,
	)
	if err != nil {
		return fmt.Errorf("durable bus: publish to %s: %w", channel, err)
	}
	return nil
}

// DurableConsumer polls the bus_queue table and dispatches messages to handlers.
type DurableConsumer struct {
	db       *sql.DB
	handlers map[string]func(ctx context.Context, payload json.RawMessage) error
	interval time.Duration
}

// NewDurableConsumer creates a consumer with the given polling interval.
func NewDurableConsumer(db *sql.DB, interval time.Duration) *DurableConsumer {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &DurableConsumer{
		db:       db,
		handlers: make(map[string]func(ctx context.Context, payload json.RawMessage) error),
		interval: interval,
	}
}

// Handle registers a handler for a given channel.
func (c *DurableConsumer) Handle(channel string, fn func(ctx context.Context, payload json.RawMessage) error) {
	c.handlers[channel] = fn
}

// Run starts the consumer loop. It polls for pending messages and dispatches them.
// Uses SELECT FOR UPDATE SKIP LOCKED for safe multi-instance consumption.
func (c *DurableConsumer) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	slog.Info("Durable bus consumer started", "interval", c.interval, "channels", len(c.handlers))

	for {
		select {
		case <-ctx.Done():
			slog.Info("Durable bus consumer stopped")
			return
		case <-ticker.C:
			c.poll(ctx)
		}
	}
}

// poll claims and processes pending messages.
func (c *DurableConsumer) poll(ctx context.Context) {
	for channel := range c.handlers {
		if err := c.processChannel(ctx, channel); err != nil {
			slog.Warn("Durable bus: error processing channel",
				"channel", channel, "error", err)
		}
	}
}

func (c *DurableConsumer) processChannel(ctx context.Context, channel string) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Claim one pending message (SKIP LOCKED avoids contention across replicas)
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
			return nil // nothing to process
		}
		return fmt.Errorf("scan: %w", err)
	}

	// Mark as processing
	_, err = tx.ExecContext(ctx,
		`UPDATE bus_queue SET status = 'processing', claimed_at = NOW(), attempts = attempts + 1 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("claim message %d: %w", id, err)
	}

	// Dispatch to handler
	handler := c.handlers[channel]
	if handlerErr := handler(ctx, payload); handlerErr != nil {
		// Check if retries are exhausted
		if attempts+1 >= 3 { // max_retries default
			_, _ = tx.ExecContext(ctx,
				`UPDATE bus_queue SET status = 'failed', error_msg = $1, updated_at = NOW() WHERE id = $2`,
				handlerErr.Error(), id,
			)
		} else {
			// Return to pending for retry
			_, _ = tx.ExecContext(ctx,
				`UPDATE bus_queue SET status = 'pending', error_msg = $1, updated_at = NOW() WHERE id = $2`,
				handlerErr.Error(), id,
			)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("commit after handler error: %w", commitErr)
		}
		return nil // Don't propagate handler errors
	}

	// Mark as done
	_, err = tx.ExecContext(ctx,
		`UPDATE bus_queue SET status = 'done', completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark done %d: %w", id, err)
	}

	return tx.Commit()
}

// CleanupCompleted removes processed messages older than the given age.
// Should be run periodically (e.g., every hour) to prevent table bloat.
func CleanupCompleted(ctx context.Context, db *sql.DB, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.ExecContext(ctx,
		`DELETE FROM bus_queue WHERE status IN ('done', 'failed') AND completed_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("durable bus cleanup: %w", err)
	}
	return result.RowsAffected()
}
