package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/grpcreflect"
	grpcruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/labstack/echo/v5"

	"github.com/bytebase/bytebase/backend/component/config"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/service"
	"github.com/bytebase/bytebase/backend/transport"

	"connectrpc.com/connect"
)

// RESTGateway handles gRPC-Gateway REST proxy registration.
// It preserves the existing REST API surface (/v1/*) while routing
// ConnectRPC requests through the gateway reverse proxy.
type RESTGateway struct {
	mux  *grpcruntime.ServeMux
	conn *grpc.ClientConn
}

// NewRESTGateway creates a REST gateway proxy.
func NewRESTGateway(profile *config.Profile) (*RESTGateway, error) {
	mux := grpcruntime.NewServeMux(
		grpcruntime.WithMarshalerOption(grpcruntime.MIMEWildcard, &grpcruntime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{},
			//nolint:forbidigo
			UnmarshalOptions: protojson.UnmarshalOptions{},
		}),
		grpcruntime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			switch strings.ToLower(key) {
			case "authorization", "cookie", "origin":
				return key, true
			default:
				return "", false
			}
		}),
		grpcruntime.WithOutgoingHeaderMatcher(func(key string) (string, bool) {
			switch strings.ToLower(key) {
			case "set-cookie":
				return key, true
			default:
				return "", false
			}
		}),
		grpcruntime.WithRoutingErrorHandler(func(ctx context.Context, sm *grpcruntime.ServeMux, m grpcruntime.Marshaler, w http.ResponseWriter, r *http.Request, httpStatus int) {
			err := &grpcruntime.HTTPStatusError{
				HTTPStatus: httpStatus,
				Err:        connect.NewError(connect.CodeNotFound, errors.Errorf("gateway routing error %d: request method %v, URI %v", httpStatus, r.Method, r.RequestURI)),
			}
			grpcruntime.DefaultHTTPErrorHandler(ctx, sm, m, w, r, err)
		}),
	)

	grpcEndpoint := fmt.Sprintf(":%d", profile.Port)
	grpcConn, err := grpc.NewClient(
		grpcEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024),
		),
	)
	if err != nil {
		return nil, err
	}

	return &RESTGateway{mux: mux, conn: grpcConn}, nil
}

// RegisterAllServices registers all 31 service handlers on the REST gateway mux.
func (rg *RESTGateway) RegisterAllServices(ctx context.Context) error {
	registrations := []func(context.Context, *grpcruntime.ServeMux, *grpc.ClientConn) error{
		v1pb.RegisterAIServiceHandler,
		v1pb.RegisterAccessGrantServiceHandler,
		v1pb.RegisterActuatorServiceHandler,
		v1pb.RegisterAuditLogServiceHandler,
		v1pb.RegisterAuthServiceHandler,
		v1pb.RegisterCelServiceHandler,
		v1pb.RegisterDatabaseCatalogServiceHandler,
		v1pb.RegisterDatabaseGroupServiceHandler,
		v1pb.RegisterDatabaseServiceHandler,
		v1pb.RegisterGroupServiceHandler,
		v1pb.RegisterIdentityProviderServiceHandler,
		v1pb.RegisterInstanceRoleServiceHandler,
		v1pb.RegisterInstanceServiceHandler,
		v1pb.RegisterIssueServiceHandler,
		v1pb.RegisterOrgPolicyServiceHandler,
		v1pb.RegisterPlanServiceHandler,
		v1pb.RegisterProjectServiceHandler,
		v1pb.RegisterReleaseServiceHandler,
		v1pb.RegisterReviewConfigServiceHandler,
		v1pb.RegisterRevisionServiceHandler,
		v1pb.RegisterRoleServiceHandler,
		v1pb.RegisterRolloutServiceHandler,
		v1pb.RegisterSettingServiceHandler,
		v1pb.RegisterSheetServiceHandler,
		v1pb.RegisterSQLServiceHandler,
		v1pb.RegisterSubscriptionServiceHandler,
		v1pb.RegisterUserServiceHandler,
		v1pb.RegisterServiceAccountServiceHandler,
		v1pb.RegisterWorkloadIdentityServiceHandler,
		v1pb.RegisterWorksheetServiceHandler,
		v1pb.RegisterWorkspaceServiceHandler,
	}

	for _, reg := range registrations {
		if err := reg(ctx, rg.mux, rg.conn); err != nil {
			return err
		}
	}
	return nil
}

