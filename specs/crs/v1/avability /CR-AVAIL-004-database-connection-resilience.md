# Change Request: Database Connection Resilience & Connection Pool HA

| Field              | Value                                                       |
|--------------------|-------------------------------------------------------------|
| **CR ID**          | CR-AVAIL-004                                                |
| **Title**          | Database Connection Resilience & Connection Pool HA          |
| **Category**       | Availability / Database                                     |
| **Priority**       | P1 — High                                                   |
| **Status**         | Draft                                                       |
| **Created**        | 2026-05-08                                                  |
| **Author**         | VNP AI Ops Team                                             |
| **Regulatory**     | SBV TT09/2020 Điều 13, PCI-DSS 4.0 Req 6.5, ISO 27001 A.14|

---

## 1. Tổng quan

### 1.1 Mô tả
Tăng cường **database connection resilience** cho cả metadata PostgreSQL và managed database instances, bao gồm **connection pool HA**, **retry with backoff**, **connection validation**, và **dynamic pool sizing** — đảm bảo tính liên tục kết nối theo yêu cầu ngành tài chính.

### 1.2 Bối cảnh
Hiện tại, Bytebase sử dụng `pgx/v5` connection pool với cấu hình mặc định:
- Connection pool hardcoded (không configurable qua env) — TDD §4.1
- Không có connection validation (stale connection detection)
- Database driver connections (`plugin/db/*/`) không có retry mechanism
- Connection leak detection không có — slow queries có thể exhaust pool
- Không có graceful degradation khi pool exhausted
- Managed instance connections không được monitor theo chuẩn tài chính

### 1.3 Mục tiêu
- Connection pool configurable và auto-scaling
- Connection validation trước khi sử dụng (stale detection)
- Retry with exponential backoff cho tất cả database operations
- Connection leak detection và auto-cleanup
- Pool metrics cho monitoring và alerting
- Managed instance connection health tracking

### 1.4 Tiêu chuẩn áp dụng

| Standard                          | Requirement                                          |
|-----------------------------------|------------------------------------------------------|
| SBV TT09/2020 — Điều 13          | Kiểm soát kết nối và truy cập cơ sở dữ liệu        |
| PCI-DSS 4.0 — Req 6.5            | Connection security and error handling               |
| ISO 27001 — A.14                  | System acquisition, development, and maintenance     |

---

## 2. Yêu cầu chức năng

### FR-001: Configurable Connection Pool with Auto-Scaling
- **Mô tả**: Connection pool cho metadata PostgreSQL với cấu hình linh hoạt và auto-scaling.
- **Logic**:
  ```
  ConnectionPoolConfig:
      minConns:    max(5, numCPU)         // Minimum idle connections
      maxConns:    max(50, numCPU * 10)   // Maximum connections
      maxIdleTime: 15 minutes             // Close idle connections after
      maxLifetime: 1 hour                 // Max connection lifetime
      healthCheck: 30 seconds             // Background health check interval

  AutoScaling:
      IF pool.utilization > 80% AND pool.size < maxConns:
          pool.growBy(min(10, maxConns - pool.size))
          log("pool_scaled_up", pool.size)
      IF pool.utilization < 20% AND pool.size > minConns:
          pool.shrinkBy(min(5, pool.size - minConns))
          log("pool_scaled_down", pool.size)
  ```
- **Acceptance Criteria**:
  - AC-1: Pool size configurable via environment variables
  - AC-2: Auto-scaling responds within 30 seconds of utilization change
  - AC-3: Pool never drops below minConns
  - AC-4: Pool metrics (active, idle, waiting, total) exported to Prometheus

### FR-002: Connection Validation & Stale Detection
- **Mô tả**: Validate connections trước khi sử dụng, detect và remove stale connections.
- **Logic**:
  ```
  ConnectionValidator:
      ON acquireConnection(conn):
          IF conn.age > maxLifetime:
              conn.close()
              RETURN acquireNewConnection()
          IF conn.idleTime > healthCheckInterval:
              IF NOT conn.ping(timeout=1s):
                  conn.close()
                  metrics.increment("stale_connections_removed")
                  RETURN acquireNewConnection()
          RETURN conn

      BackgroundHealthCheck (every 30s):
          FOR each idleConn IN pool.idleConnections:
              IF NOT idleConn.ping(timeout=2s):
                  pool.removeConnection(idleConn)
                  metrics.increment("unhealthy_connections_removed")
  ```
