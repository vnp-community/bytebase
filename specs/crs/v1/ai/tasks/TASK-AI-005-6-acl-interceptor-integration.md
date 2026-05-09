# TASK-AI-005-6: ACL Interceptor Integration + Fail-Closed

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P0 |
| Depends On | TASK-AI-005-5 |
| Est. | M (modify acl.go to use static map) |

## Objective

Replace the reflection-based `getResourceFromSingleRequest()` in `acl.go` with a static map lookup. Fail-closed for unknown methods.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/acl.go` — replace resource extraction logic |

## Specification

### Replace extraction logic

```go
func (in *ACLInterceptor) getResourcesFromRequest(
    ctx context.Context,
    fullMethod string,
    req proto.Message,
) ([]string, error) {
    extractor, ok := aclResourceExtractors[fullMethod]
    if !ok {
        // SECURITY: fail-closed — unknown method = error, not skip
        return nil, status.Errorf(codes.Internal,
            "ACL: no resource extractor registered for method %s", fullMethod)
    }
    return extractor(req)
}
```

### Remove old reflection-based probing

Delete/deprecate `getResourceFromSingleRequest()` and the HACK-comment helper functions.

### Verification

```bash
go build ./backend/api/v1/...
go test ./backend/api/v1/... -run TestACL -count=1
go test ./backend/tests/... -count=1  # Full integration
```

## Acceptance Criteria

- [ ] `acl.go` uses static map lookup instead of reflection
- [ ] Unknown methods return `CodeInternal` error
- [ ] All integration tests pass (ACL behavior identical)
- [ ] HACK comments removed
