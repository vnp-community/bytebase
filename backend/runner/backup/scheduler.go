package backup

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/store"
)

type Scheduler struct {
	store      *store.Store
	profile    *config.Profile
	executor   *Executor
	cronEngine *cron.Cron
	isLeader   func() bool
}

func NewScheduler(st *store.Store, prof *config.Profile, executor *Executor, isLeader func() bool) *Scheduler {
	return &Scheduler{
		store:      st,
		profile:    prof,
		executor:   executor,
		cronEngine: cron.New(),
		isLeader:   isLeader,
	}
}

func (s *Scheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	if !s.profile.BackupEnabled {
		return
	}

	_, err := s.cronEngine.AddFunc(s.profile.BackupSchedule, func() {
		if !s.isLeader() {
			return
		}
		
		// Attempt to acquire advisory lock for backup
		lock, locked, err := store.TryAdvisoryLock(ctx, s.store.GetDB(), store.AdvisoryLockKeyBackup)
		if err != nil || !locked {
			return
		}
		defer lock.Release()

		slog.Info("Starting scheduled full backup")
		result, err := s.executor.ExecuteFullBackup(ctx)
		if err != nil {
			slog.Error("Scheduled backup failed", log.BBError(err))
			return
		}
		slog.Info("Scheduled backup completed successfully", slog.String("backupID", result.BackupID))
	})

	if err != nil {
		slog.Error("Failed to schedule backup cron", log.BBError(err))
		return
	}

	s.cronEngine.Start()
	defer s.cronEngine.Stop()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkRPO(ctx)
		}
	}
}

func (s *Scheduler) checkRPO(ctx context.Context) {
	if !s.isLeader() {
		return
	}
	latest, err := s.store.GetLatestSuccessfulBackup(ctx)
	if err != nil {
		slog.Error("Failed to check latest backup for RPO", log.BBError(err))
		return
	}
	if latest == nil {
		slog.Warn("No successful backups found, RPO violated")
		return
	}

	durationSinceBackup := time.Since(latest.DataTimestamp)
	targetRPO := time.Duration(s.profile.TargetRPOMinutes) * time.Minute

	if durationSinceBackup > targetRPO {
		slog.Warn("RPO violated", 
			slog.Duration("since_last_backup", durationSinceBackup),
			slog.Duration("target_rpo", targetRPO))
	} else {
		slog.Debug("RPO compliance verified", slog.Duration("since_last_backup", durationSinceBackup))
	}
}
