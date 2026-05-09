# Change Request: Build Profile Registry & Driver Interface Extraction

| Field              | Value                                                             |
|--------------------|-------------------------------------------------------------------|
| **CR ID**          | CR-AI-003                                                         |
| **Issue IDs**      | AI-BLOCKER-003, AI-BLOCKER-004                                    |
| **Title**          | Build Profile Documentation & Data-Driven Engine Capability Matrix |
| **Category**       | Architecture / Documentation (ARCH)                               |
| **Priority**       | P1 — High                                                         |
| **Status**         | Draft                                                             |
| **Created**        | 2026-05-09                                                        |
| **Author**         | VNP AI Ops Team                                                   |
| **PRD Refs**       | ADM-08 (API Integration), §5 (22+ Database Engines), §6 (Deployment) |

---

## 1. Tổng quan

### 1.1 Mô tả
Giải quyết hai vấn đề liên quan đến database engine management:
1. **Build tags ẩn**: 3 build profiles (`ultimate`, `enterprise_core`, `minidemo`) tạo dependency graph vô hình cho AI agents
2. **Engine switch sprawl**: 11 switch statements trùng lặp trong `common/engine.go` (493 LOC) phải update đồng thời khi thêm engine mới

### 1.2 Bối cảnh
Bytebase hỗ trợ 22+ database engines (PRD §5) được triển khai dưới dạng plugin driver (`backend/plugin/db/<engine>/`). Hệ thống sử dụng build tags để tạo binary profiles cho các deployment modes (PRD §6):

| Profile | Build Constraint | Engines | Deployment |
|---------|-----------------|---------|------------|
| Ultimate | `!enterprise_core && !minidemo` | 22+ (full) | Docker, K8s, Self-hosted |
| Enterprise Core | `enterprise_core && !minidemo` | 5 core | Cloud SaaS |
| Minimal Demo | `minidemo` | Minimal | Demo/eval |

`common/engine.go` defines 11 capability functions, mỗi function chứa 22-way exhaustive switch — tổng 493 LOC repetitive code.

### 1.3 Mục tiêu
- AI agents có thể xác định engines available cho mỗi build profile mà không cần parse build tags
- Thêm engine mới = 1 line change (thay vì 11 switch updates)
- Giảm `engine.go` từ 493 LOC xuống ~100 LOC

---

## 2. Yêu cầu chức năng

### FR-001: Build Profile Registry Document
- **Mô tả**: Tạo `BUILD_PROFILES.md` structured documentation
- **Content**:
  ```markdown
  ## Profile: ultimate
  - Build tag: `!enterprise_core && !minidemo`
  - File: `backend/server/ultimate.go`
  - Engines: PostgreSQL, MySQL, TiDB, MariaDB, OceanBase, Oracle,
             MSSQL, Snowflake, ClickHouse, MongoDB, Redis, Spanner,
             BigQuery, Cassandra, CosmosDB, DynamoDB, Elasticsearch,
             Hive, Databricks, Trino, StarRocks, SQLite,
             CockroachDB, Redshift
  - Plugins: All (advisor, parser, schema, mailer, webhook, idp, stripe)

  ## Profile: enterprise_core
  - Build tag: `enterprise_core && !minidemo`
  - File: `backend/server/enterprise_core.go`
  - Engines: PostgreSQL, MySQL, TiDB, MariaDB, OceanBase
  - Plugins: Core subset (no stripe, limited advisor)

  ## Profile: minidemo
  - Build tag: `minidemo`
  - File: `backend/server/minimal.go`
  - Engines: PostgreSQL, MySQL (demo only)
  - Plugins: Minimal
  ```
- **Acceptance Criteria**:
  - AC-1: Document accurately reflects each profile's engine set
  - AC-2: Document is co-located with build-tagged files

### FR-002: AI Context Comments in Build-Tagged Files
- **Mô tả**: Thêm structured comments vào mỗi build-tagged file
- **Logic**:
  ```go
  // AI-CONTEXT: This file is compiled ONLY when build tags satisfy:
  //   enterprise_core=true AND minidemo=false
  // Available engines: Postgres, MySQL, TiDB, MariaDB, OceanBase
  // Available plugins: core advisor, core parser, core schema
  // See BUILD_PROFILES.md for full profile comparison
  ```
- **Acceptance Criteria**:
  - AC-1: Every `//go:build` file has `AI-CONTEXT` comment block
  - AC-2: Comments include engine list and plugin list

### FR-003: Data-Driven Engine Capability Matrix
- **Mô tả**: Thay thế 11 switch statements bằng declarative map
- **Logic**:
  ```go
  // EngineCapabilities defines which features an engine supports.
  type EngineCapabilities struct {
      SQLReview       bool
      QueryNewACL     bool
      Masking         bool
      AutoComplete    bool
      StatementAdvise bool
      StatementReport bool
      PriorBackup     bool
      CreateDatabase  bool
      QuerySpanPlain  bool
      SyntaxCheck     bool
      BackupDBName    string // default: "bbdataarchive"
  }

  // engineCapabilities is the single source of truth for engine support.
  // Adding a new engine requires ONE entry here instead of 11 switch cases.
  var engineCapabilities = map[storepb.Engine]EngineCapabilities{
      storepb.Engine_POSTGRES:     {SQLReview: true, QueryNewACL: true, Masking: true, ...},
      storepb.Engine_MYSQL:        {SQLReview: true, QueryNewACL: true, Masking: true, ...},
      storepb.Engine_TIDB:         {SQLReview: true, QueryNewACL: false, Masking: true, ...},
      // ... one line per engine (22 lines total)
  }

  func init() {
      // Compile-time exhaustiveness: panic if any known engine is missing
      for _, e := range storepb.Engine_value {
          eng := storepb.Engine(e)
          if eng == storepb.Engine_ENGINE_UNSPECIFIED { continue }
          if _, ok := engineCapabilities[eng]; !ok {
              panic(fmt.Sprintf("engine %s missing from capability matrix", eng))
          }
      }
  }

  // Public API preserved — backward compatible
  func EngineSupportSQLReview(e storepb.Engine) bool {
      return engineCapabilities[e].SQLReview
  }
  ```
