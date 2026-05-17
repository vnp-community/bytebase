// Package sqlsvc implements the SQL & Database domain service.
// Package name is sqlsvc to avoid conflict with database/sql stdlib.
package sqlsvc

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/bytebase/bytebase/backend/api/auth"
	apiv1 "github.com/bytebase/bytebase/backend/api/v1"
	"github.com/bytebase/bytebase/backend/common/stacktrace"
	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/ratelimit"
	"github.com/bytebase/bytebase/backend/component/sampleinstance"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/transport"

	bblog "github.com/bytebase/bytebase/backend/common/log"
	"github.com/pkg/errors"
)

// Deps holds all dependencies needed by the SQL service.
type Deps struct {
	Store                 *store.Store
	SchemaSyncer          *schemasync.Syncer
	DBFactory             *dbfactory.DBFactory
	LicenseService        *enterprise.LicenseService
	Profile               *config.Profile
	IAMManager            *iam.Manager
	Bus                   bus.EventBus
	Secret                string
	SampleInstanceManager *sampleinstance.Manager
}

// Service is the SQL domain service.
type Service struct {
	name      string
	transport *transport.BufconnTransport
	server    *http.Server
}

// NewService creates the SQL domain service with all handlers registered.
func NewService(deps *Deps) *Service {
	tr := transport.NewBufconnTransport()
	mux := http.NewServeMux()

	onPanic := func(_ context.Context, s connect.Spec, _ http.Header, p any) error {
		stack := stacktrace.TakeStacktrace(20, 5)
		slog.Error("sql service panic", "method", s.Procedure, bblog.BBError(errors.Errorf("error: %v\n%s", p, stack)))
		return connect.NewError(connect.CodeInternal, errors.Errorf("error: %v\n%s", p, stack))
	}
	handlerOpts := connect.WithHandlerOptions(
		connect.WithInterceptors(
			validate.NewInterceptor(),
			auth.New(deps.Store, deps.Secret, deps.LicenseService, deps.Bus, deps.Profile),
			apiv1.NewRateLimitInterceptor(ratelimit.New(ratelimit.DefaultConfig)),
			apiv1.NewACLInterceptor(deps.Store, deps.Secret, deps.IAMManager, deps.Profile),
			apiv1.NewAuditInterceptor(deps.Store, deps.Secret, deps.Profile),
			apiv1.NewStandbyInterceptor(deps.Profile),
		),
		connect.WithRecover(onPanic),
	)

	// 8 SQL sub-services.
	sqlSvc := apiv1.NewSQLService(deps.Store, deps.SchemaSyncer, deps.DBFactory, deps.LicenseService, deps.IAMManager)
	databaseSvc := apiv1.NewDatabaseService(deps.Store, deps.SchemaSyncer, deps.Profile, deps.IAMManager, deps.LicenseService)
	databaseCatalogSvc := apiv1.NewDatabaseCatalogService(deps.Store)
	databaseGroupSvc := apiv1.NewDatabaseGroupService(deps.Store, deps.LicenseService)
	instanceSvc := apiv1.NewInstanceService(deps.Store, deps.Profile, deps.LicenseService, deps.DBFactory, deps.SchemaSyncer, deps.SampleInstanceManager)
	instanceRoleSvc := apiv1.NewInstanceRoleService(deps.Store)
	sheetSvc := apiv1.NewSheetService(deps.Store)
	worksheetSvc := apiv1.NewWorksheetService(deps.Store, deps.IAMManager)

	// Register ConnectRPC handlers.
	sqlPath, sqlHandler := v1connect.NewSQLServiceHandler(sqlSvc, handlerOpts)
	mux.Handle(sqlPath, sqlHandler)
	dbPath, dbHandler := v1connect.NewDatabaseServiceHandler(databaseSvc, handlerOpts)
	mux.Handle(dbPath, dbHandler)
	dbCatalogPath, dbCatalogHandler := v1connect.NewDatabaseCatalogServiceHandler(databaseCatalogSvc, handlerOpts)
	mux.Handle(dbCatalogPath, dbCatalogHandler)
	dbGroupPath, dbGroupHandler := v1connect.NewDatabaseGroupServiceHandler(databaseGroupSvc, handlerOpts)
	mux.Handle(dbGroupPath, dbGroupHandler)
	instPath, instHandler := v1connect.NewInstanceServiceHandler(instanceSvc, handlerOpts)
	mux.Handle(instPath, instHandler)
	instRolePath, instRoleHandler := v1connect.NewInstanceRoleServiceHandler(instanceRoleSvc, handlerOpts)
	mux.Handle(instRolePath, instRoleHandler)
	sheetPath, sheetHandler := v1connect.NewSheetServiceHandler(sheetSvc, handlerOpts)
	mux.Handle(sheetPath, sheetHandler)
	worksheetPath, worksheetHandler := v1connect.NewWorksheetServiceHandler(worksheetSvc, handlerOpts)
	mux.Handle(worksheetPath, worksheetHandler)

	svc := &Service{
		name:      "sql",
		transport: tr,
		server:    &http.Server{Handler: mux},
	}
	return svc
}

func (s *Service) Name() string                    { return s.name }
func (s *Service) Listener() net.Listener           { return s.transport.Listener() }
func (s *Service) HTTPClient() *http.Client         { return s.transport.HTTPClient() }

func (s *Service) Start(_ context.Context) error {
	go func() {
		if err := s.server.Serve(s.transport.Listener()); err != nil && err != http.ErrServerClosed {
			slog.Error("SQL service server error", "err", err)
		}
	}()
	slog.Info("SQL service started")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	slog.Info("SQL service stopping")
	return s.server.Shutdown(ctx)
}

func (s *Service) Healthy(_ context.Context) error {
	return nil
}
