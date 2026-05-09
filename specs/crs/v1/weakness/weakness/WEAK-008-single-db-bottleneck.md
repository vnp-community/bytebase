# WEAK-008 — Single Database Metadata Bottleneck

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-008                                   |
| Category       | Performance / Scalability                  |
| Severity       | MEDIUM                                     |
| Affected Layer | L8 (Store), L10 (Infrastructure)           |
| Source Files   | `backend/store/db_connection.go`           |

---

## Mô tả

Tất cả metadata (users, projects, issues, changelogs, audit logs, task runs) lưu trong **single PostgreSQL instance** với connection pool capped tại 50.

## Chi tiết

### Connection Pool Limit

```go
// db_connection.go:242-246
maxOpenConns := maxConns - reservedConns
if maxOpenConns > 50 {
    maxOpenConns = 50  // Hard cap
}
db.SetMaxOpenConns(maxOpenConns)
```

- Hard cap 50 connections bất kể PG max_connections.
- 8 background runners + API requests + LSP + MCP chia sẻ 50 connections.
- Heavy workloads (bulk migration, schema sync) có thể exhaust pool.

### Single DB chứa tất cả data domains

- Authentication (users, tokens, sessions)
- Change management (plans, issues, tasks, task runs, changelogs)
- Security (policies, audit logs, access grants)
- Configuration (settings, environments, instances)
- Schema metadata (database schemas, revisions)

### Connection Reconnection Weakness

```go
// db_connection.go:138-139
func (m *DBConnectionManager) reloadConnection(ctx, filePath) {
    time.Sleep(100 * time.Millisecond) // Arbitrary delay
    // ...
    // Force close after 1 hour as a safety measure
    go func() {
        time.Sleep(1 * time.Hour)  // ← 1 hour drain window
        oldDB.Close()
    }()
}
```

- 100ms arbitrary sleep for file write completion — race condition possible.
- 1 hour grace period for old connections — resource leak potential.

### No Read Replica Support

- All reads and writes go to same PostgreSQL instance.
- No read/write splitting capability.

## Impact

- Under high load, connection pool exhaustion causes 503 errors.
- Audit log writes compete with real-time API queries.
- Schema sync (periodic) adds continuous background load.

## Khuyến nghị

1. Increase connection pool cap based on workload profiling.
2. Implement connection pool metrics (active/idle/waiting).
3. Consider read/write splitting for heavy read paths.
4. Use separate connection pools for runners vs API.
