package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
)

// PoolType identifies the purpose of a connection pool.
type PoolType int

const (
	// PoolAPI is for API handler queries (latency-sensitive).
	PoolAPI PoolType = iota
	// PoolRunner is for background runner queries (throughput-oriented).
	PoolRunner
)

// PoolConfig configures dual connection pool management.
type PoolConfig struct {
	// PGURL is the PostgreSQL connection string.
	PGURL string
	// MaxConns is the total maximum connections across both pools. Default: 50.
	MaxConns int
	// APIRatio is the fraction of connections allocated to the API pool. Default: 0.7.
	APIRatio float64
	// RunnerRatio is the fraction allocated to the Runner pool. Default: 0.3.
	RunnerRatio float64
	// MinConnsPerPool is the minimum connections per pool. Default: 5.
	MinConnsPerPool int
}

// PoolManager manages dual connection pools (API vs Runner) to prevent
// resource exhaustion from background operations starving API requests.
type PoolManager struct {
	apiPool    *sql.DB
	runnerPool *sql.DB
	config     PoolConfig
}

// NewPoolManager creates and validates dual connection pools.
func NewPoolManager(ctx context.Context, cfg PoolConfig) (*PoolManager, error) {
	cfg = applyPoolDefaults(cfg)

	apiConns := int(math.Max(float64(cfg.MinConnsPerPool), math.Floor(float64(cfg.MaxConns)*cfg.APIRatio)))
	runnerConns := int(math.Max(float64(cfg.MinConnsPerPool), math.Floor(float64(cfg.MaxConns)*cfg.RunnerRatio)))

	slog.Info("Creating dual connection pools",
		"api_max", apiConns,
		"runner_max", runnerConns,
		"total", apiConns+runnerConns,
	)

	apiPool, err := createConnectionWithTracer(ctx, cfg.PGURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create API pool: %w", err)
	}
	apiPool.SetMaxOpenConns(apiConns)
	apiPool.SetMaxIdleConns(apiConns / 2)

	runnerPool, err := createConnectionWithTracer(ctx, cfg.PGURL)
	if err != nil {
		apiPool.Close()
		return nil, fmt.Errorf("failed to create Runner pool: %w", err)
	}
	runnerPool.SetMaxOpenConns(runnerConns)
	runnerPool.SetMaxIdleConns(runnerConns / 2)

	return &PoolManager{
		apiPool:    apiPool,
		runnerPool: runnerPool,
		config:     cfg,
	}, nil
}

// APIPool returns the pool for API handler queries.
func (pm *PoolManager) APIPool() *sql.DB {
	return pm.apiPool
}

// RunnerPool returns the pool for background runner queries.
func (pm *PoolManager) RunnerPool() *sql.DB {
	return pm.runnerPool
}

// Close closes both pools.
func (pm *PoolManager) Close() error {
	var errs []error
	if err := pm.apiPool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("api pool: %w", err))
	}
	if err := pm.runnerPool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("runner pool: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("pool manager close errors: %v", errs)
	}
	return nil
}

func applyPoolDefaults(cfg PoolConfig) PoolConfig {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 50
	}
	if cfg.APIRatio <= 0 {
		cfg.APIRatio = 0.7
	}
	if cfg.RunnerRatio <= 0 {
		cfg.RunnerRatio = 0.3
	}
	if cfg.MinConnsPerPool <= 0 {
		cfg.MinConnsPerPool = 5
	}
	return cfg
}
