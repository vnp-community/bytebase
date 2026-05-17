package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bytebase/bytebase/backend/common/resilience"
)

// PoolType identifies the purpose of a connection pool.
type PoolType int

const (
	// PoolAPI is for API handler queries (latency-sensitive).
	PoolAPI PoolType = iota
	// PoolRunner is for background runner queries (throughput-oriented).
	PoolRunner
)

// Pool sizing constraints.
const (
	defaultMaxConns      = 50
	defaultAPIRatio      = 0.7
	defaultRunnerRatio   = 0.3
	defaultMinPerPool    = 5
	maxTotalConns        = 200
	defaultDrainTimeout  = 5 * time.Minute
	metricsInterval      = 5 * time.Second
	fileStabilityRetries = 5
	fileStabilityDelay   = 50 * time.Millisecond
)

// PoolConfig configures dual connection pool management.
type PoolConfig struct {
	// MaxConns is the total maximum connections across both pools.
	// 0 = auto-detect from PG max_connections.
	MaxConns int
	// APIRatio is the fraction of connections allocated to the API pool. Default: 0.7.
	APIRatio float64
	// RunnerRatio is the fraction allocated to the Runner pool. Default: 0.3.
	RunnerRatio float64
	// MinConnsPerPool is the minimum connections per pool. Default: 5.
	MinConnsPerPool int
	// DrainTimeout is the maximum time to wait for old pool connections to drain
	// during a reconnection. Default: 5 minutes.
	DrainTimeout time.Duration
}

// PoolManager manages dual connection pools (API vs Runner) to prevent
// resource exhaustion from background operations starving API requests.
//
// The API pool (default 70%) serves latency-sensitive API handler queries.
// The Runner pool (default 30%) serves throughput-oriented background tasks
// like schema sync, task runs, and data cleanup.
type PoolManager struct {
	mu         sync.RWMutex
	apiPool    *sql.DB
	runnerPool *sql.DB
	config     PoolConfig
	pgURL      string

	// File watcher for dynamic PG URL updates
	watcher     *fsnotify.Watcher
	stopWatcher chan struct{}
	reconnectCB *resilience.CircuitBreaker

	// Metrics
	metrics    *PoolMetrics
	reconnects prometheus.Counter
}

// NewPoolManager creates and initializes dual connection pools.
// If pgURL is a file path, the manager watches it for changes and
// performs atomic pool swaps on update.
func NewPoolManager(ctx context.Context, pgURL string, cfg PoolConfig) (*PoolManager, error) {
	if pgURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	cfg = applyPoolDefaults(cfg)

	resolvedURL := pgURL
	isFile := isFilePath(pgURL)
	if isFile {
		var err error
		resolvedURL, err = readURLFromFile(pgURL)
		if err != nil {
			return nil, fmt.Errorf("failed to read PG URL from file: %w", err)
		}
	}

	pm := &PoolManager{
		config:      cfg,
		pgURL:       pgURL,
		stopWatcher: make(chan struct{}),
		reconnectCB: resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
			Name:         "pool-reconnect",
			MaxFailures:  3,
			ResetTimeout: 30 * time.Second,
		}),
		reconnects: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bytebase",
			Subsystem: "db_pool",
			Name:      "reconnects_total",
			Help:      "Total number of connection pool reconnections.",
		}),
	}

	// Auto-detect max_connections if not configured
	effectiveMax := cfg.MaxConns
	if effectiveMax <= 0 {
		detected, err := probeMaxConnections(ctx, resolvedURL)
		if err != nil {
			slog.Warn("Failed to auto-detect max_connections, using default",
				"error", err, "default", defaultMaxConns)
			effectiveMax = defaultMaxConns
		} else {
			effectiveMax = detected
		}
	}

	// Apply bounds
	if effectiveMax > maxTotalConns {
		effectiveMax = maxTotalConns
	}
	if effectiveMax < cfg.MinConnsPerPool*2 {
		effectiveMax = cfg.MinConnsPerPool * 2
	}

	apiConns, runnerConns := computePoolSizes(effectiveMax, cfg)

	slog.Info("Creating dual connection pools",
		"api_max", apiConns,
		"runner_max", runnerConns,
		"total", apiConns+runnerConns,
		"auto_detected", cfg.MaxConns <= 0,
	)

	apiPool, err := createConnectionPool(ctx, resolvedURL, apiConns)
	if err != nil {
		return nil, fmt.Errorf("failed to create API pool: %w", err)
	}

	runnerPool, err := createConnectionPool(ctx, resolvedURL, runnerConns)
	if err != nil {
		apiPool.Close()
		return nil, fmt.Errorf("failed to create Runner pool: %w", err)
	}

	pm.apiPool = apiPool
	pm.runnerPool = runnerPool

	// Start file watcher if PG URL is from a file
	if isFile {
		if err := pm.startFileWatcher(ctx, pgURL); err != nil {
			apiPool.Close()
			runnerPool.Close()
			return nil, fmt.Errorf("failed to start file watcher: %w", err)
		}
	}

	return pm, nil
}

