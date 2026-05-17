package server

// This file provides the gateway-mode gRPC router configuration.
// When BB_USE_GATEWAY=true, this replaces the monolithic configureGrpcRouters()
// with the gateway reverse proxy that routes to domain services via bufconn.

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v5"

	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/sampleinstance"
	"github.com/bytebase/bytebase/backend/component/sheet"
	"github.com/bytebase/bytebase/backend/component/webhook"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/gateway"
	"github.com/bytebase/bytebase/backend/service"
	"github.com/bytebase/bytebase/backend/service/admin"
	"github.com/bytebase/bytebase/backend/service/dcm"
	runnerservice "github.com/bytebase/bytebase/backend/service/runner"
	"github.com/bytebase/bytebase/backend/service/sqlsvc"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/component/health"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
)

// GatewayDeps holds all dependencies needed to initialize the gateway-mode architecture.
type GatewayDeps struct {
	Store                 *store.Store
	SheetManager          *sheet.Manager
	DBFactory             *dbfactory.DBFactory
	LicenseService        *enterprise.LicenseService
	Profile               *config.Profile
	Bus                   bus.EventBus
	SchemaSyncer          *schemasync.Syncer
	WebhookManager        *webhook.Manager
	IAMManager            *iam.Manager
	Secret                string
	SampleInstanceManager *sampleinstance.Manager
	HealthChecker         *health.Checker
}

// GatewayResult contains the constructed gateway-mode components.
type GatewayResult struct {
	Registry      *service.Registry
	RunnerService *runnerservice.Service
}

// initGatewayServices creates and starts all domain services + runner service
// using the gateway architecture. Returns the registry for routing and the
// runner service for lifecycle management.
func initGatewayServices(deps *GatewayDeps) *GatewayResult {
	reg := service.NewRegistry()

	// 1. Create domain services (same constructors as grpc_routes.go).
	dcmSvc := dcm.NewService(&dcm.Deps{
		Store:          deps.Store,
		SheetManager:   deps.SheetManager,
		DBFactory:      deps.DBFactory,
		LicenseService: deps.LicenseService,
		Profile:        deps.Profile,
		Bus:            deps.Bus,
		WebhookManager: deps.WebhookManager,
		IAMManager:     deps.IAMManager,
		Secret:         deps.Secret,
	})
	reg.Register(dcmSvc)

	sqlSvc := sqlsvc.NewService(&sqlsvc.Deps{
		Store:                 deps.Store,
		SchemaSyncer:          deps.SchemaSyncer,
		DBFactory:             deps.DBFactory,
		LicenseService:        deps.LicenseService,
		Profile:               deps.Profile,
		IAMManager:            deps.IAMManager,
		Bus:                   deps.Bus,
		Secret:                deps.Secret,
		SampleInstanceManager: deps.SampleInstanceManager,
	})
	reg.Register(sqlSvc)

	adminSvc := admin.NewService(&admin.Deps{
		Store:                 deps.Store,
		LicenseService:        deps.LicenseService,
		Profile:               deps.Profile,
		Bus:                   deps.Bus,
		IAMManager:            deps.IAMManager,
		Secret:                deps.Secret,
		SchemaSyncer:          deps.SchemaSyncer,
		WebhookManager:        deps.WebhookManager,
		SampleInstanceManager: deps.SampleInstanceManager,
	})
	reg.Register(adminSvc)

	// 2. Create runner service.
	runnerSvc := runnerservice.NewService(&runnerservice.Deps{
		Store:          deps.Store,
		Bus:            deps.Bus,
		DBFactory:      deps.DBFactory,
		LicenseService: deps.LicenseService,
		Profile:        deps.Profile,
		SheetManager:   deps.SheetManager,
		WebhookManager: deps.WebhookManager,
		HealthChecker:  deps.HealthChecker,
	})
	reg.SetRunner(runnerSvc)

	slog.Info("Gateway services initialized",
		"domainServices", len(reg.AllDomainServices()),
	)

	return &GatewayResult{
		Registry:      reg,
		RunnerService: runnerSvc,
	}
}

// configureGatewayRouters sets up the echo routes using the gateway reverse proxy.
// This is the gateway-mode replacement for configureGrpcRouters.
func configureGatewayRouters(
	ctx context.Context,
	e *echo.Echo,
	registry *service.Registry,
	deps *GatewayDeps,
) error {
	handlerOpts := gateway.BuildHandlerOptions(&gateway.InterceptorDeps{
		Store:          deps.Store,
		Secret:         deps.Secret,
		LicenseService: deps.LicenseService,
		Bus:            deps.Bus,
		Profile:        deps.Profile,
		IAMManager:     deps.IAMManager,
	})

	return gateway.RegisterGatewayRoutes(ctx, e, registry, handlerOpts, deps.Profile)
}
