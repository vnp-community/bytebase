# T-024: Interceptor — Rate Limit

| Field | Value |
|-------|-------|
| **Task ID** | T-024 |
| **Solution** | SOL-PERF-004 |
| **Type** | New file + Edit existing |
| **Priority** | P0 |
| **Depends on** | T-022 |
| **Blocks** | None |

## Target Files

1. `backend/api/v1/ratelimit_interceptor.go` (new)
2. `backend/server/grpc_routes.go` (edit — add to interceptor chain)

## File 1: Interceptor Implementation

```go
package v1

import (
    "context"
    "connectrpc.com/connect"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "github.com/bytebase/bytebase/backend/api/auth"
    "github.com/bytebase/bytebase/backend/component/ratelimit"
)

type RateLimitInterceptor struct {
    limiter *ratelimit.WorkspaceLimiter
}

func NewRateLimitInterceptor(limiter *ratelimit.WorkspaceLimiter) *RateLimitInterceptor {
    return &RateLimitInterceptor{limiter: limiter}
}

func (i *RateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
    return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
        workspace := auth.GetWorkspaceFromContext(ctx)
        if workspace == "" {
            return next(ctx, req)
        }
        isWrite := isWriteMethod(req.Spec().Procedure)
        if !i.limiter.Allow(workspace, isWrite) {
            return nil, status.Errorf(codes.ResourceExhausted,
                "rate limit exceeded for workspace %q", workspace)
        }
        return next(ctx, req)
    }
}

func (i *RateLimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
    return next
}
func (i *RateLimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
    return next
}

func isWriteMethod(procedure string) bool {
    // Heuristic: methods containing Create/Update/Delete/Batch are writes
    for _, prefix := range []string{"Create", "Update", "Delete", "Batch"} {
        if strings.Contains(procedure, prefix) { return true }
    }
    return false
}
```

## File 2: Add to interceptor chain

In `grpc_routes.go`, find the interceptor list and add after `authInterceptor`:

```go
interceptors := connect.WithInterceptors(
    validate.NewInterceptor(),
    s.authInterceptor,
    s.rateLimitInterceptor,    // NEW
    s.aclInterceptor,
    s.auditInterceptor,
)
```
