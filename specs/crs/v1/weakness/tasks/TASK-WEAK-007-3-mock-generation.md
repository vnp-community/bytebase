# TASK-WEAK-007-3: Mock Generation Setup

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P0 |
| Depends On | TASK-WEAK-007-1, TASK-WEAK-007-2 |
| Est. | S (~40 LoC) |

## Objective

Configure `go generate` with `mockgen` to auto-generate mocks from extracted interfaces.

## Files

| Action | Path |
|--------|------|
| ADD | `//go:generate` directives in `backend/store/interfaces.go` |
| ADD | `//go:generate` directives in `backend/component/iam/interfaces.go` |
| ADD | `//go:generate` directives in `backend/enterprise/interfaces.go` |
| CREATE | `backend/testutil/mocks/` (generated directory) |

## Specification

Add to each interface file:
```go
//go:generate mockgen -source=interfaces.go -destination=../../testutil/mocks/mock_store.go -package=mocks
```

Then run:
```bash
go install go.uber.org/mock/mockgen@latest
go generate ./backend/store/... ./backend/component/iam/... ./backend/enterprise/...
```

Generated files: `mock_store.go`, `mock_iam.go`, `mock_enterprise.go`

## Acceptance Criteria

- [ ] `go generate` produces mock files without errors
- [ ] Generated mocks compile
- [ ] Mocks cover all extracted interfaces (UserReader, UserWriter, etc.)
- [ ] `go.mod` updated with `go.uber.org/mock` dependency
