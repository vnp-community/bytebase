# TASK-401: Architecture Boundary Tests

| Field | Value |
|-------|-------|
| Task ID | TASK-401 |
| Phase | 4 — Cleanup |
| Dependencies | TASK-305 |
| Status | ✅ DONE |

## Objective

Add Go tests enforcing no cross-service imports.

## File: `backend/service/architecture_test.go`

```go
func TestNoCrossServiceImports(t *testing.T) {
    // Verify dcm does not import sqlsvc or admin, etc.
    pairs := [][2]string{
        {"dcm", "sqlsvc"}, {"dcm", "admin"},
        {"sqlsvc", "dcm"}, {"sqlsvc", "admin"},
        {"admin", "dcm"}, {"admin", "sqlsvc"},
    }
    for _, pair := range pairs {
        assertNoImport(t, pair[0], pair[1])
    }
}
```

## Acceptance Criteria

- [ ] Test file created and passes
- [ ] Catches violations if added
