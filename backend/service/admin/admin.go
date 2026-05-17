// Package admin implements the Admin domain service.
// It wraps 15 api/v1 handlers related to auth, user management, settings,
// workspace, project, subscription, and other administrative functions.
package admin

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
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/ratelimit"
	"github.com/bytebase/bytebase/backend/component/sampleinstance"
	"github.com/bytebase/bytebase/backend/component/webhook"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/transport"

	bblog "github.com/bytebase/bytebase/backend/common/log"
	"github.com/pkg/errors"
)

// Deps holds all dependencies needed by the Admin service.
type Deps struct {
	Store                 *store.Store
	LicenseService        *enterprise.LicenseService
	Profile               *config.Profile
	Bus                   bus.EventBus
	IAMManager            *iam.Manager
	Secret                string
	SchemaSyncer          *schemasync.Syncer
	WebhookManager        *webhook.Manager
	SampleInstanceManager *sampleinstance.Manager
}

// Service is the Admin domain service.
type Service struct {
	name      string
	transport *transport.BufconnTransport
	server    *http.Server
}

// NewService creates the Admin domain service with all 15 handlers registered.
func NewService(deps *Deps) *Service {
	tr := transport.NewBufconnTransport()
	mux := http.NewServeMux()

	onPanic := func(_ context.Context, s connect.Spec, _ http.Header, p any) error {
		stack := stacktrace.TakeStacktrace(20, 5)
		slog.Error("admin service panic", "method", s.Procedure, bblog.BBError(errors.Errorf("error: %v\n%s", p, stack)))
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

	// 15 Admin sub-services — exact same constructors as grpc_routes.go.
	authSvc := apiv1.NewAuthService(deps.Store, deps.Secret, deps.LicenseService, deps.Profile, deps.IAMManager)
	userSvc := apiv1.NewUserService(deps.Store, deps.LicenseService, deps.Profile, deps.IAMManager)
	serviceAccountSvc := apiv1.NewServiceAccountService(deps.Store, deps.Profile, deps.IAMManager)
	workloadIdentitySvc := apiv1.NewWorkloadIdentityService(deps.Store, deps.Profile, deps.IAMManager)
	roleSvc := apiv1.NewRoleService(deps.Store, deps.IAMManager, deps.LicenseService)
	groupSvc := apiv1.NewGroupService(deps.Store, deps.IAMManager, deps.LicenseService)
	identityProviderSvc := apiv1.NewIdentityProviderService(deps.Store, deps.LicenseService, deps.Profile)
	settingSvc := apiv1.NewSettingService(deps.Store, deps.Profile, deps.LicenseService, deps.IAMManager)
	workspaceSvc := apiv1.NewWorkspaceService(deps.Store, deps.IAMManager, deps.Profile, deps.LicenseService)
	projectSvc := apiv1.NewProjectService(deps.Store, deps.Profile, deps.IAMManager)
	subscriptionSvc := apiv1.NewSubscriptionService(deps.Profile, deps.Store, deps.LicenseService)
	actuatorSvc := apiv1.NewActuatorService(deps.Store, deps.Profile, deps.SchemaSyncer, deps.LicenseService, deps.SampleInstanceManager)
	auditLogSvc := apiv1.NewAuditLogService(deps.Store, deps.LicenseService)
	celSvc := apiv1.NewCelService()
	aiSvc := apiv1.NewAIService(deps.Store)

	// Register ConnectRPC handlers.
	authPath, authHandler := v1connect.NewAuthServiceHandler(authSvc, handlerOpts)
	mux.Handle(authPath, authHandler)
	userPath, userHandler := v1connect.NewUserServiceHandler(userSvc, handlerOpts)
	mux.Handle(userPath, userHandler)
	saPath, saHandler := v1connect.NewServiceAccountServiceHandler(serviceAccountSvc, handlerOpts)
	mux.Handle(saPath, saHandler)
	wiPath, wiHandler := v1connect.NewWorkloadIdentityServiceHandler(workloadIdentitySvc, handlerOpts)
	mux.Handle(wiPath, wiHandler)
	rolePath, roleHandler := v1connect.NewRoleServiceHandler(roleSvc, handlerOpts)
	mux.Handle(rolePath, roleHandler)
	groupPath, groupHandler := v1connect.NewGroupServiceHandler(groupSvc, handlerOpts)
	mux.Handle(groupPath, groupHandler)
	idpPath, idpHandler := v1connect.NewIdentityProviderServiceHandler(identityProviderSvc, handlerOpts)
	mux.Handle(idpPath, idpHandler)
	settingPath, settingHandler := v1connect.NewSettingServiceHandler(settingSvc, handlerOpts)
	mux.Handle(settingPath, settingHandler)
	wsPath, wsHandler := v1connect.NewWorkspaceServiceHandler(workspaceSvc, handlerOpts)
	mux.Handle(wsPath, wsHandler)
	projPath, projHandler := v1connect.NewProjectServiceHandler(projectSvc, handlerOpts)
	mux.Handle(projPath, projHandler)
	subPath, subHandler := v1connect.NewSubscriptionServiceHandler(subscriptionSvc, handlerOpts)
	mux.Handle(subPath, subHandler)
	actPath, actHandler := v1connect.NewActuatorServiceHandler(actuatorSvc, handlerOpts)
	mux.Handle(actPath, actHandler)
	auditPath, auditHandler := v1connect.NewAuditLogServiceHandler(auditLogSvc, handlerOpts)
	mux.Handle(auditPath, auditHandler)
	celPath, celHandler := v1connect.NewCelServiceHandler(celSvc, handlerOpts)
	mux.Handle(celPath, celHandler)
	aiPath, aiHandler := v1connect.NewAIServiceHandler(aiSvc, handlerOpts)
	mux.Handle(aiPath, aiHandler)

	svc := &Service{
		name:      "admin",
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
			slog.Error("Admin service server error", "err", err)
		}
	}()
	slog.Info("Admin service started")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	slog.Info("Admin service stopping")
	return s.server.Shutdown(ctx)
}

func (s *Service) Healthy(_ context.Context) error {
	return nil
}