- **Acceptance Criteria**:
  - AC-1: Stale connections detected within 30 seconds
  - AC-2: Connection validation adds < 2ms to request latency
  - AC-3: Background health check runs non-disruptively
  - AC-4: Zero application errors from stale connections

### FR-003: Retry with Exponential Backoff
- **Mô tả**: Automatic retry cho transient database errors với exponential backoff.
- **Retryable Errors**:
  | Error Type                   | Retry | Max Attempts | Base Delay |
  |------------------------------|-------|--------------|------------|
  | Connection refused           | Yes   | 5            | 500ms      |
  | Connection reset             | Yes   | 3            | 1000ms     |
  | Lock timeout                 | Yes   | 3            | 2000ms     |
  | Serialization failure        | Yes   | 3            | 500ms      |
  | Query cancelled              | No    | —            | —          |
  | Constraint violation         | No    | —            | —          |
  | Syntax error                 | No    | —            | —          |
  | Insufficient permissions     | No    | —            | —          |

- **Logic**:
  ```go
  func RetryableExecute(ctx context.Context, fn func() error, opts RetryOpts) error {
      for attempt := 0; attempt <= opts.MaxAttempts; attempt++ {
          err := fn()
          if err == nil {
              return nil
          }
          if !isRetryable(err) {
              return err
          }
          delay := opts.BaseDelay * time.Duration(math.Pow(2, float64(attempt)))
          delay = min(delay, opts.MaxDelay)  // cap at 30s
          delay += jitter(delay, 0.2)        // 20% jitter
          select {
          case <-ctx.Done():
              return ctx.Err()
          case <-time.After(delay):
          }
          metrics.Increment("db_retry_attempts", "attempt", strconv.Itoa(attempt))
      }
      return ErrMaxRetriesExceeded
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Transient errors retried automatically without caller awareness
  - AC-2: Non-retryable errors returned immediately
  - AC-3: Retry metrics tracked (attempts, successes, exhaustions)
  - AC-4: Context cancellation respected during backoff
  - AC-5: Jitter prevents thundering herd

### FR-004: Connection Leak Detection
- **Mô tả**: Detect connections held too long và auto-release với warning.
- **Logic**:
  ```
  ConnectionLeakDetector:
      threshold: 60 seconds (configurable)

      ON acquireConnection(conn):
          conn.acquiredAt = now()
          conn.acquiredStack = captureStackTrace()
          leakDetector.track(conn)

      BackgroundCheck (every 10s):
          FOR each trackedConn:
              IF trackedConn.heldTime > threshold:
                  log.Warn("potential_connection_leak", {
                      heldFor:     trackedConn.heldTime,
                      acquiredAt:  trackedConn.acquiredAt,
                      stackTrace:  trackedConn.acquiredStack,
                  })
                  metrics.Increment("connection_leak_detected")
              IF trackedConn.heldTime > threshold * 3:
                  trackedConn.forceRelease()
                  metrics.Increment("connection_leak_force_released")
                  log.Error("connection_leak_force_released", {...})
  ```
- **Acceptance Criteria**:
  - AC-1: Leak detected within 60 seconds of threshold breach
  - AC-2: Stack trace captured for debugging
  - AC-3: Force-release prevents pool exhaustion
  - AC-4: Leak metrics visible in monitoring dashboard

### FR-005: Managed Instance Connection Health
- **Mô tả**: Track health và availability cho tất cả managed database instances.
- **Logic**:
  ```
  InstanceHealthMonitor:
      FOR each instance IN store.ListInstances():
          EVERY instance.healthCheckInterval (default: 60s):
              result = driver.Ping(instance)
              store.UpdateInstanceHealth(instance.ID, {
                  status:       result.status,  // HEALTHY, DEGRADED, UNREACHABLE
                  latencyMs:    result.latency,
                  lastChecked:  now(),
                  errorMessage: result.error,
                  consecutiveFailures: count,
              })
              IF result.consecutiveFailures >= 3:
                  alertOperations("instance_unhealthy", instance)
              IF result.latencyMs > instance.latencyThreshold:
                  alertOperations("instance_slow", instance)
  ```
- **Acceptance Criteria**:
  - AC-1: All instances health-checked at configured intervals
  - AC-2: Unhealthy instances alerted after 3 consecutive failures
  - AC-3: Instance health visible in admin dashboard
  - AC-4: Health check history retained 30 days for trend analysis

### FR-006: Connection Pool Graceful Degradation
- **Mô tả**: Graceful degradation khi pool exhausted — thay vì hang, trả lời clear error.
- **Logic**:
  ```
  ON acquireConnection():
      IF pool.available == 0:
          IF pool.waiting > pool.maxWaitQueue (100):
              RETURN error("pool_exhausted",
                  retryAfter: estimatedWaitTime,
                  suggestion: "try again later")
          waitResult = pool.waitForConnection(timeout=5s)
          IF waitResult.timeout:
              metrics.Increment("pool_wait_timeout")
              RETURN error("pool_timeout",
                  retryAfter: 5,
                  queueDepth: pool.waiting)
          RETURN waitResult.conn
  ```
- **Acceptance Criteria**:
  - AC-1: Pool exhaustion returns 503 with Retry-After header
  - AC-2: Wait queue bounded to prevent unbounded memory growth
  - AC-3: Pool exhaustion metric triggers alert within 30 seconds
  - AC-4: Clear error messages for debugging

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|---------------------------------------------------|
| Pool Manager                 | `backend/store/pool_manager.go`           | Configurable pool with auto-scaling               |
| Connection Validator         | `backend/store/conn_validator.go`         | Stale connection detection and validation         |
| Retry Engine                 | `backend/store/retry.go`                 | Exponential backoff retry for DB operations       |
| Leak Detector                | `backend/store/leak_detector.go`         | Connection leak detection with stack traces       |
| Instance Health Monitor      | `backend/runner/instancehealth/monitor.go`| Managed instance health checking                  |
| Pool Metrics                 | `backend/metrics/pool_metrics.go`        | Connection pool Prometheus metrics                |
| Store Constructor            | `backend/store/store.go`                 | Wire pool config from env, leak detector          |
| DB Connection Manager        | `backend/store/db.go`                    | Enhanced with retry, validation, leak detection   |
| Driver Retry Wrapper         | `backend/plugin/db/retry_driver.go`      | Retry wrapper for managed DB connections          |

### 3.2 Configuration

| Environment Variable          | Default     | Mô tả                                                |
|-------------------------------|-------------|-------------------------------------------------------|
| `PG_POOL_MIN_CONNS`          | `5`         | Minimum idle connections in pool                     |
| `PG_POOL_MAX_CONNS`          | `50`        | Maximum connections in pool                          |
| `PG_POOL_MAX_IDLE_TIME`      | `15m`       | Close idle connections after this duration           |
| `PG_POOL_MAX_LIFETIME`       | `1h`        | Maximum connection lifetime                          |
| `PG_POOL_HEALTH_CHECK_SEC`   | `30`        | Background health check interval                     |
| `PG_RETRY_MAX_ATTEMPTS`      | `5`         | Maximum retry attempts for transient errors          |
| `PG_RETRY_BASE_DELAY_MS`     | `500`       | Base delay for exponential backoff                   |
| `PG_RETRY_MAX_DELAY_MS`      | `30000`     | Maximum delay cap for backoff                        |
| `CONN_LEAK_THRESHOLD_SEC`    | `60`        | Connection held time before leak warning             |
| `CONN_LEAK_FORCE_RELEASE_SEC`| `180`       | Connection held time before force release            |
| `INSTANCE_HEALTH_CHECK_SEC`  | `60`        | Managed instance health check interval               |
| `INSTANCE_LATENCY_THRESHOLD` | `5000`      | Latency threshold (ms) for slow instance alert       |

### 3.3 Prometheus Metrics

```prometheus
# Connection Pool
bytebase_db_pool_size{pool="primary"}           # Total connections
bytebase_db_pool_active{pool="primary"}         # Active connections
bytebase_db_pool_idle{pool="primary"}           # Idle connections
bytebase_db_pool_waiting{pool="primary"}        # Waiting requests
bytebase_db_pool_max{pool="primary"}            # Max pool size
bytebase_db_pool_utilization{pool="primary"}    # Utilization ratio

