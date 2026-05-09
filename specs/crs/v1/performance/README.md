# Performance Change Requests — Bytebase 200K+ Database Scale

> **Target**: Quản lý >200,000 databases, >100 bank tenants trên shared Bytebase deployment

---

## Tổng quan

Bộ Change Requests này giải quyết các bottleneck hiệu năng được xác định qua phân tích source code Bytebase, nhắm tới quy mô **200K+ databases** và **multi-tenancy cho 100+ ngân hàng**.

## Danh sách CR

| CR ID       | Title                                            | Priority | Category               |
|-------------|--------------------------------------------------|----------|------------------------|
| CR-PERF-001 | Metadata Store — PG Partitioning & Index Opt     | P0       | Database Scalability   |
| CR-PERF-002 | Cache Layer Scaling — Adaptive & Tiered          | P0       | Caching                |
| CR-PERF-003 | Schema Sync — Incremental & Adaptive Concurrency | P0       | Background Processing  |
| CR-PERF-004 | Multi-Tenant Isolation for 100+ Banks            | P0       | Multi-Tenancy          |
| CR-PERF-005 | API Batch Ops, Lazy Loading & Streaming          | P1       | API Performance        |
| CR-PERF-006 | Runner Orchestration — Job Queue & Distribution  | P1       | Background Processing  |
| CR-PERF-007 | Frontend Virtualization & Progressive Loading    | P1       | Frontend Performance   |

## Bottleneck Analysis Summary

### Identified Issues (from source code)

| Issue | Source File | Impact |
|-------|-----------|--------|
| `databaseCache` hardcoded 32K entries | `store/store.go:52` | <16% coverage at 200K |
| `dbSchemaCache` only 128 entries | `store/store.go:76` | <0.1% coverage at 200K |
| Schema syncer loads ALL databases | `schemasync/syncer.go:209` | OOM at 200K |
| `MaximumOutstanding = 100` fixed | `schemasync/syncer.go:36` | Not adaptive |
| `COALESCE()` in WHERE not sargable | `store/database.go:146` | Full scan |
| `BatchUpdateDatabases` loops N times | `database_service.go:423` | N queries |
| No workspace column on `db` table | `store/database.go:137` | JOIN required |
| No per-tenant resource quotas | Architecture-level | Noisy neighbor |

## Dependency Graph

```
CR-PERF-001 (PG Partitioning)
    ├── CR-PERF-002 (Cache Scaling) — depends on partition-aware cache keys
    └── CR-PERF-003 (Schema Sync) — depends on paginated queries

CR-PERF-004 (Multi-Tenant)
    ├── CR-PERF-001 — depends on workspace denormalization
    ├── CR-PERF-002 — depends on tenant-aware cache
    └── CR-PERF-006 — depends on tenant-fair scheduling

CR-PERF-005 (API Batch) — independent
CR-PERF-006 (Runner Orchestration) — depends on CR-PERF-003
CR-PERF-007 (Frontend) — depends on CR-PERF-005 (streaming API)
```

## Recommended Execution Order

1. **Sprint 1-2**: CR-PERF-001 (PG infrastructure) + CR-PERF-002 (Cache)
2. **Sprint 2-3**: CR-PERF-003 (Schema Sync) + CR-PERF-005 (API)
3. **Sprint 3-4**: CR-PERF-004 (Multi-Tenant) + CR-PERF-006 (Runner)
4. **Sprint 4-5**: CR-PERF-007 (Frontend) + Integration Testing

## Success Criteria

| Metric | Target |
|--------|--------|
| Total databases managed | >200,000 |
| Concurrent tenants | >100 banks |
| ListDatabases P99 | ≤ 50ms |
| Schema sync cycle (200K) | ≤ 10 min |
| API batch update (1K DBs) | ≤ 500ms |
| Frontend initial load | ≤ 2s |
| Memory usage (server) | ≤ 4GB |
| Cross-tenant impact | ≤ 5% degradation |

---

> **Document generated**: 2026-05-08 — Based on Bytebase source code analysis (store, syncer, API layers)
