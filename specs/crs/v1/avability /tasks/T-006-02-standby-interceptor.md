# T-006-02: Standby Read-Only Interceptor

| Field | Value |
|---|---|
| **Task ID** | T-006-02 |
| **Solution** | SOL-AVAIL-006 |
| **Depends On** | T-006-01 |
| **Target Files** | `backend/api/v1/standby_interceptor.go` (NEW), `backend/server/echo_routes.go` (Modify) |

---

## Objective

Tạo ConnectRPC interceptor reject writes on standby + Echo middleware redirect HTTP writes.

## Implementation

### 1. `standby_interceptor.go` — xem SOL-AVAIL-006 §2.2

- `StandbyInterceptor` implements `connect.UnaryInterceptorFunc`
- `WrapUnary`: if standby → check `isReadOnly(procedure)`
- `isReadOnly`: parse procedure → allow Get/List/Search, block Create/Update/Delete/Set
- Read-only services whitelist: DatabaseService, ProjectService, InstanceService, AuditLogService, SQLService

### 2. Echo middleware in `echo_routes.go` — xem SOL-AVAIL-006 §2.3

```go
func standbyRedirectMiddleware(profile *config.Profile) echo.MiddlewareFunc
```
- Pass GET/HEAD/OPTIONS
- Redirect POST/PUT/DELETE → primary with JSON response + `X-Bytebase-Primary` header

### 3. Wire: Add standby interceptor to gRPC interceptor chain in `grpc_routes.go`

## Acceptance Criteria

- [ ] ConnectRPC interceptor blocks writes on standby
- [ ] Echo middleware redirects HTTP writes to primary
- [ ] Returns `codes.Unavailable` with primary URL
- [ ] No-op when `IsPrimary()` (default)
- [ ] `go build ./backend/...` passes
