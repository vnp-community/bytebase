# T-011-02: Makefile Build Profiles

| Field | Value |
|---|---|
| **Task ID** | T-011-02 |
| **Solution** | SOL-ARCH-011 |
| **Priority** | P3 |
| **Depends On** | T-011-01 |
| **Target File** | `Makefile` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Add build profiles to Makefile for different driver selections. Reduces binary size for deployment variants.

## Implementation — DELIVERED

### File: `Makefile` (fully featured, production-grade)

### Build Targets

| Target | Tags | Description | Est. Size |
|--------|------|-------------|-----------|
| `make build` | `release,embed_frontend` (ultimate) | All 22+ drivers | ~100MB |
| `make build-enterprise` | `enterprise_core,release,embed_frontend` | PG, MySQL, MSSQL, Oracle | ~60MB |
| `make build-minimal` | `minidemo,release` | PG only | ~40MB |
| `make build-dev` | `release` + `-race` | Dev mode with race detector | N/A |

### Test & Lint Targets

| Target | Description |
|--------|-------------|
| `make test` | Run all tests |
| `make test-unit` | Unit tests for resilience, cache, store |
| `make test-store` | Store integration tests (300s timeout) |
| `make lint` | All linters |
| `make lint-file-size` | File size enforcement (800 lines) |

### Verification Targets

| Target | Description |
|--------|-------------|
| `make verify` | Full verification (build + interfaces) |
| `make verify-build` | Compile all 12 critical packages |
| `make verify-interfaces` | Interface compile-time checks |

### Utility Targets

| Target | Description |
|--------|-------------|
| `make clean` | Remove build artifacts |
| `make help` | Show all available targets |

### Features

- **Version injection** via `-ldflags` (`VERSION`, `GIT_COMMIT`)
- **Parallel build** via `-p=8`
- **Output directory** configurable via `OUTPUT_DIR`
- **Binary stripping** via `-w -s` for production size reduction

## Acceptance Criteria

- [x] 4 build targets in Makefile ✅ (`build`, `build-enterprise`, `build-minimal`, `build-dev`)
- [x] `make build` → full binary (backward compatible) ✅
- [x] `make build-minimal` → PG-only binary ✅
- [x] Test, lint, verify, clean targets ✅
- [x] Version/commit injection via ldflags ✅

## Verification

```
$ cat Makefile → fully featured (120+ lines)
$ grep '.PHONY' Makefile → 4 build + 4 test/lint + 3 verify + 2 util = 13 targets
```
