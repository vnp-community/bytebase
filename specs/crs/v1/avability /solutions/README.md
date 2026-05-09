# Solutions — Availability & High Availability

> **Source**: Architecture Document + TDD + Source Code Analysis
> **Created**: 2026-05-08
> **Author**: VNP AI Ops Team

---

## Giải pháp tổng quan

Thư mục này chứa các giải pháp kỹ thuật chi tiết cho 6 Change Requests về Availability, được thiết kế dựa trên kiến trúc 10-layer hiện tại của Bytebase (Architecture §1) và tận dụng tối đa các cơ chế có sẵn trong codebase.

### Nguyên tắc thiết kế

1. **Leverage existing infrastructure** — Tận dụng PG Advisory Locks, LISTEN/NOTIFY, heartbeat runner, Echo server đã có
2. **Backward compatible** — Single-node mode giữ nguyên behavior, features mới chỉ active khi cấu hình HA/cluster
3. **Minimal external dependencies** — PostgreSQL là backbone, chỉ thêm Redis (optional) cho cache
4. **Layer-aligned changes** — Mỗi giải pháp map rõ vào 10-layer architecture, không phá vỡ dependency rules

---

## Danh sách Solutions

| Solution | CR | Title | Focus Layers |
|---|---|---|---|
| [SOL-AVAIL-001](./SOL-AVAIL-001-ha-clustering.md) | CR-AVAIL-001 | HA Active-Active Clustering | L2, L6, L8, L10 |
| [SOL-AVAIL-002](./SOL-AVAIL-002-failover-dr.md) | CR-AVAIL-002 | Automated Failover & DR | L5, L6, L8 |
| [SOL-AVAIL-003](./SOL-AVAIL-003-health-circuit-breaker.md) | CR-AVAIL-003 | Health Monitoring & Circuit Breaker | L2, L5, L8 |
| [SOL-AVAIL-004](./SOL-AVAIL-004-connection-resilience.md) | CR-AVAIL-004 | Database Connection Resilience | L7, L8 |
| [SOL-AVAIL-005](./SOL-AVAIL-005-backup-recovery.md) | CR-AVAIL-005 | Backup, Recovery & RPO/RTO | L6, L8, L10 |
| [SOL-AVAIL-006](./SOL-AVAIL-006-multi-region.md) | CR-AVAIL-006 | Multi-Region Geo-Redundancy | L2, L5, L8, L10 |

---

## Implementation Priority

```
Phase 1 (Sprint 1-2): Foundation
├── SOL-AVAIL-004 — Connection Resilience (standalone, no dependencies)
├── SOL-AVAIL-003 — Health Monitoring (standalone, prerequisite for others)
└── SOL-AVAIL-001 — HA Clustering (core infra)

Phase 2 (Sprint 3-4): Resilience
├── SOL-AVAIL-002 — Failover & DR (depends on 001, 003)
└── SOL-AVAIL-005 — Backup & Recovery (depends on 004)

Phase 3 (Sprint 5-7): Geographic
└── SOL-AVAIL-006 — Multi-Region (depends on all above)
```

---

## Codebase Anchor Points

Các giải pháp sẽ thay đổi/mở rộng các files sau (xem chi tiết trong từng SOL):

| Existing File | Modifications |
|---|---|
| `backend/server/server.go` | Cluster lifecycle, graceful shutdown |
| `backend/server/echo_routes.go` | /readyz, /healthz/deep endpoints |
| `backend/store/store.go` | Cache interface, pool config |
| `backend/store/advisory_lock.go` | Extended lock keys for runners |
| `backend/store/replica_heartbeat.go` | Enhanced heartbeat with metadata |
| `backend/runner/heartbeat/runner.go` | Cluster registration |
| `backend/component/bus/bus.go` | Circuit breaker integration |
| `backend/component/dbfactory/` | Retry wrapper for drivers |
