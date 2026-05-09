# Change Request: Plugin Build Tag Isolation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-011                                              |
| **Source ID**      | ARCH-WEAK-006                                            |
| **Title**          | Plugin Build Tag Isolation — Selective Driver Compilation |
| **Category**       | Architecture (Build Optimization)                        |
| **Priority**       | P3 — Low                                                 |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | ADM-08 (API Integration)                                 |

---

## 1. Tổng quan

### 1.1 Mô tả
Introduce Go build tags cho selective plugin compilation. Hiện tại 23 DB drivers + 9 advisor engines + ANTLR parsers all compiled into every binary via `init()` — tạo ~200MB binary even khi deployment chỉ dùng PostgreSQL.

### 1.2 Bối cảnh
- 23 DB driver directories all compiled via `init()` registration
- Plugin code ~160K lines = ~65% of backend code
- Each driver brings unique dependencies (MongoDB driver, Oracle OCI, etc.)
- Binary includes ANTLR grammars for all 23 SQL dialects
- Attack surface: unused drivers may have CVEs

### 1.3 Mục tiêu
- Build tags cho selective driver inclusion
- Default build: all drivers (backward compatible)
- Custom build: specify needed drivers only
- Binary size reduced 40-60% cho targeted deployments
- Docker multi-variant images (slim, full)

---

## 2. Yêu cầu chức năng

### FR-001: Build Tag per Driver
- **Mô tả**: Add build constraint to each driver package.
- **Logic**:
  ```go
  // plugin/db/pg/driver.go
  //go:build plugin_all || plugin_pg
  
  package pg
  
  func init() {
      db.Register(storepb.Engine_POSTGRES, func() db.Driver { return &Driver{} })
  }
  ```
  ```go
  // plugin/db/all/all.go — default import
  //go:build plugin_all || !(plugin_pg || plugin_mysql || ...)
  
  package all
  
  import (
      _ "github.com/bytebase/bytebase/backend/plugin/db/pg"
      _ "github.com/bytebase/bytebase/backend/plugin/db/mysql"
      // ... all drivers
  )
  ```
- **Acceptance Criteria**:
  - AC-1: Default build (`go build`) includes all drivers
  - AC-2: `go build -tags plugin_pg,plugin_mysql` includes only PG + MySQL
  - AC-3: Missing driver → clear error message at runtime

### FR-002: Build Variants
- **Mô tả**: Pre-defined build profiles cho common use cases.
- **Profiles**:

  | Profile | Drivers | Est. Binary Size |
  |---------|---------|------------------|
  | `full` (default) | All 23 | ~200MB |
  | `relational` | PG, MySQL, MSSQL, Oracle, SQLite | ~120MB |
  | `cloud` | PG, MySQL, Snowflake, BigQuery, Redshift | ~110MB |
  | `minimal` | PG only | ~80MB |

- **Acceptance Criteria**:
  - AC-1: Makefile targets for each profile
  - AC-2: Docker multi-stage builds for slim images

### FR-003: Runtime Driver Discovery
- **Mô tả**: API endpoint listing available drivers in current build.
- **Logic**:
  ```go
  // GET /v1/actuator → includes available_engines
  {
    "available_engines": ["POSTGRES", "MYSQL"],
    "build_profile": "relational"
  }
  ```
- **Acceptance Criteria**:
  - AC-1: UI shows only available engines in instance creation
  - AC-2: Creating instance with unavailable engine → clear error

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Build tags             | `backend/plugin/db/*/driver.go`       | Add `//go:build` constraint per driver       |
| Default import         | `backend/plugin/db/all/all.go`        | Import all drivers (default build)           |
| Advisor build tags     | `backend/plugin/advisor/*/`           | Matching build tags per engine               |
| Parser build tags      | `backend/plugin/parser/*/`            | Matching build tags per engine               |
| Actuator               | `backend/api/v1/actuator_service.go`  | Report available engines                     |
| Makefile               | `Makefile`                            | Build profile targets                        |
| Docker                 | `Dockerfile`                          | Multi-variant builds                         |

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Default build → all 23 drivers available                     | Backward compatible                      |
| TC-002     | `tags=plugin_pg` → only PostgreSQL driver                    | Selective compilation                    |
| TC-003     | Create MySQL instance on pg-only build → clear error         | User-friendly error                      |
| TC-004     | Binary size: full vs minimal → ≥40% reduction               | Size reduction achieved                  |
| TC-005     | Each build profile compiles and passes unit tests            | No broken configs                        |
| TC-006     | Actuator API reports available engines correctly             | Runtime discovery works                  |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Build tags for DB drivers (non-breaking)           | Sprint 1-2   |
| Phase 2 | Build tags for advisors + parsers                  | Sprint 2-3   |
| Phase 3 | Makefile profiles + Docker variants                | Sprint 3     |
| Phase 4 | Actuator engine discovery API                      | Sprint 3     |
| Phase 5 | Documentation + migration guide                   | Sprint 4     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Build tag combinations create untested configs | HIGH  | CI matrix tests for each predefined profile          |
| Import cycle when splitting drivers           | MEDIUM | Careful package structure, driver.go build tag only  |
| Users confused by missing engines             | MEDIUM | Clear error messages + UI filter                     |
| Default build must not change                 | HIGH   | `plugin_all` default tag, no change for existing users |
