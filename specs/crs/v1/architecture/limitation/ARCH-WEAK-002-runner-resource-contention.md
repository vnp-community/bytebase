# ARCH-WEAK-002 — Runner/API Resource Contention

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Weakness (Needs Fix)                       |
| **Layer**      | L6 (Runner), L8 (Store)                   |
| **Impact**     | Performance, API Latency                   |
| **Severity**   | High                                       |

---

## 1. Description

8 background runners share the **same `*sql.DB` connection pool** with all 30+ API service handlers. Heavy runner operations (schema sync, migration execution, data export) compete with user-facing API requests for database connections.

### Evidence (server.go:262-287 + db_connection.go:242-246)

```go
// All runners started in same process, using same store
go s.taskScheduler.Run(ctx, &s.runnerWG)    // heavy: migration execution
go s.schemaSyncer.Run(ctx, &s.runnerWG)     // heavy: sync all DB schemas
go s.approvalRunner.Run(ctx, &s.runnerWG)
go s.planCheckScheduler.Run(ctx, &s.runnerWG)
go s.dataCleaner.Run(ctx, &s.runnerWG)      // heavy: bulk deletes
go s.heartbeatRunner.Run(ctx, &s.runnerWG)
go s.notifyListener.Run(ctx, &s.runnerWG)
go mmm.Run(ctx, &s.runnerWG)

// Connection pool: HARD CAP at 50
maxOpenConns := maxConns - reservedConns
if maxOpenConns > 50 { maxOpenConns = 50 }  // ← shared by ALL
```

### Contention Scenario

```
User Query (API)         ←→  Schema Sync (Runner)
  needs 1 conn               needs 5+ conns (per instance)
  latency-sensitive           bulk operations
  
Pool: 50 conns max
If schema sync uses 30 → API gets 20 → users experience slowdown
```

---

## 2. Metrics

- **1** connection pool for everything (50 conn hard cap)
- **8** runners competing with **30+** API services
- **0** pool partitioning or priority
- **5** context timeout usages in entire runner/ directory
- Schema sync can scan **hundreds of instances** simultaneously

---

## 3. Root Cause

Single `DBConnectionManager` with single `*sql.DB`. No concept of pool isolation between API and background workloads.

```go
// db_connection.go — single pool
type DBConnectionManager struct {
    db *sql.DB           // ← SINGLE POOL for everything
    pgURLOrFile string
}
```

### Missing Architecture: Pool Isolation

```
CURRENT:
  API requests + Runners → [single sql.DB pool (50 conns)]

NEEDED:
  API requests  → [api_pool (35 conns)]
  Runners       → [runner_pool (15 conns)]
```
