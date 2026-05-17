// Package dcm implements the Database Change Management domain service.
// It wraps 8 api/v1 handlers related to plan, issue, rollout, release,
// revision, review config, access grant, and org policy management.
package dcm

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
	"github.com/bytebase/bytebase/backend/component/sheet"
	"github.com/bytebase/bytebase/backend/component/webhook"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/transport"

	bblog "github.com/bytebase/bytebase/backend/common/log"
	"github.com/pkg/errors"
)

// Deps holds all dependencies needed by the DCM service.
type Deps struct {
	Store          *store.Store
	SheetManager   *sheet.Manager
	DBFactory      *dbfactory.DBFactory
	LicenseService *enterprise.LicenseService
	Profile        *config.Profile
	Bus            bus.EventBus
	WebhookManager *webhook.Manager
	IAMManager     *iam.Manager
	Secret         string
}

// Service is the DCM domain service.
type Service struct {
	name      string
	transport *transport.BufconnTransport
	server    *http.Server
}

// NewService creates the DCM domain service with all handlers registered.
func NewService(deps *Deps) *Service {
	tr := transport.NewBufconnTransport()
	mux := http.NewServeMux()

	// Build interceptor chain (same order as grpc_routes.go).
	onPanic := func(_ context.Context, s connect.Spec, _ http.Header, p any) error {
		stack := stacktrace.TakeStacktrace(20, 5)
		slog.Error("dcm service panic", "method", s.Procedure, bblog.BBError(errors.Errorf("error: %v\n%s", p, stack)))
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

	// 8 DCM sub-services — exact same constructors as grpc_routes.go.
	planSvc := apiv1.NewPlanService(deps.Store, deps.Bus, deps.IAMManager, deps.WebhookManager, deps.LicenseService)
	issueSvc := apiv1.NewIssueService(deps.Store, deps.WebhookManager, deps.Bus, deps.LicenseService, deps.IAMManager)
	rolloutSvc := apiv1.NewRolloutService(deps.Store, deps.DBFactory, deps.Bus, deps.WebhookManager, deps.IAMManager)
	releaseSvc := apiv1.NewReleaseService(deps.Store, deps.SheetManager, deps.DBFactory)
	revisionSvc := apiv1.NewRevisionService(deps.Store)
	reviewConfigSvc := apiv1.NewReviewConfigService(deps.Store)
	accessGrantSvc := apiv1.NewAccessGrantService(deps.Store, deps.LicenseService, deps.WebhookManager, deps.Bus)
	orgPolicySvc := apiv1.NewOrgPolicyService(deps.Store, deps.LicenseService, deps.IAMManager)

	// Register ConnectRPC handlers.
	planPath, planHandler := v1connect.NewPlanServiceHandler(planSvc, handlerOpts)
	mux.Handle(planPath, planHandler)
	issuePath, issueHandler := v1connect.NewIssueServiceHandler(issueSvc, handlerOpts)
	mux.Handle(issuePath, issueHandler)
	rolloutPath, rolloutHandler := v1connect.NewRolloutServiceHandler(rolloutSvc, handlerOpts)
	mux.Handle(rolloutPath, rolloutHandler)
	releasePath, releaseHandler := v1connect.NewReleaseServiceHandler(releaseSvc, handlerOpts)
	mux.Handle(releasePath, releaseHandler)
	revisionPath, revisionHandler := v1connect.NewRevisionServiceHandler(revisionSvc, handlerOpts)
	mux.Handle(revisionPath, revisionHandler)
	reviewConfigPath, reviewConfigHandler := v1connect.NewReviewConfigServiceHandler(reviewConfigSvc, handlerOpts)
	mux.Handle(reviewConfigPath, reviewConfigHandler)
	accessGrantPath, accessGrantHandler := v1connect.NewAccessGrantServiceHandler(accessGrantSvc, handlerOpts)
	mux.Handle(accessGrantPath, accessGrantHandler)
	orgPolicyPath, orgPolicyHandler := v1connect.NewOrgPolicyServiceHandler(orgPolicySvc, handlerOpts)
	mux.Handle(orgPolicyPath, orgPolicyHandler)

	svc := &Service{
		name:      "dcm",
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
			slog.Error("DCM service server error", "err", err)
		}
	}()
	slog.Info("DCM service started")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	slog.Info("DCM service stopping")
	return s.server.Shutdown(ctx)
}

func (s *Service) Healthy(_ context.Context) error {
	return nil
}
