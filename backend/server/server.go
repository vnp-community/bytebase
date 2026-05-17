// Package server implements the API server for Bytebase.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pkg/errors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	directorysync "github.com/bytebase/bytebase/backend/api/directory-sync"
	"github.com/bytebase/bytebase/backend/api/lsp"
	"github.com/bytebase/bytebase/backend/api/mcp"
	"github.com/bytebase/bytebase/backend/api/oauth2"
	stripeapi "github.com/bytebase/bytebase/backend/api/stripe"
	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/health"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/leader"
	"github.com/bytebase/bytebase/backend/component/pghealth"
	"github.com/bytebase/bytebase/backend/component/sampleinstance"
	"github.com/bytebase/bytebase/backend/component/sheet"
	"github.com/bytebase/bytebase/backend/component/telemetry"
	"github.com/bytebase/bytebase/backend/component/webhook"
	"github.com/bytebase/bytebase/backend/demo"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/prometheus/client_golang/prometheus"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/migrator"
	"github.com/bytebase/bytebase/backend/resources/postgres"
	"github.com/bytebase/bytebase/backend/runner/approval"
	"github.com/bytebase/bytebase/backend/runner/backup"
	"github.com/bytebase/bytebase/backend/runner/cleaner"
	"github.com/bytebase/bytebase/backend/runner/heartbeat"
	runnerleader "github.com/bytebase/bytebase/backend/runner/leader"
	"github.com/bytebase/bytebase/backend/runner/leaderrunner"
	"github.com/bytebase/bytebase/backend/runner/monitor"
	"github.com/bytebase/bytebase/backend/runner/notifylistener"
	"github.com/bytebase/bytebase/backend/runner/plancheck"
	"github.com/bytebase/bytebase/backend/runner/replication"
	"github.com/bytebase/bytebase/backend/runner/selfheal"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/runner/taskrun"
	"github.com/bytebase/bytebase/backend/service"
	runnerservice "github.com/bytebase/bytebase/backend/service/runner"
	"github.com/bytebase/bytebase/backend/store"
)

const (
	// webhookAPIPrefix is the API prefix for Bytebase webhook.
	webhookAPIPrefix = "/hook"
	scimAPIPrefix    = "/scim"
	// lspAPI is the API for Bytebase Language Server Protocol.
	lspAPI                 = "/lsp"
	gracefulShutdownPeriod = 30 * time.Second
)

// Server is the Bytebase server.
type Server struct {
	// Asynchronous runners.
	taskScheduler      *taskrun.Scheduler
	planCheckScheduler *plancheck.Scheduler
	schemaSyncer       *schemasync.Syncer
	approvalRunner     *approval.Runner
	notifyListener     *notifylistener.Listener
	dataCleaner        *cleaner.DataCleaner
	heartbeatRunner    *heartbeat.Runner
	leaderRunner       *runnerleader.Runner
	selfhealRunner     *selfheal.Runner
	runnerWG           sync.WaitGroup

	webhookManager        *webhook.Manager
	iamManager            *iam.Manager
	sampleInstanceManager *sampleinstance.Manager

	licenseService *enterprise.LicenseService

	profile    *config.Profile
	echoServer *echo.Echo
	httpServer *http.Server
	lspServer  *lsp.Server
	store       *store.Store
	dbFactory   *dbfactory.DBFactory
	registry    *ComponentRegistry
	promRegistry *prometheus.Registry
	healthChecker *health.Checker
	poolManager *store.PoolManager
	startedTS   int64

	// PG server stoppers.
	stopper []func()

	// bus is the message bus for inter-component communication within the server.
	bus bus.EventBus

	// Gateway-mode fields (active when BB_USE_GATEWAY=true).
	gatewayMode    bool
	serviceRegistry *service.Registry
	runnerService   *runnerservice.Service

	// boot specifies that whether the server boot correctly
	cancel context.CancelFunc
}

