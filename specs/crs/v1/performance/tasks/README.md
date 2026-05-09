# Performance Tasks — Token-Optimized Breakdown

> 28 tác vụ chia nhỏ từ 7 Solutions, tối ưu cho agent execution với chi phí token thấp nhất.

## Nguyên tắc tối ưu token

1. **Self-contained**: Mỗi task chỉ cần đọc 1-2 file source, không cần context từ task khác
2. **Single-file output**: Mỗi task tạo hoặc sửa tối đa 1-2 files
3. **Exact line refs**: Chỉ rõ line numbers cần sửa, giảm scan toàn file
4. **No cross-dependency within sprint**: Tasks trong cùng sprint có thể chạy song song

## Task Registry

### Sprint 1 — Foundation (SOL-001 + SOL-002)

| Task ID | Solution | Title | Files | Type | Est. Tokens |
|---------|----------|-------|-------|------|-------------|
| T-001 | SOL-001 | Migration: workspace denorm | 1 new | SQL | ~800 |
| T-002 | SOL-001 | Migration: effective_environment | 1 new | SQL | ~600 |
| T-003 | SOL-001 | Migration: sync triggers | 1 new | SQL | ~700 |
| T-004 | SOL-001 | Store: remove JOIN in ListDatabases | 1 edit | Go | ~1200 |
| T-005 | SOL-001 | Store: update CreateDatabaseDefault | 1 edit | Go | ~800 |
| T-006 | SOL-001 | Store: dynamic pool sizing | 1 edit | Go | ~900 |
| T-007 | SOL-001 | Metrics: pool metrics | 1 new | Go | ~500 |
| T-008 | SOL-002 | Store: adaptive cache sizing | 1 edit | Go | ~1000 |
| T-009 | SOL-002 | Store: CompressedSchemaCache | 1 new | Go | ~900 |
| T-010 | SOL-002 | Store: tiered cache lookup | 1 edit | Go | ~700 |
| T-011 | SOL-002 | Metrics: cache metrics | 1 new | Go | ~500 |
| T-012 | SOL-002 | Store: cache warmer | 1 new | Go | ~600 |

### Sprint 2 — API + Sync (SOL-005 + SOL-003)

| Task ID | Solution | Title | Files | Type | Est. Tokens |
|---------|----------|-------|-------|------|-------------|
| T-013 | SOL-005 | Proto: DatabaseView + cursor fields | 1 edit | Proto | ~600 |
| T-014 | SOL-005 | Store: view-based ListDatabases | 1 edit | Go | ~1000 |
| T-015 | SOL-005 | Store: cursor-based pagination | 1 edit | Go | ~700 |
| T-016 | SOL-005 | Store: BatchUpdateDatabases SQL | 1 new func | Go | ~1000 |
| T-017 | SOL-005 | Service: batch IAM check | 1 edit | Go | ~700 |
| T-018 | SOL-003 | Syncer: instance-based pagination | 1 edit | Go | ~1200 |
| T-019 | SOL-003 | Syncer: checksum skip | 1 new | Go | ~1000 |
| T-020 | SOL-003 | Syncer: adaptive concurrency | 1 new | Go | ~600 |
| T-021 | SOL-003 | Metrics: sync metrics | 1 new | Go | ~500 |

### Sprint 3 — Isolation + Queue (SOL-004 + SOL-006)

| Task ID | Solution | Title | Files | Type | Est. Tokens |
|---------|----------|-------|-------|------|-------------|
| T-022 | SOL-004 | Component: rate limiter | 1 new | Go | ~700 |
| T-023 | SOL-004 | Component: quota manager | 1 new | Go | ~1000 |
| T-024 | SOL-004 | Interceptor: rate limit | 1 new + 1 edit | Go | ~900 |
| T-025 | SOL-004 | Metrics: tenant metrics | 1 new | Go | ~500 |
| T-026 | SOL-006 | Migration: job_queue table | 1 new | SQL | ~800 |
| T-027 | SOL-006 | Component: job queue manager | 1 new | Go | ~1200 |
| T-028 | SOL-006 | Component: worker pool | 1 new | Go | ~700 |

### Sprint 4 — Frontend (SOL-007) — deferred

> Frontend tasks depend on SOL-005 API being deployed first.

## Dependency Graph

```
T-001 ──┬── T-004 (remove JOIN needs workspace column)
T-002 ──┤   T-005 (create needs workspace column)
T-003 ──┘

T-006, T-007 (independent)

T-008 ──── T-010 (tiered lookup needs L2 cache from T-009)
T-009 ──┘
T-011, T-012 (independent)

T-013 ──── T-014, T-015 (store needs proto types)
T-016, T-017 (independent)

T-018 (needs T-004 done — workspace filter in ListDatabases)
T-019, T-020, T-021 (independent)

T-022 ──── T-024 (interceptor needs limiter)
T-023 (independent)
T-025 (independent)

T-026 ──── T-027 ──── T-028 (chain dependency)
```

## Estimated Total Token Cost

| Sprint | Tasks | Est. Tokens | Parallel? |
|--------|-------|-------------|-----------|
| Sprint 1 | 12 | ~8,200 | Yes (3 groups) |
| Sprint 2 | 9 | ~7,300 | Yes (2 groups) |
| Sprint 3 | 7 | ~5,800 | Yes (2 groups) |
| **Total** | **28** | **~21,300** | — |
