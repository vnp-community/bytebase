package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/dbmigrate"
)

var migrateDBFlags struct {
	targetURL string
	dryRun    bool
	backupDir string
}

func init() {
	migrateDBCmd := &cobra.Command{
		Use:   "migrate-db",
		Short: "Migrate Bytebase metadata from embedded PostgreSQL to an external PostgreSQL instance",
		Long: `Migrate the Bytebase metadata database from the embedded PostgreSQL to an external
PostgreSQL instance. This command performs a 6-step pipeline:

  1. Validate the target PostgreSQL (version ≥14, empty database)
  2. Create a backup of the current embedded data
  3. Dump the embedded PostgreSQL
  4. Restore the dump to the target
  5. Verify row-count integrity for key tables
  6. Report the PG_URL to use going forward

Use --dry-run to only validate the target without performing the migration.`,
		RunE: runMigrateDB,
	}

	migrateDBCmd.Flags().StringVar(&migrateDBFlags.targetURL, "target-url", "", "PostgreSQL connection URL for the migration target (required)")
	migrateDBCmd.Flags().BoolVar(&migrateDBFlags.dryRun, "dry-run", false, "Only validate the target PostgreSQL connection and exit")
	migrateDBCmd.Flags().StringVar(&migrateDBFlags.backupDir, "backup-dir", "", "Directory for backup files (default: {dataDir}/migration_backup)")

	_ = migrateDBCmd.MarkFlagRequired("target-url")

	rootCmd.AddCommand(migrateDBCmd)
}

func runMigrateDB(_ *cobra.Command, _ []string) error {
	if err := checkDataDir(); err != nil {
		return err
	}

	profile := activeProfile(flags.dataDir)

	// Only allow migration from embedded PG.
	if !profile.UseEmbedDB() {
		return fmt.Errorf("migrate-db is only available when using embedded PostgreSQL (PG_URL is not set). Current deployment already uses an external PostgreSQL")
	}

	backupDir := migrateDBFlags.backupDir
	if backupDir == "" {
		backupDir = filepath.Join(flags.dataDir, "migration_backup")
	}

	// Build source connection string for embedded PG.
	sourcePgURL := fmt.Sprintf("host=/tmp port=%d user=bb database=bb sslmode=disable", profile.DatastorePort)

	cfg := dbmigrate.Config{
		SourcePgURL:   sourcePgURL,
		TargetPgURL:   migrateDBFlags.targetURL,
		BackupDir:     backupDir,
		DryRun:        migrateDBFlags.dryRun,
		SourceDataDir: filepath.Join(flags.dataDir, "pgdata"),
	}

	engine := dbmigrate.NewEngine(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Print progress in a goroutine.
	go func() {
		for p := range engine.Progress() {
			fmt.Printf("[%s] %d%% — %s\n", p.Phase, p.Percent, p.Message)
		}
	}()

	result, err := engine.Run(ctx)
	if err != nil {
		slog.Error("Migration failed", log.BBError(err))
		return err
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════")
	fmt.Println("  Migration Complete!")
	fmt.Println("════════════════════════════════════════════════════")
	fmt.Printf("  Duration:      %s\n", result.Duration.Round(100*1000000)) // round to 100ms
	if result.BackupPath != "" {
		fmt.Printf("  Backup:        %s\n", result.BackupPath)
	}
	fmt.Printf("  Tables verified: %d\n", result.RowsVerified)
	fmt.Println()
	fmt.Println("  Next step: restart Bytebase with the external PG:")
	fmt.Printf("    PG_URL=%s bytebase\n", result.TargetURL)
	fmt.Println("════════════════════════════════════════════════════")
	fmt.Println()

	os.Exit(0)
	return nil
}
