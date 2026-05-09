# AI-BLOCKER-003: Build Tags Create Invisible Dependency Graph

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-003 |
| **Severity** | 🟠 High |
| **Category** | Build Complexity / Conditional Compilation |
| **Layer** | L10 Infra (`backend/server/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

The project uses Go build tags to conditionally compile different plugin sets, creating 3+ distinct binary profiles whose dependency graphs are invisible to AI tools:

| File | Build Constraint | Plugins Included |
|------|-----------------|------------------|
| `ultimate.go` | `!enterprise_core && !minidemo` | All 8 DB drivers + all plugins |
| `enterprise_core.go` | `enterprise_core && !minidemo` | 5 core DB drivers only |
| `minimal.go` | `minidemo` | Minimal demo set |

AI agents that analyze or modify code without specifying the build tag context will:
- Suggest imports for drivers that don't exist in `enterprise_core` builds
- Generate code that references plugins unavailable in `minidemo`
- Fail to understand which `initXxxDrivers()` calls are active

## Impact on AI Operations

- **Silent Build Failures**: AI generates `import "plugin/db/hive"` → builds fine with `ultimate` tag → fails with `enterprise_core` tag.
- **Phantom Dependencies**: AI cannot determine if `plugin/db/dynamodb` is available without reading all 3 build-tagged files and understanding the tag Boolean algebra.
- **Incomplete Analysis**: Static analysis tools (gopls, golangci-lint) used by AI operate on a single build configuration. Code that compiles under one tag but not another goes undetected.

## Evidence

```go
// backend/server/ultimate.go
//go:build !enterprise_core && !minidemo

func initDrivers(s *Server) {
    s.initPostgresDriver()
    s.initMySQLDriver()
    s.initOracleDriver()
    s.initMSSQLDriver()
    s.initSnowflakeDriver()
    // ... 8 total drivers
}

// backend/server/enterprise_core.go
//go:build enterprise_core && !minidemo

func initDrivers(s *Server) {
    s.initPostgresDriver()
    s.initMySQLDriver()
    // ... only 5 drivers
}
```

Additionally, `common/config_dev.go` and `common/config_release.go` use build tags to define different constants, further fragmenting the dependency graph.

## Recommended Remediation

1. **Build Tag Registry**: Create `backend/server/BUILD_PROFILES.md` documenting each profile's included drivers/plugins as a quick-reference for AI agents.

2. **Driver Interface Extraction**: Define a `DriverRegistry` interface that all profiles implement, making the contract visible regardless of build tags:
   ```go
   type DriverRegistry interface {
       AvailableEngines() []storepb.Engine
       GetDriver(engine storepb.Engine) (db.Driver, error)
   }
   ```

3. **AI Context Hint**: Add structured comments at the top of each build-tagged file:
   ```go
   // AI-CONTEXT: This file is compiled ONLY when build tags satisfy:
   //   enterprise_core=true AND minidemo=false
   // Available drivers: Postgres, MySQL, TiDB, MariaDB, OceanBase
   ```

## Files to Modify

```
backend/server/ultimate.go
backend/server/enterprise_core.go
backend/server/minimal.go
backend/common/config_dev.go
backend/common/config_release.go
NEW: backend/server/BUILD_PROFILES.md
```
