package v1

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
)

// BusAdminService provides DLQ inspection, replay, and purge endpoints
// for the durable bus queue.
type BusAdminService struct {
	db *sql.DB
}

// NewBusAdminService creates a new bus admin service.
func NewBusAdminService(db *sql.DB) *BusAdminService {
	return &BusAdminService{db: db}
}

// DLQMessage represents a failed message in the dead-letter queue.
type DLQMessage struct {
	ID        int64     `json:"id"`
	Channel   string    `json:"channel"`
	Payload   string    `json:"payload"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	ErrorMsg  string    `json:"error_msg"`
	CreatedAt time.Time `json:"created_at"`
}

// ListDLQMessagesRequest is the request for listing DLQ messages.
type ListDLQMessagesRequest struct {
	Channel  string
	PageSize int
	Offset   int
}

// ListDLQMessages returns failed messages from the bus queue.
func (s *BusAdminService) ListDLQMessages(ctx context.Context, req *ListDLQMessagesRequest) ([]*DLQMessage, int64, error) {
	if req.PageSize <= 0 {
		req.PageSize = 50
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) FROM bus_queue WHERE status = 'failed'`
	args := []any{}
	if req.Channel != "" {
		countQuery += ` AND channel = $1`
		args = append(args, req.Channel)
	}
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count DLQ: %w", err)
	}

	// Fetch page
	query := `SELECT id, channel, payload::TEXT, status, attempts, COALESCE(error_msg, ''), created_at
		FROM bus_queue WHERE status = 'failed'`
	fetchArgs := []any{}
	argIdx := 1
	if req.Channel != "" {
		query += fmt.Sprintf(` AND channel = $%d`, argIdx)
		fetchArgs = append(fetchArgs, req.Channel)
		argIdx++
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	fetchArgs = append(fetchArgs, req.PageSize, req.Offset)

	rows, err := s.db.QueryContext(ctx, query, fetchArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list DLQ: %w", err)
	}
	defer rows.Close()

	var messages []*DLQMessage
	for rows.Next() {
		msg := &DLQMessage{}
		if err := rows.Scan(&msg.ID, &msg.Channel, &msg.Payload, &msg.Status, &msg.Attempts, &msg.ErrorMsg, &msg.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan DLQ: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, total, rows.Err()
}

// ReplayDLQMessage resets a failed message to pending for reprocessing.
func (s *BusAdminService) ReplayDLQMessage(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bus_queue SET status = 'pending', attempts = 0, error_msg = NULL, updated_at = NOW()
		 WHERE id = $1 AND status = 'failed'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("replay DLQ message %d: %w", id, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("DLQ message %d not found or not in failed status", id)
	}

	slog.Info("DLQ message replayed", "id", id)
	return nil
}

// ReplayAllDLQ resets all failed messages in a channel to pending.
func (s *BusAdminService) ReplayAllDLQ(ctx context.Context, channel string) (int64, error) {
	query := `UPDATE bus_queue SET status = 'pending', attempts = 0, error_msg = NULL, updated_at = NOW()
		 WHERE status = 'failed'`
	args := []any{}
	if channel != "" {
		query += ` AND channel = $1`
		args = append(args, channel)
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("replay all DLQ: %w", err)
	}

	rows, _ := result.RowsAffected()
	slog.Info("DLQ messages replayed", "count", rows, "channel", channel)
	return rows, nil
}

// PurgeDLQ deletes failed messages older than the given date.
func (s *BusAdminService) PurgeDLQ(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM bus_queue WHERE status = 'failed' AND created_at < $1`,
		before,
	)
	if err != nil {
		return 0, fmt.Errorf("purge DLQ: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		slog.Info("DLQ messages purged", "count", rows, "before", before)
	}
	return rows, nil
}

// GetDLQStats returns summary statistics for the DLQ.
func (s *BusAdminService) GetDLQStats(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT channel, COUNT(*) FROM bus_queue WHERE status = 'failed' GROUP BY channel`,
	)
	if err != nil {
		return nil, fmt.Errorf("DLQ stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var channel string
		var count int64
		if err := rows.Scan(&channel, &count); err != nil {
			slog.Warn("DLQ stats scan error", log.BBError(err))
			continue
		}
		stats[channel] = count
	}
	return stats, rows.Err()
}
