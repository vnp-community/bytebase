# WEAK-007 — Insufficient Unit Test Coverage in Core Paths

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-007                                   |
| Category       | Testing / Quality Assurance                |
| Severity       | MEDIUM                                     |
| Affected Layer | L4 (Service), L5 (Component), L8 (Store)   |
| Source Files   | `backend/api/v1/`, `backend/store/`        |

---

## Mô tả

Core service layer (~1MB+ code) có unit test coverage thấp. Integration tests phụ thuộc testcontainers — chậm và khó chạy trong CI.

## Chi tiết

### Service Layer — Minimal unit tests

- `backend/api/v1/` chứa 79 files, ~1MB+ code.
- Unit test files chủ yếu cho converter/filter logic, không cho service handlers.
- auth_service.go (78KB) — **không có unit test file** tương ứng.
- sql_service.go (77KB) — không có unit test cho query execution paths.

### Store Layer — Partial coverage

```
store/changelog_test.go          — 14 bytes (empty test file!)
store/predefined_roles_test.go   — 491 bytes (minimal)
store/common_test.go             — 886 bytes (basic)
```

- `changelog_test.go` là **file test rỗng** — critical change tracking không có tests.
- Nhiều store files không có test file tương ứng.

### Integration Tests — Heavy dependency

- `backend/tests/` — 42 test files sử dụng testcontainers.
- Yêu cầu Docker runtime → không chạy được trên tất cả CI environments.
- Slow execution — PostgreSQL container startup.

### Component Layer — Limited testing

- `component/iam/manager_test.go` — 2.7KB (basic check function tests).
- Bus, webhook, masker, export — không thấy test files.

## Impact

- **Regression risk** — code changes trong service layer không bị catch sớm.
- **Refactoring fear** — developers tránh refactor vì thiếu safety net.
- **CI reliability** — integration tests flaky do container dependency.

## Khuyến nghị

1. Add unit tests cho auth_service critical paths (login, SSO, MFA).
2. Implement mock-based service tests không cần testcontainers.
3. Fix empty test files (changelog_test.go).
4. Target ≥60% coverage cho service and store layers.
