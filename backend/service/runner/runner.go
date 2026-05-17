// Package runner implements the Runner domain service.
// It wraps all background runner goroutines (task scheduler, plan check,
// schema sync, approval, etc.) into a single managed service.
package runner

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/health"
	"github.com/bytebase/bytebase/backend/component/leader"
	"github.com/bytebase/bytebase/backend/component/pghealth"
	"github.com/bytebase/bytebase/backend/component/sheet"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/runner/approval"
	"github.com/bytebase/bytebase/backend/runner/backup"
	"github.com/bytebase/bytebase/backend/runner/cleaner"
	"github.com/bytebase/bytebase/backend/runner/heartbeat"
	runnerleader "github.com/bytebase/bytebase/backend/runner/leader"
	"github.com/bytebase/bytebase/backend/runner/monitor"
	"github.com/bytebase/bytebase/backend/runner/notifylistener"
	"github.com/bytebase/bytebase/backend/runner/plancheck"
	"github.com/bytebase/bytebase/backend/runner/replication"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/runner/selfheal"
	"github.com/bytebase/bytebase/backend/runner/taskrun"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/component/webhook"
)

// Deps holds all dependencies needed to construct runners.
type Deps struct {
	Store          *store.Store
	Bus            bus.EventBus
	DBFactory      *dbfactory.DBFactory
	LicenseService *enterprise.LicenseService
	Profile        *config.Profile
	SheetManager   *sheet.Manager
	WebhookManager *webhook.Manager
	HealthChecker  *health.Checker
}

// Service manages all background runners.
type Service struct {
	taskScheduler      *taskrun.Scheduler
	planCheckScheduler *plancheck.Scheduler
	schemaSyncer       *schemasync.Syncer
	approvalRunner     *approval.Runner
	notifyListener     *notifylistener.Listener
	dataCleaner        *cleaner.DataCleaner
	heartbeatRunner    *heartbeat.Runner
	leaderRunner       *runnerleader.Runner
	selfhealRunner     *selfheal.Runner

	deps *Deps
	wg   sync.WaitGroup
}

// NewService creates the Runner service with all background runners constructed.
// Constructors are the EXACT SAME as server.go lines 252-272.
func NewService(deps *Deps) *Service {
	schemaSyncer := schemasync.NewSyncer(deps.Store, deps.DBFactory, deps.LicenseService)
	approvalRunner := approval.NewRunner(deps.Store, deps.Bus, deps.WebhookManager, deps.LicenseService)

	taskScheduler := taskrun.NewScheduler(deps.Store, deps.Bus, deps.WebhookManager, deps.LicenseService, deps.Profile)
	taskScheduler.Register(storepb.Task_DATABASE_CREATE, taskrun.NewDatabaseCreateExecutor(deps.Store, deps.DBFactory, schemaSyncer))
	taskScheduler.Register(storepb.Task_DATABASE_MIGRATE, taskrun.NewDatabaseMigrateExecutor(deps.Store, deps.DBFactory, deps.Bus, schemaSyncer, deps.Profile))
	taskScheduler.Register(storepb.Task_DATABASE_EXPORT, taskrun.NewDataExportExecutor(deps.Store, deps.DBFactory, deps.LicenseService))

	combinedExecutor := plancheck.NewCombinedExecutor(deps.Store, deps.SheetManager, deps.DBFactory)
	planCheckScheduler := plancheck.NewScheduler(deps.Store, deps.Bus, combinedExecutor, deps.LicenseService)
	notifyListener := notifylistener.NewListener(deps.Store.GetDB(), deps.Bus)

	dataCleaner := cleaner.NewDataCleaner(deps.Store, deps.LicenseService)
	heartbeatRunner := heartbeat.NewRunner(deps.Store, deps.Profile)
	leaderRunner := runnerleader.NewRunner(deps.Store, deps.Profile)
	selfhealRunner := selfheal.NewRunner(deps.Store, deps.Profile, deps.HealthChecker)

	return &Service{
		taskScheduler:      taskScheduler,
		planCheckScheduler: planCheckScheduler,
		schemaSyncer:       schemaSyncer,
		approvalRunner:     approvalRunner,
		notifyListener:     notifyListener,
		dataCleaner:        dataCleaner,
		heartbeatRunner:    heartbeatRunner,
		leaderRunner:       leaderRunner,
		selfhealRunner:     selfhealRunner,
		deps:               deps,
	}
}

