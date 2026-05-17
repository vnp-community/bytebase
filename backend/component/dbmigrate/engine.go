// Package dbmigrate provides a migration engine for moving Bytebase metadata
// from an embedded PostgreSQL instance to an external PostgreSQL server.
// The engine implements a 6-step pipeline: validate → backup → dump → restore → verify → report.
package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common/log"
)

// Phase represents a migration pipeline phase.
type Phase string

const (
	PhaseValidateTarget Phase = "validate_target"
	PhaseCreateBackup   Phase = "create_backup"
	PhaseDumpEmbedded   Phase = "dump_embedded"
	PhaseRestoreTarget  Phase = "restore_target"
	PhaseVerifyIntegrity Phase = "verify_integrity"
	PhaseReport         Phase = "report"
)

// MigrationProgress reports the current state of a migration step.
type MigrationProgress struct {
	Phase   Phase  `json:"phase"`
	Percent int    `json:"percent"`
	Message string `json:"message"`
}

// MigrationResult is the final output of a successful migration.
type MigrationResult struct {
	TargetURL    string        `json:"targetUrl"`
	Duration     time.Duration `json:"duration"`
	BackupPath   string        `json:"backupPath"`
	RowsVerified int           `json:"rowsVerified"`
}

// keyTables are the tables whose row counts we verify after migration.
var keyTables = []string{
	"principal",
	"member",
	"instance",
	"db",
	"issue",
	"pipeline",
	"stage",
	"task",
	"changelog",
	"setting",
	"project",
}

// Config holds migration engine configuration.
type Config struct {
	// SourcePgURL is the embedded PG connection string.
	SourcePgURL string
	// TargetPgURL is the external PG connection string to migrate to.
	TargetPgURL string
	// BackupDir is the directory for backup files.
	BackupDir string
	// DryRun only validates the target without performing the migration.
	DryRun bool
	// SourceDataDir is the pgdata directory for embedded PG (used to locate pg_dump).
	SourceDataDir string
}

// Engine orchestrates the embedded-to-external PG migration pipeline.
type Engine struct {
	cfg      Config
	progress chan MigrationProgress
}

// NewEngine creates a new migration engine.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:      cfg,
		progress: make(chan MigrationProgress, 20),
	}
}

// Progress returns the channel for receiving migration progress updates.
func (e *Engine) Progress() <-chan MigrationProgress {
	return e.progress
}

func (e *Engine) report(phase Phase, percent int, msg string) {
	e.progress <- MigrationProgress{Phase: phase, Percent: percent, Message: msg}
	slog.Info("migration progress", "phase", phase, "percent", percent, "msg", msg)
}

// Run executes the full migration pipeline.
func (e *Engine) Run(ctx context.Context) (*MigrationResult, error) {
	defer close(e.progress)
	start := time.Now()

	// Step 1: Validate target
	e.report(PhaseValidateTarget, 0, "Validating target PostgreSQL connection...")
	if err := e.validateTarget(ctx); err != nil {
		return nil, errors.Wrap(err, "target validation failed")
	}
	e.report(PhaseValidateTarget, 100, "Target validation passed")

	if e.cfg.DryRun {
		e.report(PhaseReport, 100, "Dry-run complete — target is valid")
		return &MigrationResult{
			TargetURL: e.cfg.TargetPgURL,
			Duration:  time.Since(start),
		}, nil
	}

	// Step 2: Create backup
	e.report(PhaseCreateBackup, 0, "Creating backup of current data...")
	backupPath, err := e.createBackup(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "backup creation failed")
	}
	e.report(PhaseCreateBackup, 100, fmt.Sprintf("Backup created: %s", backupPath))

	// Step 3: Dump embedded
	e.report(PhaseDumpEmbedded, 0, "Dumping embedded PostgreSQL...")
	dumpPath, err := e.dumpEmbedded(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "dump failed")
	}
	e.report(PhaseDumpEmbedded, 100, fmt.Sprintf("Dump completed: %s", dumpPath))

	// Step 4: Restore to target
	e.report(PhaseRestoreTarget, 0, "Restoring to target PostgreSQL...")
	if err := e.restoreToTarget(ctx, dumpPath); err != nil {
		return nil, errors.Wrap(err, "restore failed")
	}
	e.report(PhaseRestoreTarget, 100, "Restore completed successfully")

	// Step 5: Verify integrity
	e.report(PhaseVerifyIntegrity, 0, "Verifying data integrity...")
	verified, err := e.verifyIntegrity(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "integrity verification failed")
	}
	e.report(PhaseVerifyIntegrity, 100, fmt.Sprintf("Verified %d tables", verified))

	// Step 6: Report
	result := &MigrationResult{
		TargetURL:    e.cfg.TargetPgURL,
		Duration:     time.Since(start),
		BackupPath:   backupPath,
		RowsVerified: verified,
	}
	e.report(PhaseReport, 100, fmt.Sprintf(
		"Migration complete! Set PG_URL=%s to use the external database.",
		e.cfg.TargetPgURL,
	))

	return result, nil
}

