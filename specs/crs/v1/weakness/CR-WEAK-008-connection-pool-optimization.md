# Change Request: Metadata DB Connection Pool & Observability

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-008                                              |
| **Weakness ID**    | WEAK-008                                                 |
| **Title**          | Connection Pool Optimization & DB Observability          |
| **Category**       | Performance / Scalability                                |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | ADM-08 (API Integration), DCM-01 (Change Management)    |

---

## 1. Tổng quan

### 1.1 Mô tả
Tối ưu connection pool management cho metadata PostgreSQL: configurable pool size, separate pools cho API vs runners, connection pool metrics, và fix reconnection race conditions.

### 1.2 Bối cảnh
- Connection pool hard-capped tại **50** bất kể PG `max_connections`
- 8 background runners + API requests + LSP + MCP chia sẻ cùng pool
- `reloadConnection()` sử dụng `time.Sleep(100ms)` — race condition khi file chưa ghi xong
- 1 hour grace period cho old connections → potential resource leak
- Không có pool metrics → exhaustion invisible cho operators

### 1.3 Mục tiêu
- Configurable pool size qua environment variable
- Separate connection pools cho API (high priority) vs runners (background)
- Pool utilization metrics exported to Prometheus
- Robust reconnection without arbitrary sleeps
- Connection health monitoring với alerting

---

## 2. Yêu cầu chức năng

### FR-001: Configurable Connection Pool
- **Mô tả**: Pool size configurable qua environment variable, với dynamic scaling.
- **Logic**:
  ```go
  maxPool := getEnvInt("PG_MAX_CONNECTIONS", 0)
  if maxPool == 0 {
      // Auto-detect: use 70% of available PG connections
      maxPool = int(float64(maxConns - reservedConns) * 0.7)
  }
  if maxPool > 200 {
      maxPool = 200  // Safety cap
  }
  if maxPool < 10 {
      maxPool = 10   // Minimum viable
  }
  ```
- **Acceptance Criteria**:
  - AC-1: `PG_MAX_CONNECTIONS=0` → auto-detect from PG max_connections
  - AC-2: Manual override respected (within 10-200 range)
  - AC-3: Startup log shows effective pool size

### FR-002: Dual Connection Pool (API vs Runners)
- **Mô tả**: Separate pool cho API requests (high priority) và background runners.
- **Logic**:
  ```go
  type DualPoolManager struct {
      apiPool    *sql.DB  // 70% of connections — for API requests
      runnerPool *sql.DB  // 30% of connections — for background runners
  }
  ```
- **Acceptance Criteria**:
  - AC-1: API requests never blocked by heavy runner operations
  - AC-2: Runners can still function when API pool near capacity
  - AC-3: Pool ratio configurable via `PG_API_POOL_RATIO` (default 0.7)

### FR-003: Connection Pool Prometheus Metrics
- **Mô tả**: Export pool utilization metrics.
- **Metrics**:
  ```
  bytebase_db_pool_active_connections{pool="api"}
  bytebase_db_pool_idle_connections{pool="api"}
  bytebase_db_pool_waiting_requests{pool="api"}
  bytebase_db_pool_max_connections{pool="api"}
  bytebase_db_pool_active_connections{pool="runner"}
  bytebase_db_pool_idle_connections{pool="runner"}
  bytebase_db_pool_connection_errors_total{pool="api",reason="timeout"}
  bytebase_db_pool_connection_wait_duration_seconds{pool="api"}
  ```
- **Acceptance Criteria**:
  - AC-1: All metrics available on `/metrics` endpoint
  - AC-2: Grafana dashboard template provided
  - AC-3: Alert rule: pool utilization > 80% for 5 minutes

