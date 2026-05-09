package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ComponentInitFunc is the signature for component initialization functions.
type ComponentInitFunc func(ctx context.Context) error

// ParallelInit runs multiple component initializations concurrently.
// Critical components that fail cause the function to return an error.
// Important/Optional components that fail are marked as degraded/disabled.
func ParallelInit(ctx context.Context, registry *ComponentRegistry, components map[string]ComponentInitFunc) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(components))

	for name, initFn := range components {
		wg.Add(1)
		go func(name string, fn ComponentInitFunc) {
			defer wg.Done()

			start := time.Now()
			if err := fn(ctx); err != nil {
				slog.Error("Component initialization failed",
					"component", name,
					"duration", time.Since(start),
					"error", err,
				)
				status, exists := registry.HealthReport()[name]
				if exists && status.Class == Critical {
					errCh <- fmt.Errorf("critical component %s failed: %w", name, err)
				} else if exists && status.Class == Important {
					registry.SetDegraded(name, err)
				} else {
					registry.SetDisabled(name, err)
				}
				return
			}

			registry.SetHealthy(name)
			slog.Info("Component initialized",
				"component", name,
				"duration", time.Since(start),
			)
		}(name, initFn)
	}

	wg.Wait()
	close(errCh)

	// Collect all critical errors
	var criticalErrors []error
	for err := range errCh {
		criticalErrors = append(criticalErrors, err)
	}

	if len(criticalErrors) > 0 {
		return fmt.Errorf("critical initialization failures: %v", criticalErrors)
	}
	return nil
}
