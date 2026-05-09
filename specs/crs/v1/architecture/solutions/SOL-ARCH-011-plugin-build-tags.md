# Solution: Plugin Build Tag Isolation — CR-ARCH-011

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-011                                             |
| **CR Reference**   | CR-ARCH-011                                              |
| **Title**          | Go Build Tags for Selective Driver Compilation           |
| **Affected Layers**| L7 (Plugin)                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §7 (L7 — Plugin Layer):
- 23 DB drivers registered via `init()` in `plugin/db/*/driver.go`
- Each driver: DB driver + advisor + parser (ANTLR grammar)
- Total plugin code: ~160K lines

Per [TDD.md](../../TDD.md) §7:
- Plugin registration via `db.Register(engine, factory)` in `init()`
- Import-driven inclusion: `_ "github.com/bytebase/bytebase/backend/plugin/db/pg"`

---

## 2. Current Implementation

### 2.1 Auto-Registration (plugin/db/pg/driver.go)

```go
package pg

func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}
```

### 2.2 Bulk Import (typically in main or a central file)

```go
import (
    _ "github.com/bytebase/bytebase/backend/plugin/db/pg"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mysql"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mongodb"
    // ... 20 more drivers
)
```

---

## 3. Solution Design

### 3.1 Build Tag per Driver

**Modified file**: `backend/plugin/db/pg/driver.go`

```go
//go:build plugin_all || plugin_pg

package pg

import (
    db "github.com/bytebase/bytebase/backend/plugin/db"
    storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

func init() {
    db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
}
```

**Modified file**: `backend/plugin/db/mysql/driver.go`

```go
//go:build plugin_all || plugin_mysql

package mysql

func init() {
    db.Register(storepb.Engine_MYSQL, func() db.Driver { return &Driver{} })
}
```

### 3.2 Default Build — Include All

**New file**: `backend/plugin/db/all/all.go`

```go
// Package all imports all database drivers for the default build.
// The default build (no tags specified) includes all drivers via plugin_all.
package all

// Build with: go build (no tags) → all drivers included
// Build with: go build -tags plugin_pg,plugin_mysql → only PG + MySQL

// When no tags are specified, Go includes files without build constraints.
// We provide an explicit "all" import for clarity.
import (
    _ "github.com/bytebase/bytebase/backend/plugin/db/pg"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mysql"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mongodb"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mssql"
    _ "github.com/bytebase/bytebase/backend/plugin/db/oracle"
    _ "github.com/bytebase/bytebase/backend/plugin/db/clickhouse"
    _ "github.com/bytebase/bytebase/backend/plugin/db/snowflake"
    _ "github.com/bytebase/bytebase/backend/plugin/db/bigquery"
    _ "github.com/bytebase/bytebase/backend/plugin/db/redshift"
    _ "github.com/bytebase/bytebase/backend/plugin/db/sqlite"
    _ "github.com/bytebase/bytebase/backend/plugin/db/tidb"
    _ "github.com/bytebase/bytebase/backend/plugin/db/oceanbase"
    _ "github.com/bytebase/bytebase/backend/plugin/db/mariadb"
    _ "github.com/bytebase/bytebase/backend/plugin/db/spanner"
    _ "github.com/bytebase/bytebase/backend/plugin/db/dynamodb"
    _ "github.com/bytebase/bytebase/backend/plugin/db/elasticsearch"
    _ "github.com/bytebase/bytebase/backend/plugin/db/cockroachdb"
    _ "github.com/bytebase/bytebase/backend/plugin/db/risingwave"
    _ "github.com/bytebase/bytebase/backend/plugin/db/hive"
    _ "github.com/bytebase/bytebase/backend/plugin/db/starrocks"
    _ "github.com/bytebase/bytebase/backend/plugin/db/doris"
    _ "github.com/bytebase/bytebase/backend/plugin/db/dm"
    _ "github.com/bytebase/bytebase/backend/plugin/db/databricks"
)
```

### 3.3 Matching Build Tags for Advisors + Parsers

Each advisor and parser file for an engine gets the matching build tag:

```go
// backend/plugin/advisor/pg/advisor.go
//go:build plugin_all || plugin_pg

package pg
// ... PostgreSQL advisor implementation
```

```go
// backend/plugin/parser/pg/parser.go
//go:build plugin_all || plugin_pg

package pg
// ... PostgreSQL ANTLR parser
```

