# Change Request: Durable Message Bus with Observability

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-002                                              |
| **Source ID**      | ARCH-LIM-002                                             |
| **Title**          | Durable Message Bus — PG-Backed Queue with Observability |
| **Category**       | Architecture (Reliability + HA)                          |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01 (Change Workflow), DCM-09 (Batch Changes), SEC-09 (Approval Workflow) |

---

## 1. Tổng quan

### 1.1 Mô tả
Thay thế volatile Go channel bus bằng **PG-backed persistent queue** với observability metrics. Đảm bảo message durability khi server crash, backpressure control, và HA-safe coordination.

### 1.2 Bối cảnh
- Message Bus hiện tại dùng buffered Go channels (4,100 buffer total)
- Messages mất khi server crash → tasks stuck PENDING
- Channel buffer full → goroutine blocked → API starvation
- HA mode: 2 replicas có independent Bus → duplicate processing risk
- Không có dead-letter queue hay retry mechanism

### 1.3 Mục tiêu
- Zero message loss trên server restart
- Backpressure signaling khi queue depth > threshold
- HA-safe: single consumer per message
- Prometheus metrics cho queue depth, dequeue latency
- Backward compatible — Bus interface không thay đổi

---

## 2. Yêu cầu chức năng

### FR-001: PG Queue Table
- **Mô tả**: Tạo `bus_queue` table trong metadata database cho persistent messages.
- **Logic**:
  ```sql
  CREATE TABLE bus_queue (
      id          BIGSERIAL PRIMARY KEY,
      channel     TEXT NOT NULL,          -- 'approval_check', 'task_run_tickle', etc.
      payload     JSONB NOT NULL,         -- serialized message
      status      TEXT NOT NULL DEFAULT 'pending',  -- pending/processing/done/failed
      created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      claimed_by  TEXT,                    -- server instance ID
      claimed_at  TIMESTAMPTZ,
      attempts    INT NOT NULL DEFAULT 0,
      max_retries INT NOT NULL DEFAULT 3
  );
  CREATE INDEX idx_bus_queue_channel_status ON bus_queue (channel, status) WHERE status = 'pending';
  ```
- **Acceptance Criteria**:
  - AC-1: Messages persist qua server restart
  - AC-2: `SELECT ... FOR UPDATE SKIP LOCKED` cho HA consumer safety
  - AC-3: Messages tự chuyển `processing → failed` sau 5 phút timeout

### FR-002: Bus Interface Compatibility
- **Mô tả**: Giữ nguyên Bus API nhưng backend chuyển sang PG queue.
- **Logic**:
  ```go
  type Bus struct {
      db          *sql.DB
      instanceID  string
      // Channels kept for backward compatibility (consumers đọc từ channels)
      ApprovalCheckChan       chan IssueRef
      // ...
      // Internal: goroutine polls PG queue → pushes to channels
  }
  func (b *Bus) Enqueue(ctx context.Context, channel string, payload any) error
  func (b *Bus) startConsumers(ctx context.Context)  // poll PG → push channel
  ```
- **Acceptance Criteria**:
  - AC-1: Existing runner code vẫn đọc từ channels (zero migration)
  - AC-2: `Enqueue()` ghi vào PG thay vì channel trực tiếp
  - AC-3: Consumer poll interval ≤ 100ms cho low latency

### FR-003: Backpressure & Dead-Letter Queue
- **Mô tả**: Implement backpressure signaling và DLQ cho failed messages.
- **Acceptance Criteria**:
  - AC-1: Khi queue depth > 5,000: log warning, metric alert
  - AC-2: Messages failed 3 lần → chuyển sang `failed` status
  - AC-3: Admin API endpoint để retry/purge failed messages

### FR-004: Bus Observability
- **Mô tả**: Prometheus metrics cho bus operations.
- **Metrics**:
  - `bytebase_bus_queue_depth{channel}` — pending messages per channel
  - `bytebase_bus_dequeue_duration_seconds{channel}` — processing latency
  - `bytebase_bus_enqueue_total{channel}` — total enqueued
  - `bytebase_bus_failed_total{channel}` — failed message count

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Bus redesign           | `backend/component/bus/bus.go`        | Add PG queue backend, keep channel interface |
| Migration              | `backend/migrator/migration/`         | `bus_queue` table DDL                        |
| Bus metrics            | `backend/component/bus/metrics.go`    | Prometheus counters & gauges                 |
| NOTIFY integration     | `backend/runner/notifylistener/`      | PG NOTIFY triggers immediate queue poll      |

### 3.2 Database Changes
- New table: `bus_queue`
- New index: `idx_bus_queue_channel_status`

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Enqueue message → restart server → message still pending     | Message recovered after restart          |
| TC-002     | 2 replicas consume queue → no duplicate processing           | SKIP LOCKED prevents double-pick         |
| TC-003     | Message fails 3 times → moved to failed status              | DLQ working                              |
| TC-004     | Queue depth > 5000 → Prometheus alert fires                 | Backpressure observable                  |
| TC-005     | Enqueue → consumer receives within 100ms                    | Low latency maintained                   |
| TC-006     | Existing runner code unchanged → still works                | Backward compatible                      |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | PG queue table + Bus backend implementation        | Sprint 1-2   |
| Phase 2 | Consumer polling + channel bridge                  | Sprint 2     |
| Phase 3 | Feature flag: `BUS_PERSISTENT_ENABLED=true`        | Sprint 3     |
| Phase 4 | Metrics + monitoring dashboard                     | Sprint 3     |
| Phase 5 | HA testing: 2-replica queue consumption            | Sprint 4     |
| Phase 6 | Remove in-memory fallback (feature flag → default) | Sprint 5     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| PG queue adds latency vs in-memory channels   | MEDIUM | NOTIFY trigger + 100ms poll = near-realtime         |
| Queue table growth (unbounded)                | LOW    | DataCleaner runner auto-purge done/failed > 7 days  |
| Migration risk on existing databases          | LOW    | Additive DDL only, no schema modification            |
| Consumer crash leaves messages in `processing`| MEDIUM | 5-minute timeout auto-reset to `pending`             |
