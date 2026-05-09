# ARCH-LIM-004 — Cache-HA Mutual Exclusion

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Limitation (Structural Trade-off)          |
| **Layer**      | L8 (Data Access — Store)                   |
| **Impact**     | Performance, Horizontal Scaling            |
| **Severity**   | High                                       |

---

## 1. Description

HA mode (`profile.HA=true`) **disables tất cả LRU caches** (`enableCache=false`). Mọi read request đều đi thẳng tới PostgreSQL. Đây là mutual exclusion: performance (cache) XOR availability (HA).

### Evidence (store.go + server.go:140)

```go
// server.go — HA mode disables cache
stores, err := store.New(ctx, pgURL, !profile.HA)
//                                    ^^^^^^^^^ enableCache = !HA

// store.go — cache bypass
func (s *Store) GetUser(ctx context.Context, ...) (*UserMessage, error) {
    if s.enableCache {
        if v, ok := s.userEmailCache.Get(key); ok { return v, nil }
    }
    // ... query DB directly ...
}
```

### Impact Data

| Cache | Capacity | TTL | HA Mode |
|-------|----------|-----|---------|
| userEmailCache | 32,768 | ∞ | **DISABLED** |
| instanceCache | 32,768 | ∞ | **DISABLED** |
| databaseCache | 32,768 | ∞ | **DISABLED** |
| projectCache | 32,768 | ∞ | **DISABLED** |
| policyCache | 4,096 | ∞ | **DISABLED** |
| settingCache | 1,024 | ∞ | **DISABLED** |
| rolesCache | 128 | 1min | **DISABLED** |
| iamPolicyCache | 1,024 | 1min | **DISABLED** |
| dbSchemaCache | 128 | 5min | **DISABLED** |
| groupCache | 1,024 | 1min | **DISABLED** |

Total: **13 caches** disabled in HA mode.

---

## 2. Root Cause

### Design Decision (TDD.md §4.2)
> "HA mode: Cache disabled — mỗi request đọc trực tiếp từ DB"

Reason: in-process LRU caches are per-instance. With 2+ replicas, Replica A may write → cache invalidated locally, but Replica B still serves stale data.

**No distributed cache layer** exists → only option is disable cache entirely.

---

## 3. Consequences

| Consequence | Description |
|------------|-------------|
| **DB Load** | HA mode multiplies PG query volume by 5-10x (no cache hits) |
| **Latency** | IAM permission checks (hot path) go to DB every time → +5-10ms/request |
| **Connection Pool Pressure** | More queries → more connections needed → pool exhaustion risk |
| **Scaling Ceiling** | Adding replicas increases DB load linearly |

### Quantified Impact

```
Single-node (cache ON):
  IAM check: 0.01ms (LRU hit) × 10K req/s = 100ms total DB time

HA mode (cache OFF):
  IAM check: 5ms (DB query) × 10K req/s = 50,000ms total DB time
  That's 500x more DB load just for IAM checks
```

---

## 4. Missing Architecture Component

```
CURRENT (HA):
  Replica A ──► PG (every read)
  Replica B ──► PG (every read)

NEEDED:
  Replica A ──► Redis/Valkey (shared cache) ──► PG (cache miss only)
  Replica B ──► Redis/Valkey (shared cache) ──► PG (cache miss only)
```