### 3.4 Build Profiles (Makefile)

**Modified file**: `Makefile`

```makefile
# Default: all drivers (backward compatible)
.PHONY: build
build:
	go build -tags plugin_all -o bytebase ./backend/cmd/server

# Profile: relational databases only
.PHONY: build-relational
build-relational:
	go build -tags "plugin_pg,plugin_mysql,plugin_mssql,plugin_oracle,plugin_sqlite" \
		-o bytebase-relational ./backend/cmd/server

# Profile: cloud data warehouses
.PHONY: build-cloud
build-cloud:
	go build -tags "plugin_pg,plugin_mysql,plugin_snowflake,plugin_bigquery,plugin_redshift" \
		-o bytebase-cloud ./backend/cmd/server

# Profile: minimal (PostgreSQL only)
.PHONY: build-minimal
build-minimal:
	go build -tags "plugin_pg" -o bytebase-minimal ./backend/cmd/server
```

### 3.5 Runtime Engine Discovery

**Modified file**: `backend/api/v1/actuator_service.go`

```go
func (s *ActuatorService) GetActuatorInfo(ctx context.Context, ...) (*v1pb.ActuatorInfo, error) {
    info := &v1pb.ActuatorInfo{
        Version:          s.profile.Version,
        GitCommit:        s.profile.GitCommit,
        // NEW: report available engines
        AvailableEngines: db.RegisteredEngines(),
    }
    return info, nil
}
```

**Modified file**: `backend/plugin/db/registry.go`

```go
func RegisteredEngines() []storepb.Engine {
    registryMu.RLock()
    defer registryMu.RUnlock()
    engines := make([]storepb.Engine, 0, len(registry))
    for engine := range registry {
        engines = append(engines, engine)
    }
    return engines
}
```

### 3.6 Docker Multi-Variant Builds

**Modified file**: `Dockerfile`

```dockerfile
# === FULL BUILD (default) ===
FROM golang:1.24-alpine AS builder-full
WORKDIR /src
COPY . .
RUN go build -tags plugin_all -o /bytebase ./backend/cmd/server

# === MINIMAL BUILD ===
FROM golang:1.24-alpine AS builder-minimal
WORKDIR /src
COPY . .
RUN go build -tags "plugin_pg" -o /bytebase ./backend/cmd/server

# === RUNTIME ===
FROM alpine:3.20 AS runtime-full
COPY --from=builder-full /bytebase /usr/local/bin/bytebase
ENTRYPOINT ["bytebase"]

FROM alpine:3.20 AS runtime-minimal
COPY --from=builder-minimal /bytebase /usr/local/bin/bytebase
ENTRYPOINT ["bytebase"]
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/plugin/db/*/driver.go` (23 files) | L7 | **MODIFY** | Add `//go:build` tag |
| `backend/plugin/advisor/*/` (23 dirs) | L7 | **MODIFY** | Matching build tags |
| `backend/plugin/parser/*/` (23 dirs) | L7 | **MODIFY** | Matching build tags |
| `backend/plugin/db/all/all.go` | L7 | **NEW** | Default all-drivers import |
| `backend/plugin/db/registry.go` | L7 | **MODIFY** | `RegisteredEngines()` |
| `backend/api/v1/actuator_service.go` | L4 | **MODIFY** | Report engines |
| `Makefile` | CI | **MODIFY** | Build profiles |
| `Dockerfile` | CI | **MODIFY** | Multi-variant images |

---

## 5. CI Matrix

```yaml
# .github/workflows/build-profiles.yml
strategy:
  matrix:
    profile:
      - { name: full, tags: "plugin_all" }
      - { name: relational, tags: "plugin_pg,plugin_mysql,plugin_mssql,plugin_oracle,plugin_sqlite" }
      - { name: minimal, tags: "plugin_pg" }

steps:
  - run: go build -tags "${{ matrix.profile.tags }}" ./backend/cmd/server
  - run: go test -tags "${{ matrix.profile.tags }}" ./backend/...
```

---

## 6. Migration Strategy

1. Add build tags to ALL driver files (backward compatible — default `plugin_all`)
2. Verify `go build` without tags still includes all drivers
3. Add `make build-minimal` for testing
4. CI matrix for all profiles
5. Docker multi-variant images

---

## 7. Rollback Plan

Remove `//go:build` constraint lines → all files included in every build (original behavior).