// NewServer creates a server.
func NewServer(ctx context.Context, profile *config.Profile) (*Server, error) {
	s := &Server{
		profile:      profile,
		startedTS:    time.Now().Unix(),
		registry:     NewComponentRegistry(),
		promRegistry: prometheus.NewRegistry(),
	}

	// Display config
	slog.Info("-----Config BEGIN-----")
	slog.Info(fmt.Sprintf("mode=%s", profile.Mode))
	slog.Info(fmt.Sprintf("dataDir=%s", profile.DataDir))
	slog.Info(fmt.Sprintf("demo=%v", profile.Demo))
	slog.Info(fmt.Sprintf("replicaID=%s", profile.ReplicaID))
	slog.Info("-----Config END-------")

	serverStarted := false
	defer func() {
		if !serverStarted {
			_ = s.Shutdown(ctx)
		}
	}()

	if profile.HA && profile.UseEmbedDB() {
		return nil, errors.New("HA mode requires external PostgreSQL (set PG_URL environment variable)")
	}

	var pgURL string
	if profile.UseEmbedDB() {
		pgDataDir := path.Join(profile.DataDir, "pgdata")
		if profile.Demo {
			pgDataDir = path.Join(profile.DataDir, "pgdata-demo")
		}

		stopper, err := postgres.StartMetadataInstance(ctx, pgDataDir, profile.DatastorePort, profile.Mode)
		if err != nil {
			return nil, err
		}
		s.stopper = append(s.stopper, stopper)
		pgURL = fmt.Sprintf("host=%s port=%d user=bb database=bb", common.GetPostgresSocketDir(), profile.DatastorePort)
	} else {
		pgURL = profile.PgURL
	}

	// Connect to the instance that stores bytebase's own metadata.
	stores, err := store.New(ctx, pgURL, !profile.HA)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to new store")
	}

	if profile.Demo {
		if err := demo.LoadDemoData(ctx, stores.GetDB()); err != nil {
			stores.Close()
			return nil, errors.Wrapf(err, "failed to load demo data")
		}
	}
	if err := migrator.MigrateSchema(ctx, stores.GetDB()); err != nil {
		stores.Close()
		return nil, errors.Wrapf(err, "failed to migrate schema")
	}
	s.store = stores
	s.store.RegisterPoolMetrics()

	// T-012: Pre-populate database cache at startup.
	s.store.WarmDatabaseCache(ctx)

	s.healthChecker = health.NewChecker(stores.GetDB(), s.promRegistry)

	// Apply feature-flagged Store options (dual pool, cache backend).
	poolManager, err := applyStoreOptions(ctx, stores, profile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to apply store options")
	}
	s.poolManager = poolManager
	sheetManager := sheet.NewManager()

	s.licenseService, err = enterprise.NewLicenseService(profile.Mode, stores, profile.SaaS, profile.LicensePrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create license service")
	}

	logSetup := &storepb.WorkspaceProfileSetting{
		EnableAuditLogStdout:   false,
		EnableMetricCollection: true,
		EnableDebug:            profile.Debug,
	}
	if !s.profile.SaaS {
		s.sampleInstanceManager = sampleinstance.NewManager(stores, profile)

		// Load workspace-dependent settings if workspace exists.
		// On first boot (no workspace yet), these remain at defaults and get
		// initialized when the workspace is created and settings are updated via API.
		if workspaceID, _ := stores.GetWorkspaceID(ctx); workspaceID != "" {
			if err := s.sampleInstanceManager.StartIfExist(ctx, workspaceID); err != nil {
				slog.Warn("failed to start sample instances", log.BBError(err))
			}

			if workspaceProfileSetting, err := s.store.GetWorkspaceProfileSetting(ctx, workspaceID); err == nil {
				logSetup = workspaceProfileSetting
				if logSetup.GetEnableAuditLogStdout() && s.licenseService.IsFeatureEnabled(ctx, workspaceID, v1pb.PlanFeature_FEATURE_AUDIT_LOG) == nil {
					s.profile.RuntimeEnableAuditLogStdout.Store(true)
				}
			}
		}
	}

	profile.RuntimeDebug.Store(logSetup.EnableDebug)
	if logSetup.EnableDebug {
		log.LogLevel.Set(slog.LevelDebug)
	}
	telemetry.InitGlobalReporter(
		profile.Version,
		profile.GitCommit,
		logSetup.GetEnableMetricCollection(),
	)

	s.bus, err = bus.NewEventBus(profile, stores.GetDB())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create message bus")
	}

	// The auth secret is a global infrastructure value stored in server_config.
	secret, err := s.store.GetAuthSecret(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get auth secret")
	}

	s.iamManager, err = iam.NewManager(stores, s.licenseService, profile.SaaS)
	if err := s.iamManager.ReloadCache(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to reload iam cache")
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create iam manager")
	}
	s.webhookManager = webhook.NewManager(stores, profile)
	s.dbFactory = dbfactory.New(s.store, s.licenseService, s.promRegistry)

	// Configure echo server.
	s.echoServer = echo.New()

	s.schemaSyncer = schemasync.NewSyncer(stores, s.dbFactory, s.licenseService)
	s.approvalRunner = approval.NewRunner(stores, s.bus, s.webhookManager, s.licenseService)

	s.taskScheduler = taskrun.NewScheduler(stores, s.bus, s.webhookManager, s.licenseService, profile)
	s.taskScheduler.Register(storepb.Task_DATABASE_CREATE, taskrun.NewDatabaseCreateExecutor(stores, s.dbFactory, s.schemaSyncer))
	s.taskScheduler.Register(storepb.Task_DATABASE_MIGRATE, taskrun.NewDatabaseMigrateExecutor(stores, s.dbFactory, s.bus, s.schemaSyncer, profile))
	s.taskScheduler.Register(storepb.Task_DATABASE_EXPORT, taskrun.NewDataExportExecutor(stores, s.dbFactory, s.licenseService))

	combinedExecutor := plancheck.NewCombinedExecutor(stores, sheetManager, s.dbFactory)
	s.planCheckScheduler = plancheck.NewScheduler(stores, s.bus, combinedExecutor, s.licenseService)
	s.notifyListener = notifylistener.NewListener(stores.GetDB(), s.bus)

	// Data cleaner
	s.dataCleaner = cleaner.NewDataCleaner(stores, s.licenseService)

	// Heartbeat runner
	s.heartbeatRunner = heartbeat.NewRunner(stores, profile)
	// Leader runner
	s.leaderRunner = runnerleader.NewRunner(stores, profile)
	// Selfheal runner
	s.selfhealRunner = selfheal.NewRunner(stores, profile, s.healthChecker)

	// LSP server.
	s.lspServer = lsp.NewServer(s.store, profile, secret, s.bus, s.iamManager, s.licenseService)

	directorySyncServer := directorysync.NewService(s.store, s.licenseService, s.iamManager, profile)
	oauth2Service := oauth2.NewService(stores, profile, secret)
	mcpServer, err := mcp.NewServer(stores, profile, secret)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MCP server")
	}

	stripeWebhookHandler := stripeapi.NewWebhookHandler(s.store, s.licenseService, profile.StripeWebhookSecret)

	// Choose routing mode: gateway (new) vs legacy (existing).
	if os.Getenv("BB_USE_GATEWAY") == "true" {
		slog.Info("Gateway mode enabled — initializing domain services")
		s.gatewayMode = true

		gwResult := initGatewayServices(&GatewayDeps{
			Store:                 s.store,
			SheetManager:          sheetManager,
			DBFactory:             s.dbFactory,
			LicenseService:        s.licenseService,
			Profile:               s.profile,
			Bus:                   s.bus,
			SchemaSyncer:          s.schemaSyncer,
			WebhookManager:        s.webhookManager,
			IAMManager:            s.iamManager,
			Secret:                secret,
			SampleInstanceManager: s.sampleInstanceManager,
			HealthChecker:         s.healthChecker,
		})
		s.serviceRegistry = gwResult.Registry
		s.runnerService = gwResult.RunnerService

		// Start domain services (bufconn HTTP servers).
		if err := s.serviceRegistry.StartAll(ctx); err != nil {
			return nil, errors.Wrapf(err, "failed to start domain services")
		}

		// Configure gateway routing.
		if err := configureGatewayRouters(ctx, s.echoServer, s.serviceRegistry, &GatewayDeps{
			Store:          s.store,
			Secret:         secret,
			LicenseService: s.licenseService,
			Bus:            s.bus,
			Profile:        s.profile,
			IAMManager:     s.iamManager,
		}); err != nil {
			return nil, errors.Wrapf(err, "failed to configure gateway routers")
		}
	} else {
		// Legacy mode — existing monolithic router.
		if err := configureGrpcRouters(ctx, s.echoServer, s.store, sheetManager, s.dbFactory, s.licenseService, s.profile, s.bus, s.schemaSyncer, s.webhookManager, s.iamManager, secret, s.sampleInstanceManager); err != nil {
			return nil, errors.Wrapf(err, "failed to configure gRPC routers")
		}
	}
	configureEchoRouters(s.echoServer, s, s.lspServer, directorySyncServer, oauth2Service, mcpServer, stripeWebhookHandler, profile)

	serverStarted = true
	return s, nil
}