// Mux returns the REST gateway mux for registration with echo.
func (rg *RESTGateway) Mux() *grpcruntime.ServeMux {
	return rg.mux
}

// RegisterGatewayRoutes registers the full gateway routing on an echo instance.
// This replaces the monolithic configureGrpcRouters function.
func RegisterGatewayRoutes(
	ctx context.Context,
	e *echo.Echo,
	registry *service.Registry,
	handlerOpts connect.HandlerOption,
	profile *config.Profile,
) error {
	// 1. Create ConnectRPC handlers on each domain service.
	// (Already done in domain service constructors during NewService())

	// 2. Build ConnectRPC route map for direct proxying via bufconn.
	gw := NewGateway(registry)

	// 3. Create reflection handlers for all 31 services.
	reflector := grpcreflect.NewStaticReflector(
		v1connect.AIServiceName,
		v1connect.AccessGrantServiceName,
		v1connect.ActuatorServiceName,
		v1connect.AuditLogServiceName,
		v1connect.AuthServiceName,
		v1connect.CelServiceName,
		v1connect.DatabaseCatalogServiceName,
		v1connect.DatabaseGroupServiceName,
		v1connect.DatabaseServiceName,
		v1connect.GroupServiceName,
		v1connect.IdentityProviderServiceName,
		v1connect.InstanceRoleServiceName,
		v1connect.InstanceServiceName,
		v1connect.IssueServiceName,
		v1connect.OrgPolicyServiceName,
		v1connect.PlanServiceName,
		v1connect.ProjectServiceName,
		v1connect.ReleaseServiceName,
		v1connect.ReviewConfigServiceName,
		v1connect.RevisionServiceName,
		v1connect.RoleServiceName,
		v1connect.RolloutServiceName,
		v1connect.SettingServiceName,
		v1connect.ServiceAccountServiceName,
		v1connect.SheetServiceName,
		v1connect.SQLServiceName,
		v1connect.SubscriptionServiceName,
		v1connect.UserServiceName,
		v1connect.WorkloadIdentityServiceName,
		v1connect.WorksheetServiceName,
		v1connect.WorkspaceServiceName,
	)
	reflectPath, reflectHandler := grpcreflect.NewHandlerV1(reflector)
	reflectAlphaPath, reflectAlphaHandler := grpcreflect.NewHandlerV1Alpha(reflector)

	// 4. REST gateway.
	restGW, err := NewRESTGateway(profile)
	if err != nil {
		return errors.Wrap(err, "failed to create REST gateway")
	}
	if err := restGW.RegisterAllServices(ctx); err != nil {
		return errors.Wrap(err, "failed to register REST gateway services")
	}

	// 5. Register echo routes.
	// Websocket proxy for admin execute.
	e.GET("/v1:adminExecute", echo.WrapHandler(wsproxy.WebsocketProxy(
		restGW.Mux(),
		wsproxy.WithTokenCookieName("access-token"),
		wsproxy.WithMaxRespBodyBufferSize(100*1024*1024),
	)))
	// REST gateway.
	e.Any("/v1/*", echo.WrapHandler(restGW.Mux()))

	// ConnectRPC gateway proxy — routes to domain services via bufconn.
	e.Any("/bytebase.v1.*", echo.WrapHandler(gw.ConnectHandler()))

	// Reflection handlers.
	e.Any(reflectPath+"*", echo.WrapHandler(reflectHandler))
	e.Any(reflectAlphaPath+"*", echo.WrapHandler(reflectAlphaHandler))

	slog.Info("Gateway routes registered",
		"connectServices", len(gw.connectRoutes),
		"restServices", 31,
	)

	return nil
}

// Ensure transport package is used (suppress unused import in some builds).
var _ = transport.NewBufconnTransport