// GetDB returns a pool by type.
func (pm *PoolManager) GetDB(poolType PoolType) *sql.DB {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	switch poolType {
	case PoolRunner:
		return pm.runnerPool
	default:
		return pm.apiPool
	}
}

// GetDefaultDB returns the API pool for backward compatibility with Store.GetDB().
func (pm *PoolManager) GetDefaultDB() *sql.DB {
	return pm.GetDB(PoolAPI)
}

// APIPool returns the API connection pool.
func (pm *PoolManager) APIPool() *sql.DB {
	return pm.GetDB(PoolAPI)
}

// RunnerPool returns the Runner connection pool.
func (pm *PoolManager) RunnerPool() *sql.DB {
	return pm.GetDB(PoolRunner)
}

// StartMetricsCollector starts a background goroutine collecting pool metrics.
func (pm *PoolManager) StartMetricsCollector(ctx context.Context, registerer prometheus.Registerer) {
	pm.metrics = NewPoolMetrics(registerer)

	// Register reconnect counter
	if registerer != nil {
		registerer.MustRegister(pm.reconnects)
	}

	pm.metrics.RunCollector(ctx, metricsInterval, pm.poolMap())
}

// Close stops the file watcher and closes both pools.
func (pm *PoolManager) Close() error {
	// Stop file watcher
	if pm.watcher != nil {
		close(pm.stopWatcher)
		pm.watcher.Close()
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errs []error
	if pm.apiPool != nil {
		if err := pm.apiPool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("api pool: %w", err))
		}
	}
	if pm.runnerPool != nil {
		if err := pm.runnerPool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("runner pool: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("pool manager close errors: %v", errs)
	}
	return nil
}

// GetPgURL returns the original PG URL or file path.
func (pm *PoolManager) GetPgURL() string {
	return pm.pgURL
}

// ──────────────────────────────────────────────────────────────────────────────
// Reconnection with file stability check + atomic pool swap
// ──────────────────────────────────────────────────────────────────────────────

func (pm *PoolManager) startFileWatcher(ctx context.Context, filePath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	pm.watcher = watcher

	if err := watcher.Add(filePath); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch file %s: %w", filePath, err)
	}

	go pm.watchFile(ctx, filePath)
	return nil
}

func (pm *PoolManager) watchFile(ctx context.Context, filePath string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopWatcher:
			return
		case event, ok := <-pm.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				pm.reloadConnection(ctx, filePath)
			}
		case err, ok := <-pm.watcher.Errors:
			if ok && err != nil {
				slog.Error("Pool manager file watcher error", "error", err)
			}
		}
	}
}

// reloadConnection performs a robust reconnection with:
// 1. File stability check (read twice, verify stable)
// 2. Atomic pool swap (RWMutex)
// 3. Graceful drain with configurable timeout
func (pm *PoolManager) reloadConnection(ctx context.Context, filePath string) {
	err := pm.reconnectCB.Execute(ctx, func(ctx context.Context) error {
		return resilience.Retry(ctx, "pool-reconnect", resilience.RetryConfig{
			MaxRetries:   3,
			InitialDelay: 500 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
		}, func(ctx context.Context) error {
			return pm.doReloadConnection(ctx, filePath)
		})
	})
	if err != nil {
		slog.Error("Pool reconnection failed (circuit breaker may be open)",
			"error", err, "file", filePath)
	}
}

