# Change Request: In-Memory Cache Scaling for 200K+ Databases

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-002                                              |
| **Title**          | Cache Layer Scaling — Adaptive LRU & Tiered Caching      |
| **Category**       | Performance / Caching                                    |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01, DCM-09, ADM-08                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Hiện tại cache layer sử dụng hardcoded LRU sizes không phù hợp cho quy mô 200K+ databases. `databaseCache` chỉ chứa 32,768 entries (coverage <16%), `dbSchemaCache` chỉ 128 entries (coverage <0.1%). Cần scaling cache capacity và implement tiered caching strategy.

### 1.2 Bối cảnh (từ `backend/store/store.go`)
```go
databaseCache, err := lru.New[string, *DatabaseMessage](32768)    // 16% coverage @ 200K
dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](128, nil, 5*time.Minute)  // 0.06%
instanceCache, err := lru.New[string, *InstanceMessage](32768)    // Overkill cho instances
projectCache, err := lru.New[string, *ProjectMessage](32768)      // Overkill cho projects
```

### 1.3 Mục tiêu
- Cache hit ratio ≥ 85% cho database lookups tại quy mô 200K+
- Giảm metadata DB queries ≥ 60%
- Memory footprint scalable: ~2GB cho 200K database cache entries
- Hỗ trợ tenant-aware cache với hot/cold separation

---

## 2. Yêu cầu chức năng

### FR-001: Adaptive Cache Sizing
- **Mô tả**: Tự động adjust cache size dựa trên database count thực tế
- **Logic**:
  ```go
  func calculateCacheSize(entityCount int) int {
      // Cache 40% of entities, min 32768, max 500000
      target := max(32768, entityCount * 40 / 100)
      return min(target, 500000)
  }
  ```
- **AC**:
  - AC-1: Database cache size auto-scales khi database count tăng
  - AC-2: Memory monitoring exposed via Prometheus
  - AC-3: Cache resize không gây service interruption

### FR-002: Tiered Schema Cache
- **Mô tả**: 3-tier cache cho database schema: L1 (hot, in-process), L2 (warm, compressed), L3 (cold, Redis)
- **Tiers**:
  | Tier | Capacity    | TTL    | Storage           |
  |------|-------------|--------|-------------------|
  | L1   | 1,000       | 5 min  | In-process LRU    |
  | L2   | 50,000      | 30 min | Compressed proto  |
  | L3   | Unlimited   | 2 hr   | Redis (HA mode)   |
- **AC**:
  - AC-1: L1 hit ratio ≥ 90% cho recently accessed schemas
  - AC-2: L2 compression ratio ≥ 3x (proto → compressed)
  - AC-3: L3 fallback khi L1/L2 miss

### FR-003: Tenant-Aware Cache Partitioning
- **Mô tả**: Partition cache space theo tenant để tránh cache pollution
- **Logic**: Mỗi tenant (workspace) nhận quota = total_cache / tenant_count * weight_factor
- **AC**:
  - AC-1: Tenant với 50K databases không evict cache của tenant nhỏ hơn
  - AC-2: Cache metrics per-tenant cho monitoring
  - AC-3: Weight factor configurable per tenant

### FR-004: Batch Cache Warming
- **Mô tả**: Pre-load cache cho hot databases khi service start hoặc sau cache purge
- **Logic**: Load top N databases theo access frequency từ audit log
- **AC**:
  - AC-1: Top 10K databases pre-cached within 30s sau startup
  - AC-2: Cache warming không block API serving
  - AC-3: Warming progress tracked via health endpoint

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component          | File                              | Thay đổi                              |
|--------------------|-----------------------------------|----------------------------------------|
| Store Constructor  | `backend/store/store.go`          | Adaptive cache sizing, tiered schema cache |
| Cache Manager      | `backend/store/cache_manager.go`  | New: manages cache lifecycle, resize   |
| Cache Metrics      | `backend/metrics/cache_metrics.go`| Per-cache, per-tenant hit/miss/eviction |
| Cache Warmer       | `backend/store/cache_warmer.go`   | New: background cache warming          |
| Schema Cache       | `backend/store/db_schema.go`      | Tiered cache lookup (L1→L2→L3→DB)     |

### 3.2 Configuration

| Environment Variable     | Default   | Mô tả                              |
|--------------------------|-----------|--------------------------------------|
| `CACHE_AUTO_SCALE`       | `true`    | Enable adaptive cache sizing         |
| `CACHE_DB_MAX_SIZE`      | `500000`  | Max database cache entries           |
| `CACHE_SCHEMA_L1_SIZE`   | `1000`    | L1 schema cache entries              |
| `CACHE_SCHEMA_L2_SIZE`   | `50000`   | L2 compressed cache entries          |
| `CACHE_WARM_TOP_N`       | `10000`   | Databases to pre-warm                |
| `CACHE_TENANT_ISOLATION` | `true`    | Enable tenant cache partitioning     |

---

## 4. Performance Targets

| Metric                    | Current         | Target (200K+ DBs)  |
|---------------------------|-----------------|----------------------|
| Database cache hit ratio  | ~70% (at 200K)  | ≥ 85%               |
| Schema cache hit ratio    | ~5% (at 200K)   | ≥ 60%               |
| Cache memory footprint    | ~500MB fixed     | ~2GB adaptive       |
| Startup cache warm time   | N/A              | ≤ 30s               |
| DB query reduction        | baseline         | ≥ 60% fewer queries |

---

## 5. Test Cases

| Test ID | Mô tả                                         | Expected Result                |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | 200K databases, random access pattern          | Hit ratio ≥ 85%               |
| TC-002  | Cache resize from 32K → 80K during runtime     | No service interruption        |
| TC-003  | Tenant A (50K DBs) heavy load                  | Tenant B cache not evicted     |
| TC-004  | Cache warming on startup with 200K DBs         | Top 10K cached within 30s     |
| TC-005  | L2 compressed cache memory measurement         | ≤ 3x smaller than L1          |
| TC-006  | Memory pressure: system at 90% RAM             | Cache gracefully shrinks       |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline   |
|---------|--------------------------------------|------------|
| Phase 1 | Adaptive cache sizing               | Sprint 1   |
| Phase 2 | Tiered schema cache (L1+L2)         | Sprint 2   |
| Phase 3 | Tenant-aware partitioning           | Sprint 3   |
| Phase 4 | Cache warming + Redis L3            | Sprint 3   |
| Phase 5 | Load testing + tuning               | Sprint 4   |

---

## 7. Risks & Mitigations

| Risk                           | Impact | Mitigation                              |
|--------------------------------|--------|-----------------------------------------|
| Memory OOM with large cache    | HIGH   | Max cap + memory pressure monitoring    |
| Cache inconsistency across tiers | MEDIUM | Short TTL + invalidation propagation  |
| Warm-up storm on startup       | LOW    | Rate-limited warming, staggered start  |
| Tenant weight gaming           | LOW    | Admin-only weight configuration        |
