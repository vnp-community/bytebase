# TASK-AI-002-1: Mock Generation from store/interfaces.go

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~10 LoC generate directive, ~5K auto-generated) |

## Objective

Generate mock implementations for all 18 store interfaces using `go.uber.org/mock/mockgen`. This is the foundation for all DI migration tasks.

## Files

| Action | Path |
|--------|------|
| VERIFY | `backend/store/mock/generate.go` — confirm `//go:generate` directive exists |
| GENERATE | `backend/store/mock/mock_store.go` — auto-generated ~5K LOC |

## Specification

```bash
# 1. Install mockgen (if not present)
go install go.uber.org/mock/mockgen@latest

# 2. Verify generate.go directive
cat backend/store/mock/generate.go
# Expected: //go:generate mockgen -source=../interfaces.go -destination=mock_store.go -package=mock

# 3. Generate
cd backend && go generate ./store/mock/...

# 4. Verify compilation
go build ./store/mock/...
```

If `generate.go` doesn't exist, create:

```go
package mock

//go:generate mockgen -source=../interfaces.go -destination=mock_store.go -package=mock
```

### Verification

```bash
go build ./backend/store/mock/...
# Verify all 18 interfaces have mock types
grep -c "type Mock" backend/store/mock/mock_store.go
```

## Acceptance Criteria

- [ ] `mock_store.go` generated successfully
- [ ] Contains MockUserStore, MockProjectReader, MockDataStore, etc.
- [ ] `go build ./backend/store/mock/...` passes
- [ ] No manual edits to generated file