// Run will run the server.
func (s *Server) Run(ctx context.Context, port int) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	if s.gatewayMode {
		// Gateway mode — delegate to RunnerService.
		slog.Info("Gateway mode: starting runners via RunnerService")
		s.runnerService.Run(ctx)
	} else if s.profile.HA {
		// HA mode: exclusive runners use leader election, shared runners run on all replicas.
		slog.Info("HA mode enabled — starting runners with leader election")

		// Exclusive runners — only leader executes
		s.startLeaderRunner(ctx, s.taskScheduler, leader.LockIDTaskScheduler, "TaskScheduler")
		s.startLeaderRunner(ctx, s.schemaSyncer, leader.LockIDSchemaSync, "SchemaSync")
		s.startLeaderRunner(ctx, s.approvalRunner, leader.LockIDApproval, "Approval")
		s.startLeaderRunner(ctx, s.planCheckScheduler, leader.LockIDPlanCheck, "PlanCheck")
		s.startLeaderRunner(ctx, s.dataCleaner, leader.LockIDDataCleaner, "DataCleaner")

		// Shared runners — all replicas
		s.runnerWG.Add(1)
		go s.leaderRunner.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.selfhealRunner.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.heartbeatRunner.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.notifyListener.Run(ctx, &s.runnerWG)

		// Start cache invalidator for cross-replica cache coherence
		cacheInvalidator := store.NewCacheInvalidator(s.store, s.store.GetDB())
		s.runnerWG.Add(1)
		go cacheInvalidator.Run(ctx, &s.runnerWG)

		// Start PGBus consumers for durable message processing
		if pgBus, ok := s.bus.(*bus.PGBus); ok {
			pgBus.StartConsumers(ctx, &s.runnerWG)
		}
	} else {
		// Single-node: existing behavior (unchanged)
		s.runnerWG.Add(1)
		go s.taskScheduler.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.schemaSyncer.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.approvalRunner.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.planCheckScheduler.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.dataCleaner.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.heartbeatRunner.Run(ctx, &s.runnerWG)
		s.runnerWG.Add(1)
		go s.notifyListener.Run(ctx, &s.runnerWG)
	}

	// Additional monitors — only in legacy/HA mode (gateway RunnerService handles these).
	if !s.gatewayMode {
		s.runnerWG.Add(1)
		mmm := monitor.NewMemoryMonitor(s.profile)
		go mmm.Run(ctx, &s.runnerWG)

		s.runnerWG.Add(1)
		poolMon := monitor.NewPoolMonitor(s.store.GetDB())
		go poolMon.Run(ctx, &s.runnerWG)

		// PG health monitor — embedded PG only (no-op for external PG).
		pgMon := pghealth.NewMonitor(s.store.GetDB(), s.profile, nil)
		s.runnerWG.Add(1)
		go pgMon.Run(ctx, &s.runnerWG)

		// PG backup scheduler
		if s.profile.BackupEnabled {
			backupExecutor := backup.NewExecutor(s.store, s.profile)
			backupSched := backup.NewScheduler(s.store, s.profile, backupExecutor, s.leaderRunner.IsLeader)
			s.runnerWG.Add(1)
			go backupSched.Run(ctx, &s.runnerWG)
		}

		// Multi-Region Replication Monitor
		if s.profile.RegionRole != "" {
			repMon := replication.NewMonitor(s.store.GetDB(), s.profile)
			s.runnerWG.Add(1)
			go repMon.Run(ctx, &s.runnerWG)
		}
	}

	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	// Create HTTP server with H2C support (Echo v5)
	s.httpServer = &http.Server{
		Addr:    address,
		Handler: h2c.NewHandler(s.echoServer, &http2.Server{}),
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("http server listen error", log.BBError(err))
			}
		}
	}()

	return nil
}

