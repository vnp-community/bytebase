package gateway

import (
	"context"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/bytebase/bytebase/backend/api/auth"
	apiv1 "github.com/bytebase/bytebase/backend/api/v1"
	"github.com/bytebase/bytebase/backend/common/stacktrace"
	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/ratelimit"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/store"

	bblog "github.com/bytebase/bytebase/backend/common/log"
	"github.com/pkg/errors"
)

// InterceptorDeps holds dependencies for building the ConnectRPC interceptor chain.
type InterceptorDeps struct {
	Store          *store.Store
	Secret         string
	LicenseService *enterprise.LicenseService
	Bus            bus.EventBus
	Profile        *config.Profile
	IAMManager     *iam.Manager
}

// BuildInterceptorChain creates the interceptor chain in the EXACT same order
// as grpc_routes.go lines 143-161. Order is critical for correct auth flow.
func BuildInterceptorChain(deps *InterceptorDeps) []connect.Interceptor {
	return []connect.Interceptor{
		validate.NewInterceptor(),
		auth.New(deps.Store, deps.Secret, deps.LicenseService, deps.Bus, deps.Profile),
		apiv1.NewRateLimitInterceptor(ratelimit.New(ratelimit.DefaultConfig)),
		apiv1.NewACLInterceptor(deps.Store, deps.Secret, deps.IAMManager, deps.Profile),
		apiv1.NewAuditInterceptor(deps.Store, deps.Secret, deps.Profile),
		apiv1.NewStandbyInterceptor(deps.Profile),
	}
}

// BuildHandlerOptions creates connect.HandlerOption with interceptors and panic recovery.
func BuildHandlerOptions(deps *InterceptorDeps) connect.HandlerOption {
	onPanic := func(_ context.Context, s connect.Spec, _ http.Header, p any) error {
		stack := stacktrace.TakeStacktrace(20, 5)
		slog.Error("v1 server panic error", "method", s.Procedure, bblog.BBError(errors.Errorf("error: %v\n%s", p, stack)))
		return connect.NewError(connect.CodeInternal, errors.Errorf("error: %v\n%s", p, stack))
	}

	return connect.WithHandlerOptions(
		connect.WithInterceptors(BuildInterceptorChain(deps)...),
		connect.WithRecover(onPanic),
	)
}
