# Performance Solutions — Implementation Guide

> Giải pháp kỹ thuật cho 7 Change Requests hiệu năng, align với Architecture Document và TDD.

---

## Solutions Mapping

| Solution ID | CR Ref | Title | Arch Layers | Key Approach |
|-------------|--------|-------|-------------|-------------|
| SOL-PERF-001 | CR-PERF-001 | Metadata Store Scalability | L8, L10 | Workspace denormalization, composite indexes, pool scaling |
| SOL-PERF-002 | CR-PERF-002 | Cache Layer Scaling | L8 | Adaptive LRU sizing, compressed L2 schema cache |
| SOL-PERF-003 | CR-PERF-003 | Schema Sync Scalability | L6, L5, L8 | Instance-based pagination, checksum skip, adaptive pool |
| SOL-PERF-004 | CR-PERF-004 | Multi-Tenant Isolation | L3, L5, L8 | Rate limiter + quota interceptors, per-tenant metrics |
| SOL-PERF-005 | CR-PERF-005 | API Batch Optimization | L4, L8 | True batch SQL, BASIC view, batch IAM, cursor pagination |
| SOL-PERF-006 | CR-PERF-006 | Runner Orchestration | L5, L6, L8 | PG job queue + SKIP LOCKED, resource-isolated workers |
| SOL-PERF-007 | CR-PERF-007 | Frontend Performance | L1 | Virtual scrolling, Web Worker layout, server-side search |

---

## Architecture Alignment

Tất cả solutions được thiết kế dựa trên:

1. **Architecture Document** — 10-layer modular monolith, dependency direction rules
2. **TDD** — Store patterns (pgx/v5, qb builder), Bus (Go channels), Interceptor chain, Plugin boundaries

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Denormalize > Partition** | Partitioning rủi ro migration cao, denormalization đạt 80% hiệu quả với 20% effort |
| **Keep Bus channels** | Low-latency signaling vẫn cần thiết; Job Queue bổ sung cho durability |
| **Adaptive > Static** | Cache/pool/concurrency tự scale dựa trên runtime metrics |
| **Interceptor chain** | Rate limit + quota tích hợp vào existing chain (TDD §3.1) |
| **React-first frontend** | Vue code đang migrated (TDD §10.1), solutions focus React layer |
| **hashicorp/golang-lru** | Giữ nguyên library đã proven; chỉ thêm compressed wrapper |

---

## Implementation Order

```
Sprint 1-2:
  SOL-PERF-001 (migration + indexes)     ← Foundation
  SOL-PERF-002 (adaptive cache)          ← Immediate perf gain

Sprint 2-3:
  SOL-PERF-005 (batch API + cursor)      ← Independent, high ROI
  SOL-PERF-003 (schema sync)             ← Depends on SOL-001 workspace column

Sprint 3-4:
  SOL-PERF-004 (tenant isolation)        ← Depends on SOL-001 + SOL-002
  SOL-PERF-006 (job queue)               ← Depends on SOL-003

Sprint 4-5:
  SOL-PERF-007 (frontend)                ← Depends on SOL-005 (BASIC view API)
  Integration Testing + Load Benchmarks
```

---

## New Files Summary

| File | Solution | Type |
|------|----------|------|
| `backend/migrator/migration/*/0001_db_workspace_denorm.sql` | SOL-001 | Migration |
| `backend/migrator/migration/*/0003_job_queue.sql` | SOL-006 | Migration |
| `backend/store/cache_compressed.go` | SOL-002 | New component |
| `backend/store/cache_metrics.go` | SOL-002 | New metrics |
| `backend/store/cache_warmer.go` | SOL-002 | New component |
| `backend/runner/schemasync/checksum.go` | SOL-003 | New utility |
| `backend/runner/schemasync/adaptive_pool.go` | SOL-003 | New utility |
| `backend/runner/schemasync/metrics.go` | SOL-003 | New metrics |
| `backend/component/quota/manager.go` | SOL-004 | New component |
| `backend/component/ratelimit/limiter.go` | SOL-004 | New component |
| `backend/api/v1/ratelimit_interceptor.go` | SOL-004 | New interceptor |
| `backend/metrics/tenant_metrics.go` | SOL-004 | New metrics |
| `backend/component/jobqueue/queue.go` | SOL-006 | New component |
| `backend/component/jobqueue/worker.go` | SOL-006 | New component |
| `frontend/src/react/components/DatabaseList/VirtualDatabaseList.tsx` | SOL-007 | New UI |
| `frontend/src/react/hooks/useDatabaseSearch.ts` | SOL-007 | New hook |
| `frontend/src/react/workers/schemaLayout.worker.ts` | SOL-007 | New Worker |

## Modified Files Summary

| File | Solutions | Changes |
|------|-----------|---------|
| `backend/store/store.go` | SOL-001, SOL-002 | Adaptive cache sizing, L2 cache field |
| `backend/store/database.go` | SOL-001, SOL-005 | Remove JOIN, view-based columns, cursor, batch SQL |
| `backend/store/db_connection.go` | SOL-001 | Dynamic pool sizing |
| `backend/store/db_schema.go` | SOL-002 | Tiered cache lookup (L1→L2→DB) |
| `backend/runner/schemasync/syncer.go` | SOL-003 | Instance-based pagination, checksum, metrics |
| `backend/server/grpc_routes.go` | SOL-004 | Add rate limit interceptor to chain |
| `backend/api/v1/database_service.go` | SOL-005 | True batch update, batch IAM check |
| `backend/runner/plancheck/scheduler.go` | SOL-006 | Enqueue to job queue |
| `frontend/src/react/stores/databaseStore.ts` | SOL-007 | Paginated state |

---

> **Generated**: 2026-05-08 — Aligned with Architecture v1 and TDD v1 specifications.
