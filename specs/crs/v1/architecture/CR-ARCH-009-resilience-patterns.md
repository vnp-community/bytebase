# Change Request: Resilience Patterns Infrastructure

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-009                                              |
| **Source ID**      | ARCH-WEAK-004                                            |
| **Title**          | Resilience Patterns — Circuit Breaker, Bulkhead, Rate Limiter |
| **Category**       | Architecture (Reliability)                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01 (Change Workflow), DCM-09 (Batch Changes), ADM-02 (IM Notifications) |

---

## 1. Tổng quan

### 1.1 Mô tả
Implement resilience patterns cơ bản: **circuit breaker** (external dependencies), **bulkhead** (concurrency isolation), **rate limiter** (API protection), **structured retry** (exponential backoff). Hiện tại 0 circuit breakers, 5 timeouts trong entire runner layer.

### 1.2 Bối cảnh
- 0 circuit breakers trong toàn codebase
- DB reconnection: fixed 100ms delay, no backoff, no retry
- Schema sync: syncs ALL instances đồng thời, no concurrency limit
- Webhook delivery: no circuit breaker, retries on every event to downed services
- Only 5 `context.WithTimeout` usages across 8 runners

### 1.3 Mục tiêu
- Circuit breaker cho external dependencies (webhook, DB connections)
- Bulkhead pattern: concurrency limit cho schema sync, task execution
- API rate limiter: per-user, per-endpoint
- Structured retry: exponential backoff cho DB reconnection
- All patterns observable via Prometheus metrics

---

## 2. Yêu cầu chức năng

### FR-001: Circuit Breaker Library
- **Mô tả**: Implement circuit breaker cho external dependency calls.
- **Logic**:
  ```go
  // resilience/circuit_breaker.go
  type CircuitBreaker struct {
      name           string
      maxFailures    int           // e.g., 5
      resetTimeout   time.Duration // e.g., 30s
      state          State         // Closed → Open → HalfOpen
  }
  func (cb *CircuitBreaker) Execute(fn func() error) error
  ```
- **Application Points**:

  | Component | Dependency | Threshold | Reset |
  |-----------|-----------|-----------|-------|
  | Webhook Manager | Slack/DingTalk/Teams | 5 failures | 30s |
  | Schema Sync | Remote DB instances | 3 failures | 60s |
  | DB Reconnect | PostgreSQL metadata | 5 failures | 10s |

- **Acceptance Criteria**:
  - AC-1: Circuit opens after N consecutive failures
  - AC-2: Half-open: tries single request after reset timeout
  - AC-3: Success in half-open → circuit closes
  - AC-4: Metric: `bytebase_circuit_breaker_state{name, state}`

### FR-002: Bulkhead (Concurrency Limiter)
- **Mô tả**: Semaphore-based concurrency limiter cho resource-intensive operations.
- **Logic**:
  ```go
  // resilience/bulkhead.go
  type Bulkhead struct {
      sem chan struct{}  // bounded semaphore
  }
  func NewBulkhead(maxConcurrent int) *Bulkhead
  func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error
  ```
- **Application Points**:

  | Component | Current Concurrency | Limit |
  |-----------|-------------------|-------|
  | Schema Sync | Unlimited (all instances) | 10 concurrent |
  | Task Execution | Unlimited | 5 concurrent per runner |
  | Data Export | Unlimited | 3 concurrent |

- **Acceptance Criteria**:
  - AC-1: Schema sync processes max 10 instances concurrently
  - AC-2: Excess requests queue, not fail
  - AC-3: Metric: `bytebase_bulkhead_active{name}`, `bytebase_bulkhead_queued{name}`

### FR-003: API Rate Limiter
- **Mô tả**: Per-user, per-endpoint rate limiting cho API layer.
- **Logic**:
  ```go
  // Interceptor: rate limit before ACL check
  // Token bucket algorithm: refill rate per second
  ```
- **Default Limits**:

  | Endpoint Category | Rate | Burst |
  |-------------------|------|-------|
  | Auth (login/signup) | 5/min | 10 |
  | SQL Query | 100/min | 200 |
  | Schema Change | 30/min | 50 |
  | Admin API | 60/min | 100 |

- **Acceptance Criteria**:
  - AC-1: Rate exceeded → HTTP 429 with Retry-After header
  - AC-2: Per-user tracking (by user ID from auth context)
  - AC-3: Rate limits configurable via setting

### FR-004: Structured Retry with Backoff
- **Mô tả**: Replace fixed delays với exponential backoff + jitter.
- **Logic**:
  ```go
  // resilience/retry.go
  type RetryConfig struct {
      MaxRetries   int
      InitialDelay time.Duration
      MaxDelay     time.Duration
      Multiplier   float64
      Jitter       bool
  }
  func Retry(ctx context.Context, config RetryConfig, fn func() error) error
  ```
- **Application Points**:
  - DB reconnection: 100ms → 200ms → 400ms → ... → 30s max
  - Webhook delivery: 1s → 2s → 4s → ... → 60s max
- **Acceptance Criteria**:
  - AC-1: Exponential backoff with jitter
  - AC-2: Context cancellation stops retries immediately
  - AC-3: Metric: `bytebase_retry_total{operation, attempt}`

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Resilience library     | `backend/common/resilience/`          | CircuitBreaker, Bulkhead, Retry, RateLimiter |
| Webhook integration    | `backend/component/webhook/manager.go`| Wrap calls with circuit breaker              |
| Schema sync            | `backend/runner/schemasync/syncer.go` | Add bulkhead (10 concurrent)                 |
| Task runner            | `backend/runner/taskrun/scheduler.go` | Add bulkhead (5 concurrent)                  |
| DB reconnection        | `backend/store/db_connection.go`      | Replace fixed delay with retry               |
| API rate limiter       | `backend/api/v1/rate_limiter.go`      | New interceptor                              |
| Metrics                | `backend/common/resilience/metrics.go`| Prometheus integration                       |

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Webhook fails 5 times → circuit opens → stops calling        | Circuit breaker activates                |
| TC-002     | Circuit open 30s → half-open → success → closes              | Recovery works                           |
| TC-003     | Schema sync with bulkhead: 20 instances, limit 10           | Max 10 concurrent syncs                 |
| TC-004     | API rate limit: 6 login attempts in 1 min → 429             | Rate limiting active                     |
| TC-005     | DB reconnect: exponential backoff observed in logs           | Increasing delays                        |
| TC-006     | Context cancelled during retry → retry stops immediately    | Clean cancellation                       |
| TC-007     | Prometheus metrics show circuit breaker state transitions    | Observable state changes                 |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Resilience library (circuit breaker, retry, bulkhead) | Sprint 1  |
| Phase 2 | Webhook circuit breaker + schema sync bulkhead     | Sprint 2     |
| Phase 3 | DB reconnection retry + task runner bulkhead       | Sprint 2     |
| Phase 4 | API rate limiter interceptor                       | Sprint 3     |
| Phase 5 | Prometheus metrics + Grafana dashboards            | Sprint 3     |
| Phase 6 | Load testing: cascading failure prevention         | Sprint 4     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Circuit breaker opens prematurely              | MEDIUM | Conservative thresholds (5 failures, not 3)          |
| Rate limiter blocks legitimate burst traffic  | MEDIUM | High burst limits, configurable per endpoint         |
| Bulkhead queuing adds latency                 | LOW    | Queue with timeout, not indefinite wait              |
| Retry backoff delays legitimate operations    | LOW    | Max delay capped (30s DB, 60s webhook)               |
