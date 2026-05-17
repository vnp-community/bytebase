package v1

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/bytebase/bytebase/backend/component/config"
)

// NewStandbyInterceptor creates an interceptor that blocks write requests on standby nodes.
func NewStandbyInterceptor(profile *config.Profile) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if profile.IsStandby() {
				if !isReadOnly(req.Spec().Procedure) {
					return nil, connect.NewError(connect.CodeUnavailable, 
						connect.NewError(connect.CodeUnavailable, nil))
				}
			}
			return next(ctx, req)
		}
	}
}

func isReadOnly(procedure string) bool {
	// Simple check based on typical gRPC procedure naming.
	// Allow Get, List, Search. Block Create, Update, Delete, Set.
	procParts := strings.Split(procedure, "/")
	if len(procParts) < 3 {
		return true // Unknown format, let it pass or block? Let's allow and let the handler decide.
	}
	method := procParts[2] // e.g., /v1.DatabaseService/GetDatabase -> GetDatabase

	if strings.HasPrefix(method, "Get") || 
	   strings.HasPrefix(method, "List") || 
	   strings.HasPrefix(method, "Search") ||
	   strings.HasPrefix(method, "Stream") {
		return true
	}
	
	// Assume any other method is a write.
	return false
}
