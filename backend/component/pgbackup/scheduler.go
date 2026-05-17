// Package pgbackup provides a scheduled backup runner for embedded PostgreSQL.
// It creates periodic pg_dump backups with compression and automatic rotation
// of old backup files. The scheduler is a no-op when using external PostgreSQL.
package pgbackup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
)

const (
	// defaultSchedule is the default backup schedule: daily at 2 AM.
	defaultScheduleHour = 2
	// defaultRetention is the number of backup files to keep.
	defaultRetention = 7
	// backupFilePrefix is the prefix for backup files.
	backupFilePrefix = "bytebase_auto_backup_"
	// backupFileSuffix is the suffix for backup files.
	backupFileSuffix = ".dump"
	// checkInterval is how often we check if it's time for a backup.
	checkInterval = 15 * time.Minute
)

// Config holds backup scheduler configuration.
type Config struct {
	// PgURL is the connection string for the embedded PostgreSQL.
	PgURL string
	// BackupDir is the directory where backups are stored.
	BackupDir string
	// Retention is the number of most recent backups to keep. Older files are deleted.
	Retention int
	// ScheduleHour is the hour of day (0-23) when the backup runs. Default: 2.
	ScheduleHour int
}

// Scheduler runs periodic pg_dump backups for embedded PostgreSQL.
type Scheduler struct {
	cfg     Config
	profile *config.Profile

	lastBackupDate string // "2006-01-02" format to prevent multiple runs per day
}

// NewScheduler creates a new backup scheduler.
// Environment variable overrides:
//   - PG_BACKUP_DIR: override BackupDir
//   - PG_BACKUP_RETENTION: override Retention
//   - PG_BACKUP_SCHEDULE_HOUR: override ScheduleHour (0-23)
func NewScheduler(pgURL string, profile *config.Profile) *Scheduler {
	backupDir := filepath.Join(profile.DataDir, "backups")
	if dir := os.Getenv("PG_BACKUP_DIR"); dir != "" {
		backupDir = dir
	}

	retention := defaultRetention
	if v := os.Getenv("PG_BACKUP_RETENTION"); v != "" {
		if n := parseInt(v); n > 0 {
			retention = n
		}
	}

	scheduleHour := defaultScheduleHour
	if v := os.Getenv("PG_BACKUP_SCHEDULE_HOUR"); v != "" {
		if n := parseInt(v); n >= 0 && n < 24 {
			scheduleHour = n
		}
	}

	return &Scheduler{
		cfg: Config{
			PgURL:        pgURL,
			BackupDir:    backupDir,
			Retention:    retention,
			ScheduleHour: scheduleHour,
		},
		profile: profile,
	}
}

// Run starts the backup scheduler. It checks every 15 minutes if a backup is needed.
// Only active for embedded PG mode (profile.UseEmbedDB() == true).
func (s *Scheduler) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	if !s.profile.UseEmbedDB() {
		slog.Info("PG backup scheduler disabled — using external PostgreSQL")
		return
	}

	slog.Info("PG backup scheduler started",
		"backup_dir", s.cfg.BackupDir,
		"retention", s.cfg.Retention,
		"schedule_hour", s.cfg.ScheduleHour,
	)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Check immediately on start.
	s.maybeBackup(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("PG backup scheduler stopped")
			return
		case <-ticker.C:
			s.maybeBackup(ctx)
		}
	}
}

// maybeBackup runs a backup if the current time matches the schedule and we haven't
// already run one today.
func (s *Scheduler) maybeBackup(ctx context.Context) {
	now := time.Now()
	today := now.Format("2006-01-02")

	// Already backed up today.
	if s.lastBackupDate == today {
		return
	}

	// Not the right hour yet.
	if now.Hour() != s.cfg.ScheduleHour {
		return
	}

	slog.Info("Starting scheduled PG backup...")

	if err := s.runBackup(ctx); err != nil {
		slog.Error("Scheduled PG backup failed", log.BBError(err))
		return
	}

	s.lastBackupDate = today
	slog.Info("Scheduled PG backup completed")

	// Rotate old backups.
	s.rotateBackups()
}

// runBackup creates a compressed pg_dump backup.
func (s *Scheduler) runBackup(ctx context.Context) error {
	if err := os.MkdirAll(s.cfg.BackupDir, 0o755); err != nil {
		return fmt.Errorf("failed to create backup directory %s: %w", s.cfg.BackupDir, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(s.cfg.BackupDir, fmt.Sprintf("%s%s%s", backupFilePrefix, timestamp, backupFileSuffix))

	cmd := exec.CommandContext(ctx,
		"pg_dump",
		"--format", "custom",
		"--compress", "6",
		"--no-owner",
		"--file", backupFile,
		s.cfg.PgURL,
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Clean up partial file.
		_ = os.Remove(backupFile)
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	info, err := os.Stat(backupFile)
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %w", err)
	}
	slog.Info("PG backup created",
		"path", backupFile,
		"size_mb", info.Size()/(1024*1024),
	)

	return nil
}

// rotateBackups removes old backup files, keeping only the most recent N (retention count).
func (s *Scheduler) rotateBackups() {
	entries, err := os.ReadDir(s.cfg.BackupDir)
	if err != nil {
		slog.Warn("Failed to list backup directory for rotation", log.BBError(err))
		return
	}

	// Filter to only our backup files.
	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, backupFilePrefix) && strings.HasSuffix(name, backupFileSuffix) {
			backups = append(backups, name)
		}
	}

	// Sort ascending (oldest first) — timestamp in filename guarantees correct order.
	sort.Strings(backups)

	// Remove oldest files exceeding retention.
	if len(backups) <= s.cfg.Retention {
		return
	}

	toDelete := backups[:len(backups)-s.cfg.Retention]
	for _, name := range toDelete {
		path := filepath.Join(s.cfg.BackupDir, name)
		if err := os.Remove(path); err != nil {
			slog.Warn("Failed to remove old backup", "path", path, log.BBError(err))
		} else {
			slog.Info("Rotated old backup", "path", path)
		}
	}
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
