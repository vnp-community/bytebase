package store

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	JitterRatio float64
}

var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   500 * time.Millisecond,
	MaxDelay:    30 * time.Second,
	JitterRatio: 0.2,
}

func RetryableExec(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isRetryable(lastErr) {
			return lastErr
		}
		if attempt == cfg.MaxAttempts {
			break
		}
		delay := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		jitter := time.Duration(float64(delay) * cfg.JitterRatio * (rand.Float64()*2 - 1))
		delay += jitter
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		slog.Debug("Retrying DB operation",
			slog.Int("attempt", attempt+1),
			slog.String("error", lastErr.Error()))
	}
	return fmt.Errorf("max retries exceeded (%d): %w", cfg.MaxAttempts, lastErr)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40001", "40P01", "55P03":     // serialization, deadlock, lock_not_available
			return true
		case "57P01", "57P02", "57P03":     // admin/crash shutdown, cannot_connect
			return true
		case "08000", "08003", "08006":     // connection errors
			return true
		}
	}
	errStr := strings.ToLower(err.Error())
	for _, p := range []string{"connection refused", "connection reset", "broken pipe", "i/o timeout"} {
		if strings.Contains(errStr, p) {
			return true
		}
	}
	return false
}