- **PRD Alignment**: §5 (22+ Database Engines) — ensuring consistent capability enforcement
- **Acceptance Criteria**:
  - AC-1: All 11 existing functions produce identical results
  - AC-2: `engine.go` reduced from 493 LOC to ≤120 LOC
  - AC-3: `init()` panics if a new protobuf engine value is missing from map
  - AC-4: Adding a new engine = 1 map entry

### FR-004: Driver Registry Interface
- **Mô tả**: Define explicit interface cho driver availability
- **Logic**:
  ```go
  // DriverRegistry exposes which database drivers are compiled in.
  type DriverRegistry interface {
      AvailableEngines() []storepb.Engine
      IsEngineAvailable(engine storepb.Engine) bool
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Each build-tagged file implements `DriverRegistry`
  - AC-2: Server exposes available engines via health endpoint

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                       | Thay đổi                                         |
|------------------------|--------------------------------------------|---------------------------------------------------|
| Engine Matrix          | `backend/common/engine.go`                 | Replace 11 switches with declarative map          |
| Engine Tests           | `backend/common/engine_test.go`            | NEW — verify matrix matches old behavior          |
| Build Profile Doc      | `backend/server/BUILD_PROFILES.md`         | NEW — structured profile documentation            |
| Ultimate Profile       | `backend/server/ultimate.go`               | Add AI-CONTEXT comment block                      |
| Enterprise Profile     | `backend/server/enterprise_core.go`        | Add AI-CONTEXT comment block                      |
| Minimal Profile        | `backend/server/minimal.go`               | Add AI-CONTEXT comment block                      |
| Config Dev             | `backend/common/config_dev.go`             | Add AI-CONTEXT comment block                      |
| Config Release         | `backend/common/config_release.go`         | Add AI-CONTEXT comment block                      |
| Driver Registry        | `backend/server/driver_registry.go`        | NEW — DriverRegistry interface + implementation   |

### 3.2 Không có Database Changes

---

## 4. Phụ thuộc

| Dependency             | Mô tả                                                          |
|------------------------|------------------------------------------------------------------|
| `storepb.Engine` enum  | Protobuf enum values — source of truth for engine list          |
| `//exhaustive:enforce` | Linter directive — replaced by `init()` runtime check           |

---

## 5. Test Cases

| Test ID | Mô tả                                                             | Expected Result                    |
|---------|---------------------------------------------------------------------|------------------------------------|
| TC-001  | Compare old switch output vs new map output for all 22 engines     | Identical results per capability   |
| TC-002  | Add fake `Engine_TESTDB` to proto — verify `init()` panics        | Panic with descriptive message     |
| TC-003  | `go build -tags enterprise_core ./backend/...`                     | Compiles successfully              |
| TC-004  | `go build -tags minidemo ./backend/...`                            | Compiles successfully              |
| TC-005  | `go build ./backend/...` (ultimate default)                        | Compiles successfully              |
| TC-006  | Verify `engine.go` ≤120 LOC after refactor                        | Max 120 lines                      |
| TC-007  | `BUILD_PROFILES.md` exists and lists all 3 profiles               | Document present, accurate         |
| TC-008  | AI-CONTEXT comments present in all build-tagged files              | 5 files annotated                  |

---

## 6. Rollout Plan

| Phase   | Mô tả                                                    | Timeline  |
|---------|-----------------------------------------------------------|-----------|
| Phase 1 | Create `BUILD_PROFILES.md` + AI-CONTEXT comments          | Sprint 1  |
| Phase 2 | Refactor `engine.go` to data-driven matrix                | Sprint 1  |
| Phase 3 | Write engine matrix tests (old vs new comparison)         | Sprint 1  |
| Phase 4 | Implement `DriverRegistry` interface                       | Sprint 2  |
| Phase 5 | CI verification across all build profiles                  | Sprint 2  |

---

## 7. Risks & Mitigations

| Risk                                           | Impact | Mitigation                                         |
|------------------------------------------------|--------|-----------------------------------------------------|
| `init()` panic in production                   | HIGH   | Only panics for missing engines — caught in CI     |
| `//exhaustive:enforce` linter conflicts        | LOW    | Remove linter directive from refactored functions  |
| Build-tagged file edits trigger recompilation  | LOW    | Comments only — no functional change               |

---

## 8. Success Metrics

| Metric                          | Before  | Target     |
|---------------------------------|---------|------------|
| `engine.go` LOC                 | 493     | ≤120       |
| Switch statements               | 11      | 0          |
| Lines to add new engine         | 11×2=22 | 1          |
| Build profiles documented       | 0       | 3          |
| Build-tagged files with context  | 0      | 5+         |