func (pm *PoolManager) doReloadConnection(ctx context.Context, filePath string) error {
	// File stability check: read twice with delay, verify content is stable
	newURL, err := readStableURL(filePath)
	if err != nil {
		return fmt.Errorf("file stability check failed: %w", err)
	}

	slog.Info("PG URL file content updated, reconnecting dual pools")

	cfg := pm.config
	apiConns, runnerConns := computePoolSizes(cfg.MaxConns, cfg)

	newAPIPool, err := createConnectionPool(ctx, newURL, apiConns)
	if err != nil {
		return fmt.Errorf("failed to create new API pool: %w", err)
	}

	newRunnerPool, err := createConnectionPool(ctx, newURL, runnerConns)
	if err != nil {
		newAPIPool.Close()
		return fmt.Errorf("failed to create new Runner pool: %w", err)
	}

	// Atomic pool swap
	pm.mu.Lock()
	oldAPI, oldRunner := pm.apiPool, pm.runnerPool
	pm.apiPool = newAPIPool
	pm.runnerPool = newRunnerPool
	pm.mu.Unlock()

	pm.reconnects.Inc()
	slog.Info("Dual pool connection updated successfully", "file", filePath)

	// Graceful drain of old pools
	drainTimeout := pm.config.DrainTimeout
	go drainPool("old-api", oldAPI, drainTimeout)
	go drainPool("old-runner", oldRunner, drainTimeout)

	return nil
}

// readStableURL reads the URL file twice with a delay to ensure the file write is complete.
func readStableURL(filePath string) (string, error) {
	for i := 0; i < fileStabilityRetries; i++ {
		url1, err := readURLFromFile(filePath)
		if err != nil {
			return "", err
		}
		time.Sleep(fileStabilityDelay)
		url2, err := readURLFromFile(filePath)
		if err != nil {
			return "", err
		}
		if url1 == url2 {
			return url1, nil
		}
		slog.Debug("File content unstable, retrying",
			"attempt", i+1, "file", filePath)
	}
	return "", fmt.Errorf("file content did not stabilize after %d retries", fileStabilityRetries)
}

// drainPool gracefully drains a pool within the given timeout.
func drainPool(name string, db *sql.DB, timeout time.Duration) {
	if db == nil {
		return
	}

	// Signal the pool to stop creating new idle connections
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(1 * time.Minute)

	// Force close after timeout as a safety measure
	time.Sleep(timeout)
	if err := db.Close(); err != nil {
		slog.Warn("Failed to force close old pool", "pool", name, "error", err)
	} else {
		slog.Info("Old pool drained and closed", "pool", name)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

func (pm *PoolManager) poolMap() map[string]*sql.DB {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return map[string]*sql.DB{
		"api":    pm.apiPool,
		"runner": pm.runnerPool,
	}
}

// probeMaxConnections queries PG for max_connections and superuser_reserved_connections,
// returning the effective available connections.
func probeMaxConnections(ctx context.Context, pgURL string) (int, error) {
	db, err := createConnectionWithTracer(ctx, pgURL)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var maxConns, reserved int
	if err := db.QueryRowContext(ctx, "SHOW max_connections").Scan(&maxConns); err != nil {
		return 0, fmt.Errorf("failed to query max_connections: %w", err)
	}
	if err := db.QueryRowContext(ctx, "SHOW superuser_reserved_connections").Scan(&reserved); err != nil {
		return 0, fmt.Errorf("failed to query superuser_reserved_connections: %w", err)
	}

	// Use 70% of available connections to leave room for other clients
	effective := int(float64(maxConns-reserved) * 0.7)
	slog.Info("Auto-detected PG connection limits",
		"max_connections", maxConns,
		"reserved", reserved,
		"effective_for_bytebase", effective,
	)
	return effective, nil
}

// createConnectionPool creates a single *sql.DB pool with the given max connections.
func createConnectionPool(ctx context.Context, pgURL string, maxOpen int) (*sql.DB, error) {
	db, err := createConnectionWithTracer(ctx, pgURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen / 2)
	return db, nil
}

func computePoolSizes(totalConns int, cfg PoolConfig) (apiConns, runnerConns int) {
	apiConns = int(math.Max(float64(cfg.MinConnsPerPool), math.Floor(float64(totalConns)*cfg.APIRatio)))
	runnerConns = int(math.Max(float64(cfg.MinConnsPerPool), math.Floor(float64(totalConns)*cfg.RunnerRatio)))
	return apiConns, runnerConns
}

func applyPoolDefaults(cfg PoolConfig) PoolConfig {
	if cfg.APIRatio <= 0 {
		cfg.APIRatio = defaultAPIRatio
	}
	if cfg.RunnerRatio <= 0 {
		cfg.RunnerRatio = defaultRunnerRatio
	}
	if cfg.MinConnsPerPool <= 0 {
		cfg.MinConnsPerPool = defaultMinPerPool
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = defaultDrainTimeout
	}
	return cfg
}

// isFilePath and readURLFromFile are shared with db_connection.go.
// They are defined in db_connection.go and reused here.

// readURLFromFileLocal is used only when db_connection.go is removed.
func readURLFromFileLocal(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read database URL from file %s: %w", path, err)
	}
	return strings.TrimSpace(string(content)), nil
}
