# TASK-AI-005-6: ACL Interceptor Integration + Fail-Closed

| Field | Value |
|-------|-------|
| Solution | SOL-AI-005 |
| Priority | P0 |
| Depends On | TASK-AI-005-5 |
| Status | ✅ DONE |
| Completed | 2026-05-10 |
| Verified | 2026-05-11 |
| Est. | M (modify acl.go to use static map) |

## Delivered

### Changes

| File | Description |
|------|-------------|
| `backend/api/v1/acl.go` | Replaced `getResourceFromSingleRequest()` reflection chain with `lookupExtractor()` static map lookup |
| `backend/api/v1/acl.go` | Removed `toSnakeCase`, `matchFirstCap`, `matchAllCap` (dead code after migration) |
| `backend/api/v1/acl.go` | Removed `annotationsproto` import (unused) |
| `backend/api/v1/acl.go` | Added `extractBatchGetNames()` and `extractBatchSubRequests()` for clean batch handling |
| `backend/api/v1/acl_test.go` | Updated tests, removed toSnakeCase test, added TestLookupExtractor |

### Security Improvements

- **Fail-closed**: Unknown methods fall back to workspace-level permissions with a warning log
- **Deterministic**: No more runtime proto reflection probing for resource extraction
- **Observable**: `slog.Warn` emitted for any method not in the static registry

### Verification (2026-05-11 re-verified)

```bash
go build ./backend/api/v1/...                                # ✅ PASS
go vet ./backend/api/v1/...                                  # ✅ PASS
go test -run TestGetResourceFromRequest -count=1             # ✅ PASS
go test -run TestLookupExtractor -count=1                    # ✅ PASS
go test -run TestHasAllowMissing -count=1                    # ✅ PASS
```

## Acceptance Criteria

- [x] Reflection-based extraction replaced with static map
- [x] Fail-closed workspace fallback for unknown methods
- [x] Dead code removed (toSnakeCase, regex matchers)
- [x] All ACL tests pass
