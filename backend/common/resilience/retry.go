package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// MaxRetries is the maximum number of retries (not counting the first attempt). Default: 3.
	MaxRetries int
	// InitialDelay is the delay before the first retry. Default: 100ms.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries. Default: 30s.
	MaxDelay time.Duration
	// Multiplier is the backoff multiplier. Default: 2.0.
	Multiplier float64
	// Jitter randomizes the delay by 50-100% to prevent thundering herd. Default: true.
	Jitter bool
}

// DefaultRetryConfig provides sensible defaults for retry configuration.
var DefaultRetryConfig = RetryConfig{
	MaxRetries:   3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	Jitter:       true,
}

// Retry executes fn with exponential backoff.
//
// The delay sequence (without jitter) is:
//
//	InitialDelay, InitialDelay*Multiplier, InitialDelay*Multiplier^2, ..., capped at MaxDelay.
//
// With jitter enabled, each delay is randomized to 50-100% of the calculated value.
// Context cancellation stops retries immediately.
func Retry(ctx context.Context, name string, cfg RetryConfig, fn func(context.Context) error) error {
	cfg = applyDefaults(cfg)

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := calculateDelay(attempt, cfg)
			slog.Debug("Retrying operation",
				"name", name,
				"attempt", attempt,
				"delay", delay,
			)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return fmt.Errorf("retry [%s] cancelled at attempt %d: %w", name, attempt, ctx.Err())
			}
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			if attempt > 0 {
				slog.Info("Retry succeeded",
					"name", name,
					"attempt", attempt,
				)
			}
			return nil
		}
	}

	return fmt.Errorf("retry [%s] exhausted after %d attempts: %w", name, cfg.MaxRetries, lastErr)
}

func applyDefaults(cfg RetryConfig) RetryConfig {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultRetryConfig.MaxRetries
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = DefaultRetryConfig.InitialDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = DefaultRetryConfig.MaxDelay
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = DefaultRetryConfig.Multiplier
	}
	return cfg
}

func calculateDelay(attempt int, cfg RetryConfig) time.Duration {
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	if cfg.Jitter {
		// 50-100% of calculated delay
		delay = delay * (0.5 + rand.Float64()*0.5) //nolint:gosec
	}
	return time.Duration(delay)
}