// Run starts all background runners (same logic as server.go Run()).
func (s *Service) Run(ctx context.Context) {
	profile := s.deps.Profile

	if profile.HA {
		slog.Info("Runner service: HA mode — starting with leader election")
		s.startLeaderRunner(ctx, s.taskScheduler, leader.LockIDTaskScheduler, "TaskScheduler")
		s.startLeaderRunner(ctx, s.schemaSyncer, leader.LockIDSchemaSync, "SchemaSync")
		s.startLeaderRunner(ctx, s.approvalRunner, leader.LockIDApproval, "Approval")
		s.startLeaderRunner(ctx, s.planCheckScheduler, leader.LockIDPlanCheck, "PlanCheck")
		s.startLeaderRunner(ctx, s.dataCleaner, leader.LockIDDataCleaner, "DataCleaner")

		// Shared runners — all replicas.
		s.wg.Add(1)
		go s.leaderRunner.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.selfhealRunner.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.heartbeatRunner.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.notifyListener.Run(ctx, &s.wg)

		// Cache invalidator.
		cacheInvalidator := store.NewCacheInvalidator(s.deps.Store, s.deps.Store.GetDB())
		s.wg.Add(1)
		go cacheInvalidator.Run(ctx, &s.wg)

		// PGBus consumers.
		if pgBus, ok := s.deps.Bus.(*bus.PGBus); ok {
			pgBus.StartConsumers(ctx, &s.wg)
		}
	} else {
		// Single-node: existing behavior (unchanged).
		s.wg.Add(1)
		go s.taskScheduler.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.schemaSyncer.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.approvalRunner.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.planCheckScheduler.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.dataCleaner.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.heartbeatRunner.Run(ctx, &s.wg)
		s.wg.Add(1)
		go s.notifyListener.Run(ctx, &s.wg)
	}

	// Monitors — always run.
	s.wg.Add(1)
	mmm := monitor.NewMemoryMonitor(profile)
	go mmm.Run(ctx, &s.wg)

	s.wg.Add(1)
	poolMon := monitor.NewPoolMonitor(s.deps.Store.GetDB())
	go poolMon.Run(ctx, &s.wg)

	pgMon := pghealth.NewMonitor(s.deps.Store.GetDB(), profile, nil)
	s.wg.Add(1)
	go pgMon.Run(ctx, &s.wg)

	// PG backup scheduler.
	if profile.BackupEnabled {
		backupExecutor := backup.NewExecutor(s.deps.Store, profile)
		backupSched := backup.NewScheduler(s.deps.Store, profile, backupExecutor, s.leaderRunner.IsLeader)
		s.wg.Add(1)
		go backupSched.Run(ctx, &s.wg)
	}

	// Multi-Region Replication Monitor.
	if profile.RegionRole != "" {
		repMon := replication.NewMonitor(s.deps.Store.GetDB(), profile)
		s.wg.Add(1)
		go repMon.Run(ctx, &s.wg)
	}

	slog.Info("Runner service started all background runners")
}

// Wait blocks until all runners have finished.
func (s *Service) Wait() {
	s.wg.Wait()
}

// HeartbeatRunner returns the heartbeat runner for graceful shutdown signaling.
func (s *Service) HeartbeatRunner() *heartbeat.Runner {
	return s.heartbeatRunner
}

// SchemaSyncer returns the schema syncer for use by other services.
func (s *Service) SchemaSyncer() *schemasync.Syncer {
	return s.schemaSyncer
}

// startLeaderRunner wraps a runner with leader election.
func (s *Service) startLeaderRunner(ctx context.Context, runner leaderrunner, lockID int64, name string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			if s.leaderRunner.IsLeader() {
				slog.Info("Leader elected, starting runner", "runner", name)
				var innerWg sync.WaitGroup
				innerWg.Add(1)
				go runner.Run(ctx, &innerWg)
				innerWg.Wait()
			} else {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()
	_ = lockID // Used by the leader election framework externally.
}

// leaderrunner is the interface for runners that can be wrapped in leader election.
type leaderrunner interface {
	Run(ctx context.Context, wg *sync.WaitGroup)
}