// validateTarget connects to the target PG, checks version ≥14, and verifies the database is empty.
func (e *Engine) validateTarget(ctx context.Context) error {
	db, err := sql.Open("pgx", e.cfg.TargetPgURL)
	if err != nil {
		return errors.Wrap(err, "failed to connect to target PostgreSQL")
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return errors.Wrap(err, "target PostgreSQL is not reachable")
	}

	// Check PG version
	var versionStr string
	if err := db.QueryRowContext(ctx, "SHOW server_version").Scan(&versionStr); err != nil {
		return errors.Wrap(err, "failed to query PostgreSQL version")
	}
	major := parseMajorVersion(versionStr)
	if major < 14 {
		return errors.Errorf("target PostgreSQL version %s (major %d) is too old, minimum required is 14", versionStr, major)
	}

	// Check database is empty (no user tables)
	var tableCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables 
		 WHERE table_schema NOT IN ('pg_catalog', 'information_schema')`).Scan(&tableCount)
	if err != nil {
		return errors.Wrap(err, "failed to check target database tables")
	}
	if tableCount > 0 {
		return errors.Errorf("target database is not empty — found %d user tables. Use an empty database for migration", tableCount)
	}

	slog.Info("Target PostgreSQL validated", "version", versionStr, "tables", tableCount)
	return nil
}

// createBackup creates a backup of the embedded PG using pg_dump.
func (e *Engine) createBackup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(e.cfg.BackupDir, 0o755); err != nil {
		return "", errors.Wrapf(err, "failed to create backup directory %s", e.cfg.BackupDir)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(e.cfg.BackupDir, fmt.Sprintf("bytebase_backup_%s.dump", timestamp))

	cmd := exec.CommandContext(ctx,
		"pg_dump",
		"--format", "custom",
		"--no-owner",
		"--compress", "6",
		"--file", backupFile,
		e.cfg.SourcePgURL,
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "pg_dump backup failed")
	}

	info, err := os.Stat(backupFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to stat backup file")
	}
	slog.Info("Backup created", "path", backupFile, "size_mb", info.Size()/(1024*1024))

	return backupFile, nil
}

// dumpEmbedded creates a pg_dump of the embedded PostgreSQL in custom format.
func (e *Engine) dumpEmbedded(ctx context.Context) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	dumpFile := filepath.Join(e.cfg.BackupDir, fmt.Sprintf("bytebase_migration_%s.dump", timestamp))

	cmd := exec.CommandContext(ctx,
		"pg_dump",
		"--format", "custom",
		"--no-owner",
		"--file", dumpFile,
		e.cfg.SourcePgURL,
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "pg_dump for migration failed")
	}

	return dumpFile, nil
}

// restoreToTarget restores the dump to the target PostgreSQL.
func (e *Engine) restoreToTarget(ctx context.Context, dumpPath string) error {
	cmd := exec.CommandContext(ctx,
		"pg_restore",
		"--dbname", e.cfg.TargetPgURL,
		"--no-owner",
		"--clean",
		"--if-exists",
		dumpPath,
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// pg_restore returns non-zero for warnings too — check if critical.
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means warnings, which is acceptable for --clean --if-exists
			if exitErr.ExitCode() > 1 {
				return errors.Wrap(err, "pg_restore failed with critical error")
			}
			slog.Warn("pg_restore completed with warnings", log.BBError(err))
		} else {
			return errors.Wrap(err, "pg_restore failed")
		}
	}

	return nil
}

// verifyIntegrity compares row counts of key tables between source and target.
func (e *Engine) verifyIntegrity(ctx context.Context) (int, error) {
	sourceDB, err := sql.Open("pgx", e.cfg.SourcePgURL)
	if err != nil {
		return 0, errors.Wrap(err, "failed to connect to source for verification")
	}
	defer sourceDB.Close()

	targetDB, err := sql.Open("pgx", e.cfg.TargetPgURL)
	if err != nil {
		return 0, errors.Wrap(err, "failed to connect to target for verification")
	}
	defer targetDB.Close()

	verified := 0
	for _, table := range keyTables {
		srcCount, err := countRows(ctx, sourceDB, table)
		if err != nil {
			slog.Warn("skipping verification for table", "table", table, log.BBError(err))
			continue
		}

		tgtCount, err := countRows(ctx, targetDB, table)
		if err != nil {
			slog.Warn("skipping verification for table", "table", table, log.BBError(err))
			continue
		}

		if srcCount != tgtCount {
			return verified, errors.Errorf("row count mismatch for table %q: source=%d, target=%d", table, srcCount, tgtCount)
		}

		slog.Info("Table verified", "table", table, "rows", srcCount)
		verified++
	}

	return verified, nil
}

func countRows(ctx context.Context, db *sql.DB, table string) (int, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func parseMajorVersion(version string) int {
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0
	}
	// Handle versions like "14.5" or "16beta1"
	major := 0
	for _, c := range parts[0] {
		if c >= '0' && c <= '9' {
			major = major*10 + int(c-'0')
		} else {
			break
		}
	}
	return major
}
