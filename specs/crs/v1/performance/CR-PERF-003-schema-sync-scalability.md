# Change Request: Schema Sync Scalability for 200K+ Databases

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-003                                              |
| **Title**          | Schema Sync Runner — Incremental Sync & Adaptive Concurrency |
| **Category**       | Performance / Background Processing                      |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-04, DCM-12, ADM-01                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Schema syncer hiện load toàn bộ databases vào memory mỗi 15 phút (`trySyncAll`), hardcode `MaximumOutstanding = 100` goroutines. Với 200K databases, mỗi sync cycle tiêu tốn ~5GB memory và mất >30 phút, gây resource starvation.

### 1.2 Bối cảnh (từ `backend/runner/schemasync/syncer.go`)
- Line 209: `databases, err := s.store.ListDatabases(ctx, &store.FindDatabaseMessage{})` — load ALL databases
- Line 36: `MaximumOutstanding = 100` — fixed, không adaptive
- Line 31: `instanceSyncInterval = 15 * time.Minute` — sync ALL instances mỗi 15 phút
- Mỗi `SyncDatabaseSchema` mở connection tới target DB, dump schema, serialize protobuf
- Không có priority queue — database mới thay đổi cùng priority với database idle 6 tháng

### 1.3 Mục tiêu
- Sync 200K databases trong <10 phút per cycle
- Memory usage <500MB cho sync runner
- Priority-based: recently changed databases sync first
- Adaptive concurrency dựa trên system load

---

## 2. Yêu cầu chức năng

### FR-001: Paginated Database Loading
- **Mô tả**: Thay thế full load bằng paginated cursor-based loading
- **Logic**:
  ```go
  func (s *Syncer) trySyncAll(ctx context.Context) {
      cursor := ""
      for {
          databases, nextCursor := s.store.ListDatabasesPaginated(ctx, cursor, 1000)
          for _, db := range databases {
              if shouldSync(db) {
                  s.databaseSyncMap.Store(db.String(), db)
              }
          }
          if nextCursor == "" { break }
          cursor = nextCursor
      }
  }
  ```
- **AC**:
  - AC-1: Memory usage ≤ 50MB per page (1000 databases)
  - AC-2: Cursor-based — no OFFSET, no duplicate/missing
  - AC-3: Cancellation safe — stops cleanly on context cancel

### FR-002: Priority-Based Sync Queue
- **Mô tả**: Replace `sync.Map` với priority queue based on staleness
- **Priority calculation**:
  | Factor                    | Weight | Mô tả                            |
  |---------------------------|--------|-----------------------------------|
  | Time since last sync      | 40%    | Longer = higher priority          |
  | Recent change activity    | 30%    | Issue/plan/rollout activity       |
  | Environment tier          | 20%    | Production > staging > dev        |
  | Manual sync request       | 10%    | User-triggered = highest          |
- **AC**:
  - AC-1: Production databases sync trước staging/dev
  - AC-2: Recently changed databases sync within 2 minutes
  - AC-3: Manual sync request preempts auto-sync queue

### FR-003: Adaptive Concurrency Control
- **Mô tả**: Dynamic goroutine pool size dựa trên CPU/memory/connection availability
- **Logic**:
  ```go
  func (s *Syncer) calculateConcurrency() int {
      cpuLoad := runtime.NumCPU() - currentCPULoad()
      connAvailable := poolSize - activeConnections()
      memAvailable := freeMemoryMB() / 50  // ~50MB per sync task
      return max(10, min(cpuLoad*5, connAvailable/2, memAvailable))
  }
  ```
- **AC**:
  - AC-1: Concurrency giảm khi CPU >80% utilization
  - AC-2: Concurrency giảm khi connection pool >70% utilized
  - AC-3: Min 10, max 500 goroutines

### FR-004: Incremental Schema Sync
- **Mô tả**: Only sync databases whose schema has actually changed (via checksum)
- **Logic**: Store schema checksum in `db.metadata`, compare before full sync
- **AC**:
  - AC-1: Skip sync nếu remote checksum matches stored checksum
  - AC-2: Checksum comparison <5ms per database
  - AC-3: Forced full sync mỗi 24h regardless of checksum

