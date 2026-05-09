# TASK-WEAK-007-4: Auth + Changelog Unit Tests

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P1 |
| Depends On | TASK-WEAK-007-3 |
| Est. | M (~200 LoC) |

## Objective

Write unit tests for AuthService (Login, Signup) and fix empty `changelog_test.go`. First real usage of extracted interfaces + mocks.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/api/v1/auth_service_test.go` |
| MODIFY | `backend/store/changelog_test.go` — replace empty file |

## Specification

### `auth_service_test.go` (4+ test cases)

Using `gomock` + extracted interfaces:
- `TestLogin_Success` — mock returns valid user → JWT generated
- `TestLogin_WrongPassword` — mock returns user → password mismatch → error
- `TestLogin_DisabledUser` — mock returns disabled user → error
- `TestLogin_StoreError` — mock returns error → service returns 503

```go
func TestLogin_Success(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockStore := mocks.NewMockUserReader(ctrl)
    mockStore.EXPECT().GetUserByEmail(gomock.Any(), "ws-1", "admin@example.com").Return(...)
    svc := &AuthService{userReader: mockStore}
    // ...
}
```

### `changelog_test.go` (3+ test cases)

- `TestCreateChangelog_Validation` — nil fields → error
- `TestUpdateChangelog_StatusTransition` — valid PENDING→DONE
- `TestChangelogMessage_ProjectRequired` — composite PK validation

## Acceptance Criteria

- [ ] `auth_service_test.go` has ≥4 test cases using mocks
- [ ] `changelog_test.go` has ≥3 test cases (no longer empty)
- [ ] All tests pass: `go test ./backend/api/v1/... ./backend/store/...`
- [ ] No database required for these tests