### FR-004: Robust Reconnection
- **Mô tả**: Fix `reloadConnection()` race conditions.
- **Hiện tại**: `time.Sleep(100ms)` arbitrary delay
- **Sửa thành**:
  ```go
  func (m *DBConnectionManager) reloadConnection(ctx, filePath) {
      // Use file hash to detect complete write
      var newURL string
      for retries := 0; retries < 5; retries++ {
          url, err := readURLFromFile(filePath)
          if err != nil { continue }
          if url == "" { continue }
          time.Sleep(50 * time.Millisecond)
          url2, _ := readURLFromFile(filePath)
          if url == url2 {
              newURL = url  // File stable — write complete
              break
          }
      }
      // ...
      // Replace 1-hour force close with context-based drain
      drainCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
      defer cancel()
      go func() {
          <-drainCtx.Done()
          oldDB.Close()
      }()
  }
  ```
- **Acceptance Criteria**:
  - AC-1: No arbitrary `time.Sleep(100ms)` — use file stability check
  - AC-2: Drain timeout configurable (default 5 minutes, not 1 hour)
  - AC-3: Reconnection event logged with old/new URL (masked)
  - AC-4: Reconnection metric: `bytebase_db_reconnections_total`

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                 | Thay đổi                                |
|------------------------|--------------------------------------|------------------------------------------|
| Pool Manager           | `backend/store/db_connection.go`     | Dual pool, configurable sizing           |
| Pool Metrics           | `backend/metrics/pool_metrics.go`    | Prometheus pool collectors               |
| Store Constructor      | `backend/store/store.go`             | Accept pool config, wire dual pools      |
| Server Init            | `backend/server/server.go`           | Pass pool config from profile            |
| Config                 | `backend/component/config/profile.go`| Add pool config fields                   |
| Health Check           | `backend/api/v1/actuator_service.go` | Report pool health                       |

### 3.2 Configuration

| Environment Variable     | Default | Mô tả                                   |
|--------------------------|---------|------------------------------------------|
| `PG_MAX_CONNECTIONS`     | `0`     | 0 = auto-detect from PG                 |
| `PG_API_POOL_RATIO`      | `0.7`   | Fraction of pool allocated to API       |
| `PG_POOL_DRAIN_TIMEOUT`  | `300`   | Seconds to drain old pool on reconnect  |

### 3.3 Không có Database Changes

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                    |
|------------|----------------------------------------------------------|-------------------------------------|
| TC-001     | PG_MAX_CONNECTIONS=0 → auto-detect                     | Pool = 70% of PG max_connections   |
| TC-002     | PG_MAX_CONNECTIONS=100 → manual override               | Pool = 100                          |
| TC-003     | Pool exhaustion on API pool                              | Clear error, runner pool unaffected |
| TC-004     | Pool exhaustion on runner pool                           | API pool unaffected                 |
| TC-005     | Prometheus /metrics has pool metrics                    | All 8 metrics present               |
| TC-006     | File-based PG_URL changes mid-runtime                   | Reconnect without arbitrary sleep   |
| TC-007     | Reconnection drains old pool within timeout              | Old connections closed by timeout   |
| TC-008     | Pool utilization alert triggers at 80%                   | Alert fires                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                                           | Timeline     |
|---------|---------------------------------------------------|--------------|
| Phase 1 | Configurable pool size + auto-detect             | Sprint 1     |
| Phase 2 | Dual pool (API vs runner)                        | Sprint 2     |
| Phase 3 | Pool Prometheus metrics + Grafana dashboard      | Sprint 2     |
| Phase 4 | Reconnection hardening                           | Sprint 3     |
| Phase 5 | Load testing + alert tuning                      | Sprint 3     |

---

## 6. Risks & Mitigations

| Risk                                       | Impact | Mitigation                                |
|--------------------------------------------|--------|-------------------------------------------|
| Dual pool complicates connection management| MEDIUM | Clear pool selection per call site        |
| Runner pool too small → background delays  | MEDIUM | Configurable ratio, monitor metrics       |
| Auto-detect reads PG config at startup only| LOW    | Log warning if PG max_connections changes |
| Metric collection overhead                 | LOW    | Lightweight sql.DB.Stats() polling        |
