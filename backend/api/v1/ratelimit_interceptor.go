package v1

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/ratelimit"
)

// RateLimitInterceptor is a Connect interceptor that enforces per-workspace
// rate limits on incoming requests.
type RateLimitInterceptor struct {
	limiter *ratelimit.WorkspaceLimiter
}

// NewRateLimitInterceptor creates a new rate limit interceptor.
func NewRateLimitInterceptor(limiter *ratelimit.WorkspaceLimiter) *RateLimitInterceptor {
	return &RateLimitInterceptor{limiter: limiter}
}

// WrapUnary implements connect.Interceptor for unary RPCs.
func (i *RateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		workspace := common.GetWorkspaceIDFromContext(ctx)
		if workspace == "" {
			return next(ctx, req)
		}
		isWrite := isWriteMethod(req.Spec().Procedure)
		if !i.limiter.Allow(workspace, isWrite) {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				errors.Errorf("rate limit exceeded for workspace %q", workspace))
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor (no-op for server-side).
func (i *RateLimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor (no-op for streaming).
func (i *RateLimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// isWriteMethod heuristically identifies write operations by checking
// for mutation-related keywords in the procedure name.
func isWriteMethod(procedure string) bool {
	for _, prefix := range []string{"Create", "Update", "Delete", "Batch", "Sync"} {
		if strings.Contains(procedure, prefix) {
			return true
		}
	}
	return false
}
