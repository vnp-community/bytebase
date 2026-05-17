# T-004-01: Configurable Connection Pool

| Field | Value |
|---|---|
| **Task ID** | T-004-01 |
| **Solution** | SOL-AVAIL-004 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/store/db_connection.go` |
| **Type** | Modify existing |

---

## Objective

Thêm `PoolConfig` struct và apply pool settings vào `DBConnectionManager.Initialize()`. Hiện tại `database/sql` dùng default (unlimited connections) — cần limit để tránh exhaustion trong HA mode.

## Context — Current Code

```go
// backend/store/db_connection.go — current Initialize()
func (m *DBConnectionManager) Initialize(ctx context.Context) error {
    // ... existing connection logic ...
    // → NO pool settings applied
}
```

## Implementation

### 1. Add PoolConfig struct (top of file)

```go
// PoolConfig holds connection pool settings.
type PoolConfig struct {
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
    ConnMaxIdleTime time.Duration
}

func DefaultPoolConfig() PoolConfig {
    numCPU := runtime.NumCPU()
    return PoolConfig{
        MaxOpenConns:    max(50, numCPU*10),
        MaxIdleConns:    max(10, numCPU*2),
        ConnMaxLifetime: time.Hour,
        ConnMaxIdleTime: 15 * time.Minute,
    }
}

func PoolConfigFromEnv() PoolConfig {
    cfg := DefaultPoolConfig()
    if v := os.Getenv("PG_POOL_MAX_CONNS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.MaxOpenConns = n
        }
    }
    if v := os.Getenv("PG_POOL_MAX_IDLE_CONNS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.MaxIdleConns = n
        }
    }
    if v := os.Getenv("PG_POOL_MAX_LIFETIME"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.ConnMaxLifetime = d
        }
    }
    if v := os.Getenv("PG_POOL_MAX_IDLE_TIME"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.ConnMaxIdleTime = d
        }
    }
    return cfg
}
```

### 2. Apply in Initialize() — after DB open

```go
cfg := PoolConfigFromEnv()
db := m.GetDB()
db.SetMaxOpenConns(cfg.MaxOpenConns)
db.SetMaxIdleConns(cfg.MaxIdleConns)
db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

slog.Info("Connection pool configured",
    slog.Int("maxOpen", cfg.MaxOpenConns),
    slog.Int("maxIdle", cfg.MaxIdleConns),
    slog.Duration("maxLifetime", cfg.ConnMaxLifetime),
    slog.Duration("maxIdleTime", cfg.ConnMaxIdleTime))
```

## Acceptance Criteria

- [x] `PoolConfig` struct with `DefaultPoolConfig()` and `PoolConfigFromEnv()`
- [x] Pool settings applied in `Initialize()` after DB connection
- [x] Startup log shows pool configuration
- [x] No env vars → defaults (50 max, 10 idle, 1h lifetime, 15min idle)
- [x] `go build ./backend/store/...` passes