// Shutdown will shut down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Stopping Bytebase...")
	slog.Info("Stopping web server...")

	ctx, cancel := context.WithTimeout(ctx, gracefulShutdownPeriod)
	defer cancel()

	// Gateway mode: signal heartbeat via RunnerService.
	if s.gatewayMode {
		if hr := s.runnerService.HeartbeatRunner(); hr != nil {
			hr.SetStatus("DRAINING")
			hr.SendHeartbeat(context.Background())
		}
	} else if s.heartbeatRunner != nil {
		s.heartbeatRunner.SetStatus("DRAINING")
		s.heartbeatRunner.SendHeartbeat(context.Background())
	}

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown HTTP server", log.BBError(err))
		}
	}

	// Cancel the worker
	if s.cancel != nil {
		s.cancel()
	}

	// Wait for all runners to exit.
	if s.gatewayMode {
		s.runnerService.Wait()
		// Stop domain services.
		if err := s.serviceRegistry.StopAll(ctx); err != nil {
			slog.Error("Failed to stop domain services", log.BBError(err))
		}
	} else {
		s.runnerWG.Wait()
	}

	if s.gatewayMode {
		if hr := s.runnerService.HeartbeatRunner(); hr != nil {
			hr.SetStatus("STOPPED")
			hr.SendHeartbeat(context.Background())
		}
	} else if s.heartbeatRunner != nil {
		s.heartbeatRunner.SetStatus("STOPPED")
		s.heartbeatRunner.SendHeartbeat(context.Background())
	}

	// Close pool manager (dual pool)
	if s.poolManager != nil {
		if err := s.poolManager.Close(); err != nil {
			slog.Error("Failed to close pool manager", log.BBError(err))
		}
	}

	// Close db connection
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return err
		}
	}

	// Shutdown sample instances
	if s.sampleInstanceManager != nil {
		s.sampleInstanceManager.Stop()
	}

	// Shutdown postgres instances.
	for _, stopper := range s.stopper {
		stopper()
	}

	return nil
}

// startLeaderRunner wraps a runner with leader election and starts it in a goroutine.
// The runner will only execute on the replica that holds the advisory lock for lockID.
func (s *Server) startLeaderRunner(ctx context.Context, r leaderrunner.Runner, lockID int64, name string) {
	elector := leader.NewLeaderElector(s.store.GetDB(), lockID, 10*time.Second, name)
	wrapped := leaderrunner.NewLeaderRunner(r, elector, name)
	s.runnerWG.Add(1)
	go wrapped.Run(ctx, &s.runnerWG)
}

