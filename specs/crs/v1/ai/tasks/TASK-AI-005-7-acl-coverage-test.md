# TASK-AI-005-7: ACL Coverage Test + ACL_CONTRACT.md

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P0 |
| Depends On | TASK-AI-005-6 |
| Est. | S (~50 LoC test + documentation) |

## Objective

Create a CI test that verifies the ACL extractor map covers ALL registered gRPC methods. Create `ACL_CONTRACT.md` documentation.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/api/v1/acl_extractors_test.go` — coverage verification test |
| CREATE | `backend/api/v1/ACL_CONTRACT.md` — security contract documentation |

## Specification

### Coverage test

```go
func TestACLExtractors_CoverAllMethods(t *testing.T) {
    registeredMethods := getRegisteredMethods() // from grpc_routes.go
    for _, method := range registeredMethods {
        if _, ok := aclResourceExtractors[method]; !ok {
            t.Errorf("method %s has no ACL resource extractor", method)
        }
    }
}
```

This test **FAILS** if a developer adds a new RPC method but forgets to add an ACL extractor entry.

### ACL_CONTRACT.md

Document:
1. Security model (two-level permission)
2. Extraction patterns table
3. Steps for adding new RPC methods
4. Fail-closed behavior

### Verification

```bash
go test ./backend/api/v1/... -run TestACLExtractors -v -count=1
```

## Acceptance Criteria

- [ ] Coverage test passes (all methods covered)
- [ ] Test would fail if a new method is added without extractor
- [ ] ACL_CONTRACT.md documents the security model
- [ ] Steps for adding new RPC methods clearly documented