# Connection Lifecycle
bytebase_db_conn_acquired_total
bytebase_db_conn_released_total
bytebase_db_conn_stale_removed_total
bytebase_db_conn_leak_detected_total
bytebase_db_conn_leak_force_released_total
bytebase_db_conn_wait_duration_seconds

# Retry
bytebase_db_retry_attempts_total{error_type="connection_reset"}
bytebase_db_retry_success_total
bytebase_db_retry_exhausted_total

# Instance Health
bytebase_instance_health_status{instance="inst-001"}  # 0=unreachable, 1=degraded, 2=healthy
bytebase_instance_health_latency_ms{instance="inst-001"}
bytebase_instance_health_failures_consecutive{instance="inst-001"}
```

### 3.4 Database Changes

```sql
-- Instance health tracking
CREATE TABLE IF NOT EXISTS instance_health (
    instance_id     TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'UNKNOWN',
    -- Status: HEALTHY, DEGRADED, UNREACHABLE, UNKNOWN
    latency_ms      FLOAT       NOT NULL DEFAULT 0,
    error_message   TEXT,
    consecutive_failures INT    NOT NULL DEFAULT 0,
    last_healthy_at TIMESTAMPTZ,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id)
);

-- Instance health history
CREATE TABLE IF NOT EXISTS instance_health_log (
    id              BIGSERIAL   PRIMARY KEY,
    instance_id     TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    latency_ms      FLOAT       NOT NULL DEFAULT 0,
    error_message   TEXT,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_instance_health_log_instance_time
    ON instance_health_log (instance_id, checked_at DESC);

-- Retain 30 days
```

### 3.5 Frontend Changes

| Component                    | Thay đổi                                          |
|------------------------------|---------------------------------------------------|
| Instance health indicator    | Color-coded health status in instance list        |
| Pool dashboard (admin)       | Connection pool utilization charts                |
| Retry feedback               | Show retry status in SQL Editor on transient errors|

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| CR-AVAIL-003            | Circuit Breaker — coordinates with connection resilience        |
| pgx/v5                  | Existing PostgreSQL driver (enhanced configuration)             |
| Prometheus              | Metrics collection for pool monitoring                          |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                         |
|------------|-----------------------------------------------------------------|-----------------------------------------|
| TC-001     | Pool auto-scales under high load                               | Pool size increases, no connection errors|
| TC-002     | Pool shrinks when load decreases                               | Idle connections released after timeout |
| TC-003     | Stale connection detection                                     | Stale conn replaced within 30s          |
| TC-004     | Retry on connection reset error                                | Auto-retry succeeds, transparent        |
| TC-005     | Retry on lock timeout                                          | Retry with backoff, succeeds            |
| TC-006     | Non-retryable error (syntax)                                   | Immediate error return, no retry        |
| TC-007     | Connection leak detection                                      | Warning logged with stack trace         |
| TC-008     | Connection leak force release                                  | Connection released, metric incremented |
| TC-009     | Pool exhaustion                                                | 503 with Retry-After header             |
| TC-010     | Instance health check — healthy instance                       | Status: HEALTHY, latency logged         |
| TC-011     | Instance health check — unreachable instance                   | Status: UNREACHABLE, alert fired        |
| TC-012     | Pool metrics in Prometheus                                     | All pool metrics present and accurate   |
| TC-013     | Jitter prevents thundering herd on retry                       | Distributed retry timing observed       |
| TC-014     | Context cancellation during retry backoff                      | Retry aborted cleanly                   |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Configurable pool + env variables               | Sprint 1       |
| Phase 2 | Connection validation + stale detection          | Sprint 1       |
| Phase 3 | Retry engine with backoff                        | Sprint 2       |
| Phase 4 | Leak detection                                   | Sprint 2       |
| Phase 5 | Instance health monitor                          | Sprint 3       |
| Phase 6 | Pool metrics + Grafana dashboard                | Sprint 3       |
| Phase 7 | Load testing & validation                       | Sprint 4       |

---

## 7. Risks & Mitigations

| Risk                                         | Impact | Mitigation                                                |
|----------------------------------------------|--------|-----------------------------------------------------------|
| Connection validation overhead               | LOW    | Only validate idle > healthCheckInterval                  |
| Leak detector false positives (long queries) | MEDIUM | Configurable threshold, whitelist for batch operations    |
| Retry amplifies load during outage           | HIGH   | Circuit breaker integration, backoff with jitter          |
| Pool auto-scaling oscillation                | LOW    | Hysteresis (scale up > 80%, scale down < 20%)            |
| Stack trace capture performance              | LOW    | Sampling (capture every 10th acquisition only)            |
