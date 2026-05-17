// Package gateway implements the HTTP reverse proxy gateway.
// It routes incoming requests to internal domain services via bufconn HTTP transport.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"

	"connectrpc.com/grpcreflect"

	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/service"
)

// Gateway routes external HTTP requests to internal domain services.
type Gateway struct {
	registry *service.Registry

	// Per-service HTTP reverse proxies (via bufconn).
	dcmProxy   *httputil.ReverseProxy
	sqlProxy   *httputil.ReverseProxy
	adminProxy *httputil.ReverseProxy

	// ConnectRPC path → proxy mapping.
	connectRoutes map[string]*httputil.ReverseProxy

	// Reflection handler.
	reflectHandler      http.Handler
	reflectAlphaHandler http.Handler
}

// NewGateway creates a gateway that routes to services via bufconn.
func NewGateway(registry *service.Registry) *Gateway {
	g := &Gateway{
		registry:      registry,
		connectRoutes: make(map[string]*httputil.ReverseProxy),
	}

	// Create per-service reverse proxies.
	dcmSvc, _ := registry.Get("dcm")
	sqlSvc, _ := registry.Get("sql")
	adminSvc, _ := registry.Get("admin")

	if dcmSvc != nil {
		g.dcmProxy = newBufconnProxy(dcmSvc)
	}
	if sqlSvc != nil {
		g.sqlProxy = newBufconnProxy(sqlSvc)
	}
	if adminSvc != nil {
		g.adminProxy = newBufconnProxy(adminSvc)
	}

	g.buildConnectRouteMap()
	g.buildReflectionHandlers()

	return g
}

// buildConnectRouteMap maps ConnectRPC service paths to proxies.
func (g *Gateway) buildConnectRouteMap() {
	// DCM services (8).
	dcmServices := []string{
		v1connect.PlanServiceName,
		v1connect.IssueServiceName,
		v1connect.RolloutServiceName,
		v1connect.ReleaseServiceName,
		v1connect.RevisionServiceName,
		v1connect.ReviewConfigServiceName,
		v1connect.AccessGrantServiceName,
		v1connect.OrgPolicyServiceName,
	}
	for _, name := range dcmServices {
		g.connectRoutes["/"+name+"/"] = g.dcmProxy
	}

	// SQL services (8).
	sqlServices := []string{
		v1connect.SQLServiceName,
		v1connect.DatabaseServiceName,
		v1connect.DatabaseCatalogServiceName,
		v1connect.DatabaseGroupServiceName,
		v1connect.InstanceServiceName,
		v1connect.InstanceRoleServiceName,
		v1connect.SheetServiceName,
		v1connect.WorksheetServiceName,
	}
	for _, name := range sqlServices {
		g.connectRoutes["/"+name+"/"] = g.sqlProxy
	}

	// Admin services (15).
	adminServices := []string{
		v1connect.AuthServiceName,
		v1connect.UserServiceName,
		v1connect.ServiceAccountServiceName,
		v1connect.WorkloadIdentityServiceName,
		v1connect.RoleServiceName,
		v1connect.GroupServiceName,
		v1connect.IdentityProviderServiceName,
		v1connect.SettingServiceName,
		v1connect.WorkspaceServiceName,
		v1connect.ProjectServiceName,
		v1connect.SubscriptionServiceName,
		v1connect.ActuatorServiceName,
		v1connect.AuditLogServiceName,
		v1connect.CelServiceName,
		v1connect.AIServiceName,
	}
	for _, name := range adminServices {
		g.connectRoutes["/"+name+"/"] = g.adminProxy
	}
}

// buildReflectionHandlers creates gRPC reflection handlers for all 31 services.
func (g *Gateway) buildReflectionHandlers() {
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
	_, g.reflectHandler = grpcreflect.NewHandlerV1(reflector)
	_, g.reflectAlphaHandler = grpcreflect.NewHandlerV1Alpha(reflector)
}

// ServeConnectRPC handles a ConnectRPC request by routing to the correct service proxy.
// Returns true if the request was handled, false if no route matched.
func (g *Gateway) ServeConnectRPC(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path

	// Check reflection paths first.
	if strings.HasPrefix(path, "/grpc.reflection.") {
		g.reflectHandler.ServeHTTP(w, r)
		return true
	}

	// Find matching service proxy by longest prefix.
	for prefix, proxy := range g.connectRoutes {
		if strings.HasPrefix(path, prefix) {
			proxy.ServeHTTP(w, r)
			return true
		}
	}
	return false
}

// ConnectHandler returns an http.Handler that routes ConnectRPC requests.
func (g *Gateway) ConnectHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.ServeConnectRPC(w, r) {
			http.Error(w, "service not found", http.StatusNotFound)
		}
	})
}

// HealthCheckAll runs health checks across all services.
func (g *Gateway) HealthCheckAll(ctx context.Context) map[string]error {
	return g.registry.HealthCheckAll(ctx)
}

// newBufconnProxy creates a reverse proxy that forwards to a service via bufconn.
func newBufconnProxy(svc service.DomainService) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "bufconn"
		},
		Transport: svc.HTTPClient().Transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("gateway proxy error",
				"service", svc.Name(),
				"path", r.URL.Path,
				"err", err,
			)
			http.Error(w, fmt.Sprintf("service %s unavailable: %v", svc.Name(), err), http.StatusBadGateway)
		},
	}
}
