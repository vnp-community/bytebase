# T-011-01: Build Tags per Driver

| Field | Value |
|---|---|
| **Task ID** | T-011-01 |
| **Solution** | SOL-ARCH-011 |
| **Priority** | P3 |
| **Depends On** | None |
| **Target Files** | `backend/server/ultimate.go`, `enterprise_core.go`, `minimal.go` |
| **Type** | Modified existing (3 profile files with build tags) |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Add `//go:build` constraints to control which DB drivers are compiled per profile. Default build includes all drivers.

## Implementation — DELIVERED

### Architecture: Profile-Based Driver Selection

Instead of per-driver `//go:build plugin_pg || plugin_all` tags (spec's approach), the team chose **profile-level build tags** using mutually exclusive `//go:build` constraints on import aggregation files:

### Profile Files

| File | Build Tag | Engines | Lines |
|------|-----------|---------|-------|
| `ultimate.go` | `!minidemo && !enterprise_core` (default) | ALL 22+ engines | 87 |
| `enterprise_core.go` | `enterprise_core` | PG, MySQL, MariaDB, OceanBase, MSSQL, Oracle, CockroachDB, Redis (6 core) | 60 |
| `minimal.go` | `minidemo` | PostgreSQL ONLY (1 engine) | 33 |

### Build Tag Logic

```go
// ultimate.go — compiled when NO special tags are set (default)
//go:build !minidemo && !enterprise_core

// enterprise_core.go — compiled ONLY with -tags enterprise_core
//go:build enterprise_core

// minimal.go — compiled ONLY with -tags minidemo
//go:build minidemo
```

### AI-CONTEXT Comments (added by user)

Each profile file now includes AI-CONTEXT comments documenting:
- Profile name and activation condition
- Available engines list
- Available plugins
- Cross-reference to `BUILD_PROFILES.md`

### Driver Registration: 24 `init()` Registrations

```
$ grep -rn 'func init()' backend/plugin/db/ | wc -l → 24
```

Each engine registers via `db.Register(engine, factory)` at init-time. Build tags at the profile level control which `init()` functions are linked.

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| Per-driver `//go:build plugin_pg \|\| plugin_all` | Profile-level `//go:build` on import files | Simpler: 3 files vs 23+ files; same outcome |
| `backend/plugin/db/all/all.go` aggregator | `backend/server/ultimate.go` with blanket imports | Existing pattern in codebase |
| 23 drivers | 24 `init()` registrations | Some engines split (MySQL + MariaDB, etc.) |

## Acceptance Criteria

- [x] 3 profile files with `//go:build` constraints ✅
- [x] `go build ./backend/server/...` (no tags) → ultimate (all drivers) ✅
- [x] `go build -tags enterprise_core ./backend/server/...` → core engines ✅
- [x] `go build -tags minidemo ./backend/server/...` → PG only ✅
- [x] All 3 profiles compile successfully ✅

## Verification

```
$ go build ./backend/server/... → ✅ PASS (ultimate)
$ go build -tags enterprise_core ./backend/server/... → ✅ PASS
$ go build -tags minidemo ./backend/server/... → ✅ PASS
$ wc -l backend/server/{ultimate,enterprise_core,minimal}.go → 87 + 60 + 33 = 180
```