### FR-005: Tenant-Partitioned Sync Scheduling
- **Mô tả**: Stagger sync cycles per tenant để avoid thundering herd
- **Logic**: Tenant sync offset = hash(workspace) % sync_interval
- **AC**:
  - AC-1: No two tenants start full sync simultaneously
  - AC-2: Each tenant completes full sync within sync_interval
  - AC-3: Per-tenant sync progress tracking

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component          | File                                     | Thay đổi                              |
|--------------------|------------------------------------------|----------------------------------------|
| Syncer             | `backend/runner/schemasync/syncer.go`    | Paginated loading, priority queue      |
| Priority Queue     | `backend/runner/schemasync/priority.go`  | New: weighted priority queue           |
| Concurrency Ctrl   | `backend/runner/schemasync/adaptive.go`  | New: adaptive pool sizing              |
| Store — Database   | `backend/store/database.go`              | ListDatabasesPaginated (cursor-based)  |
| Checksum           | `backend/runner/schemasync/checksum.go`  | New: schema checksum comparison        |
| Metrics            | `backend/metrics/sync_metrics.go`        | Sync latency, queue depth, skip ratio  |

### 3.2 Configuration

| Environment Variable        | Default  | Mô tả                              |
|-----------------------------|----------|--------------------------------------|
| `SYNC_MAX_CONCURRENCY`      | `500`    | Max sync goroutines                  |
| `SYNC_MIN_CONCURRENCY`      | `10`     | Min sync goroutines                  |
| `SYNC_PAGE_SIZE`            | `1000`   | Databases per page                   |
| `SYNC_CHECKSUM_ENABLED`     | `true`   | Enable incremental sync              |
| `SYNC_FULL_INTERVAL_HOURS`  | `24`     | Force full sync interval             |
| `SYNC_STAGGER_ENABLED`      | `true`   | Enable tenant sync staggering        |

---

## 4. Performance Targets

| Metric                      | Current (10K DBs) | Target (200K+ DBs) |
|-----------------------------|--------------------|--------------------|
| Sync cycle time             | ~2 min             | ≤ 10 min          |
| Memory during sync          | ~500MB             | ≤ 500MB           |
| Databases synced/sec        | ~80                | ~350              |
| Skip ratio (incremental)    | 0% (full sync)     | ≥ 70%             |
| Priority response (changed) | 15 min (next cycle)| ≤ 2 min           |

---

## 5. Test Cases

| Test ID | Mô tả                                         | Expected Result                |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | Sync 200K databases paginated                  | Memory ≤ 500MB peak           |
| TC-002  | Priority: production DB changed                | Synced within 2 min           |
| TC-003  | Adaptive concurrency under CPU pressure        | Goroutines reduce to ≤ 50     |
| TC-004  | Incremental: no schema change                  | 70%+ databases skipped        |
| TC-005  | Tenant stagger: 100 tenants simultaneous       | No thundering herd            |
| TC-006  | Manual sync preempts auto queue                 | Immediate processing          |
| TC-007  | Cursor pagination: no duplicates               | All 200K synced exactly once  |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline   |
|---------|--------------------------------------|------------|
| Phase 1 | Paginated loading + cursor           | Sprint 1   |
| Phase 2 | Priority queue                       | Sprint 2   |
| Phase 3 | Adaptive concurrency                 | Sprint 2   |
| Phase 4 | Incremental checksum sync            | Sprint 3   |
| Phase 5 | Tenant stagger scheduling            | Sprint 3   |
| Phase 6 | Load testing 200K databases          | Sprint 4   |

---

## 7. Risks & Mitigations

| Risk                           | Impact | Mitigation                              |
|--------------------------------|--------|-----------------------------------------|
| Checksum false positive        | MEDIUM | Force full sync every 24h               |
| Priority starvation (low-pri)  | LOW    | Age-based priority boost after 4h       |
| Adaptive too aggressive        | MEDIUM | Min 10 goroutines, hysteresis buffer    |
| Pagination gap during mutation | LOW    | Cursor includes deleted, filter later   |
