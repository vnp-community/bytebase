# Change Request: Persistent Message Bus

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-LIM-002                                               |
| **Limitation ID**  | LIM-002                                                  |
| **Title**          | Persistent Message Bus with Durability Guarantees        |
| **Category**       | Reliability / Architecture                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Thay thế message bus dựa trên **buffered Go channels** bằng hệ thống message queue có **durability**, **dead-letter handling**, và **multi-consumer support**. Sử dụng **NATS JetStream** cho HA mode, giữ Go channels cho single-node.

### 1.2 Bối cảnh
Message bus hiện tại sử dụng Go channels (buffer 100-1000). Khi server crash, tất cả messages bị mất. Migration tasks có thể bị bỏ quên, approval flows bị stale, không có retry mechanism.

### 1.3 Mục tiêu
- Zero message loss cho critical paths (TaskRun, Approval, Rollout)
- Fan-out và load distribution cho multiple consumers
- Dead-letter queue cho failed message processing
- Channel metrics exported tới Prometheus
- Backward compatible — single-node vẫn dùng Go channels

---

## 2. Yêu cầu chức năng

### FR-001: Message Bus Interface Abstraction
- **Mô tả**: Abstract interface cho message bus, cho phép swap implementation.
- **Interface**: `MessageBus { Publish, Subscribe, Close }` + `Message { ID, Subject, Payload, Metadata, Timestamp, Attempt }`
- **Acceptance Criteria**:
  - AC-1: Interface support cả Go channel và NATS JetStream backends
  - AC-2: Existing runners không cần thay đổi business logic
  - AC-3: Message serialization sử dụng Protocol Buffers

### FR-002: NATS JetStream Backend (HA Mode)
- **Streams**: `BYTEBASE_TASKS` (task.run.*, 24h), `BYTEBASE_APPROVALS` (approval.*, 48h), `BYTEBASE_ROLLOUTS` (rollout.*, 24h), `BYTEBASE_EVENTS` (event.*, 72h)
- **Acceptance Criteria**:
  - AC-1: Messages survive server restart
  - AC-2: At-least-once delivery với idempotent consumers
  - AC-3: Consumer acknowledgment required — unacked messages redelivered
  - AC-4: Configurable max redelivery attempts (default: 5)

### FR-003: Dead-Letter Queue (DLQ)
- Messages failing after max retries moved to `BYTEBASE_DLQ` stream (retention 7 days).
- Admin API to inspect, replay, or purge DLQ messages.
- DLQ count exposed as Prometheus metric + alert threshold.

### FR-004: Channel Metrics & Observability
- Metrics: `messages_published`, `messages_consumed`, `messages_failed`, `messages_dlq`, `messages_pending`, `processing_duration`, `channel_fill_ratio`
- Grafana dashboard template provided.

### FR-005: Idempotent Consumer Framework
- Message deduplication via `message.ID` stored in PostgreSQL (`bus_message_dedup` table).
- Configurable dedup window (default 24h). Auto-cleanup expired records.

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|----------------------------------------------------|
| Bus Interface                | `backend/component/bus/bus.go`            | Extract interface, keep channel implementation     |
| Channel Bus Adapter          | `backend/component/bus/channel_bus.go`    | Existing Go channel logic + metrics wrapper        |
| NATS Bus Adapter             | `backend/component/bus/nats_bus.go`       | NATS JetStream implementation                      |
| DLQ Manager                  | `backend/component/bus/dlq.go`            | Dead-letter queue management                       |
| Idempotent Consumer          | `backend/component/bus/idempotent.go`     | Dedup wrapper for consumers                        |
| Bus Metrics                  | `backend/component/bus/metrics.go`        | Prometheus metrics collectors                      |
| Store — Dedup                | `backend/store/message_dedup.go`          | Dedup record CRUD                                  |
| Server Wire                  | `backend/server/server.go`               | Wire new bus into runners                          |

### 3.2 Configuration

| Environment Variable       | Default     | Mô tả                                      |
|----------------------------|-------------|---------------------------------------------|
| `NATS_URL`                 | _(empty)_   | NATS server URL (enables persistent bus)   |
| `BUS_MAX_RETRIES`         | `5`         | Max redelivery attempts before DLQ         |
| `BUS_DEDUP_WINDOW`        | `24h`       | Message deduplication window               |

### 3.3 Database Changes

```sql
CREATE TABLE IF NOT EXISTS bus_message_dedup (
    message_id   TEXT        PRIMARY KEY,
    subject      TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_bus_message_dedup_expires ON bus_message_dedup (expires_at);
```

---

## 4. Phụ thuộc

| Dependency                        | Mô tả                                                  |
|-----------------------------------|---------------------------------------------------------|
| NATS Server 2.10+ with JetStream | External dependency (HA only)                           |
| `github.com/nats-io/nats.go`     | Go NATS client library                                  |
| CR-LIM-001                        | Leader election có thể share NATS infrastructure        |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                        |
|------------|----------------------------------------------------------|----------------------------------------|
| TC-001     | Single-node: message publish → consume                  | Go channel bus, immediate delivery     |
| TC-002     | HA mode: publish → consume via NATS                     | NATS JetStream delivery                |
| TC-003     | Server crash, restart                                    | Pending messages redelivered           |
| TC-004     | Consumer failure → retry                                | Redelivered up to maxRetries           |
| TC-005     | Consumer failure exceeds maxRetries                      | Message moved to DLQ                   |
| TC-006     | Duplicate message published                              | Processed exactly once (dedup)         |
| TC-007     | NATS connection lost                                     | Reconnect + message buffering          |
| TC-008     | TaskRun message survives restart                         | Task continues after restart           |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Bus interface extraction + channel adapter       | Sprint 1       |
| Phase 2 | Metrics instrumentation                          | Sprint 1       |
| Phase 3 | NATS JetStream adapter                           | Sprint 2-3     |
| Phase 4 | DLQ + idempotent consumers                       | Sprint 3       |
| Phase 5 | Runner migration                                 | Sprint 4       |
| Phase 6 | Integration testing + chaos engineering          | Sprint 5       |

---

## 7. Risks & Mitigations

| Risk                                    | Impact | Mitigation                                          |
|-----------------------------------------|--------|------------------------------------------------------|
| NATS adds operational complexity        | MEDIUM | Optional — single-node keeps Go channels             |
| Message ordering requirements           | MEDIUM | Partition by entity ID for ordered delivery          |
| Dedup table growth                      | LOW    | TTL-based cleanup, index on expires_at               |
| NATS cluster availability               | HIGH   | 3-node cluster, embedded NATS option                 |
