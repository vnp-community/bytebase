# Solutions Registry — AI Readiness Change Requests

| Field           | Value                                    |
|-----------------|------------------------------------------|
| **Domain**      | AI-Assisted Development Readiness        |
| **Scope**       | `backend/` codebase                      |
| **Total SOLs**  | 5                                        |
| **Created**     | 2026-05-09                               |
| **Arch Ref**    | `specs/architecture.md`                  |
| **TDD Ref**     | `specs/TDD.md`                           |

---

## Solution Index

| SOL ID | CR Reference | Title | Affected Layers | Risk |
|--------|-------------|-------|-----------------|------|
| [SOL-AI-001](SOL-AI-001-service-decomposition.md) | CR-AI-001 | Same-Package Method Distribution | L4 (Service), L8 (Model) | Low |
| [SOL-AI-002](SOL-AI-002-interface-di-mocks.md) | CR-AI-002 | Granular Interface Injection & Mock Scaffold | L4, L8, L2 | Medium |
| [SOL-AI-003](SOL-AI-003-build-profiles-engine-matrix.md) | CR-AI-003 | Declarative Engine Capability Map | L7, L10, L2 | Medium |
| [SOL-AI-004](SOL-AI-004-resource-name-simplification.md) | CR-AI-004 | Typed Resource Refs & Parsers | L10 (Common) | Low |
| [SOL-AI-005](SOL-AI-005-bus-interface-acl-contract.md) | CR-AI-005 | EventBus Interface & Static ACL Map | L3, L5, L6 | High |

---

## Architecture Layer Coverage

Per `architecture.md` — 10-layer architecture:

```
L1  — Presentation          — (not affected)
L2  — API Gateway           — SOL-002 (wiring), SOL-003 (DriverRegistry)
L3  — Security              — SOL-005 (ACL static map)
L4  — Service               — SOL-001 (decompose), SOL-002 (interfaces)
L5  — Component             — SOL-005 (Bus interface)
L6  — Runner                — SOL-005 (EventBus migration)
L7  — Plugin                — SOL-003 (engine matrix)
L8  — Data Access (Store)   — SOL-001 (model split), SOL-002 (mock gen)
L9  — Enterprise            — (not affected)
L10 — Infrastructure        — SOL-003 (engine.go), SOL-004 (resource names)
```

---

## Key Design Decisions Across Solutions

| Decision | Rationale | Reference |
|----------|-----------|-----------|
| Same-package splitting (SOL-001) | Go allows multi-file method distribution — zero import/API impact | TDD.md §1.2 |
| Facade `DataStore` interface (SOL-002) | Services with broad store access use aggregate interface | TDD.md §4.1 |
| `init()` panic for missing engines (SOL-003) | Replaces `//exhaustive:enforce` linter with runtime check at startup | architecture.md §8 |
| Backward-compatible wrappers (SOL-004) | 50+ callers continue working while new typed API is adopted | TDD.md §14 |
| Fail-closed ACL (SOL-005) | Missing extractor → `CodeInternal` error, not silent bypass | TDD.md §7 |
| Typed bus channels (SOL-005) | Replace `chan int` with `chan struct{}` + typed events | TDD.md §5.1 |

---

## Dependency Order for Implementation

```
SOL-AI-003 (docs + engine matrix — no code dependencies)
    │
    ├── SOL-AI-001 (file decomposition — no code dependencies)
    │       │
    │       └── SOL-AI-002 (interface DI — depends on clear file boundaries)
    │               │
    │               └── SOL-AI-005 (bus/ACL — benefits from DI patterns)
    │
    └── SOL-AI-004 (resource names — independent, can run in parallel)
```

---

## Aggregate File Changes

| Action | Count |
|--------|-------|
| NEW files | ~25 |
| MODIFIED files | ~15 |
| DELETED files | 0 |
| Documentation files | 6 |
| Test files | 8 |

---

## Verification Commands

```bash
# After all solutions applied:
go build ./backend/...                              # default (ultimate)
go build -tags enterprise_core ./backend/...        # enterprise core
go build -tags minidemo ./backend/...               # minimal demo
go vet ./backend/...                                # static analysis
go test ./backend/common/...                        # common tests
go test ./backend/store/mock/...                    # mock verification
go test ./backend/api/v1/...                        # service + ACL tests
go test ./backend/component/bus/...                 # bus tests
go test ./backend/tests/...                         # integration tests
```
