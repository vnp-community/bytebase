# ARCH-WEAK-006 — Plugin Binary Inflation

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Weakness (Needs Fix)                       |
| **Layer**      | L7 (Plugin)                                |
| **Impact**     | Binary Size, Build Time, Attack Surface    |
| **Severity**   | Medium                                     |

---

## 1. Description

Tất cả 23 database drivers, 9 advisor engines, SQL parsers (ANTLR4) đều compile vào **single binary** qua `init()` registration. Mọi deployment — kể cả chỉ dùng PostgreSQL — đều carry full MongoDB, Oracle, DynamoDB, Elasticsearch drivers.

### Evidence

```bash
# 23 DB driver directories (all compiled into binary)
$ ls backend/plugin/db/
bigquery  cassandra  clickhouse  cockroachdb  cosmosdb  databricks
dynamodb  elasticsearch  hive  mongodb  mssql  mysql  oracle  pg
redis  redshift  snowflake  spanner  sqlite  starrocks  tidb  trino

# Each driver uses init() auto-registration
func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}
```

### Size Metrics

```bash
# Largest plugin files (non-generated)
plugin/schema/pg/get_database_definition.go     4,420 lines
plugin/parser/redshift/query_span_extractor.go   3,648 lines
plugin/schema/differ.go                          2,661 lines
plugin/parser/tsql/completion.go                 2,279 lines

# Total plugin code:
$ find backend/plugin -name '*.go' -not -name '*_test.go' | xargs wc -l | tail -1
→ ~160,000 lines   (248,180 total backend - 36,812 service - 50K store/runner/etc)
```

Plugin code represents **~65%** of total backend code — compiled into every binary.

---

## 2. Consequences

| Consequence | Description |
|------------|-------------|
| **Binary Size** | Final binary ~200MB+ due to ANTLR grammars, all drivers |
| **Build Time** | Compiling 23 drivers + parsers even when only 1-2 needed |
| **Attack Surface** | Unused drivers (e.g., Redis, DynamoDB) still in binary → potential CVEs |
| **Memory Footprint** | ANTLR parser states for all 23 engines loaded at startup |
| **Go Module Deps** | Each driver brings unique dependencies (MongoDB driver, Oracle OCI, etc.) |

---

## 3. Root Cause

### Design Decision (TDD.md §6.1)
> "22 driver implementations: Mỗi driver trong `backend/plugin/db/<engine>/` tự register qua `init()`. Factory pattern qua `db.Open()`."

The `init()` registration pattern is idiomatic Go but forces all plugins into the binary. There's no build-time selection mechanism.

### Alternative Pattern (Not Currently Used)

```go
// Build tag approach — compile only needed drivers
//go:build plugin_pg || plugin_mysql

func init() {
    db.Register(storepb.Engine_POSTGRES, ...)
}
```

Or plugin system using Go `plugin` package (`.so` loading) — but this has cross-platform compatibility issues.

---

## 4. Measurement

| Metric | Current | Target |
|--------|---------|--------|
| Compiled DB drivers | 23 | Configurable via build tags |
| Plugin code lines | ~160K | — (same, but selectively compiled) |
| Binary size | ~200MB | < 80MB (core + selected plugins) |
| Build time | Full | Reduced by 40-50% with selective compilation |
