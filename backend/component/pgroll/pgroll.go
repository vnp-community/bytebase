// Package pgroll provides PostgreSQL online (zero-downtime) schema change
// integration using the pgroll tool. This is analogous to the gh-ost component
// for MySQL, enabling expand-and-contract migrations for PostgreSQL.
//
// Flow:
//  1. DDL statement → ConvertDDLToPGRoll() → JSON migration definition
//  2. PGRoll.Start() executes the expand phase (dual-write)
//  3. PGRoll.Complete() executes the contract phase (drop old schema)
//
// Configuration:
//   - PGROLL_BINARY_PATH: Path to the pgroll binary (default: "pgroll")
//   - PGROLL_ENABLED: Enable pgroll integration (default: false)
package pgroll

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common/log"
)

// MigrationConfig contains the configuration for a pgroll migration.
type MigrationConfig struct {
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// MigrationName is a unique identifier for this migration.
	MigrationName string
	// Migration is the JSON migration definition.
	Migration json.RawMessage
	// Complete indicates whether to automatically run the contract phase.
	Complete bool
	// Timeout is the maximum duration for the migration.
	Timeout time.Duration
}

// PGRoll wraps the pgroll binary for zero-downtime PostgreSQL schema changes.
type PGRoll struct {
	binaryPath string
}

// New creates a new PGRoll instance. If binaryPath is empty, it defaults
// to the PGROLL_BINARY_PATH env var or "pgroll".
func New(binaryPath string) *PGRoll {
	if binaryPath == "" {
		binaryPath = os.Getenv("PGROLL_BINARY_PATH")
	}
	if binaryPath == "" {
		binaryPath = "pgroll"
	}
	return &PGRoll{binaryPath: binaryPath}
}

// IsEnabled checks if pgroll is available and enabled.
func IsEnabled() bool {
	enabled := os.Getenv("PGROLL_ENABLED")
	return enabled == "true" || enabled == "1"
}

// IsAvailable checks if the pgroll binary is accessible.
func (p *PGRoll) IsAvailable() bool {
	_, err := exec.LookPath(p.binaryPath)
	return err == nil
}

// Start executes the expand phase of a pgroll migration.
// This creates the new schema version while keeping the old one functional.
func (p *PGRoll) Start(ctx context.Context, config MigrationConfig) error {
	if config.DatabaseURL == "" {
		return errors.New("pgroll: database URL is required")
	}
	if config.Migration == nil {
		return errors.New("pgroll: migration definition is required")
	}

	// Write migration to temp file
	tmpFile, err := os.CreateTemp("", "pgroll-migration-*.json")
	if err != nil {
		return errors.Wrap(err, "pgroll: failed to create temp migration file")
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(config.Migration); err != nil {
		tmpFile.Close()
		return errors.Wrap(err, "pgroll: failed to write migration file")
	}
	tmpFile.Close()

	args := []string{
		"start",
		"--postgres-url", config.DatabaseURL,
		tmpFile.Name(),
	}

	return p.run(ctx, config.Timeout, args...)
}

// Complete executes the contract phase of a pgroll migration.
// This drops the old schema version, completing the migration.
func (p *PGRoll) Complete(ctx context.Context, config MigrationConfig) error {
	if config.DatabaseURL == "" {
		return errors.New("pgroll: database URL is required")
	}

	args := []string{
		"complete",
		"--postgres-url", config.DatabaseURL,
	}

	return p.run(ctx, config.Timeout, args...)
}

// Execute runs both start and complete phases sequentially.
// If the complete flag is false, only the start phase is executed.
func (p *PGRoll) Execute(ctx context.Context, config MigrationConfig) error {
	slog.Info("pgroll: starting expand phase",
		slog.String("migration", config.MigrationName),
	)

	if err := p.Start(ctx, config); err != nil {
		return errors.Wrap(err, "pgroll: expand phase failed")
	}

	slog.Info("pgroll: expand phase completed",
		slog.String("migration", config.MigrationName),
	)

	if config.Complete {
		slog.Info("pgroll: starting contract phase",
			slog.String("migration", config.MigrationName),
		)
		if err := p.Complete(ctx, config); err != nil {
			return errors.Wrap(err, "pgroll: contract phase failed")
		}
		slog.Info("pgroll: contract phase completed",
			slog.String("migration", config.MigrationName),
		)
	}

	return nil
}

// Rollback rolls back an in-progress pgroll migration.
func (p *PGRoll) Rollback(ctx context.Context, config MigrationConfig) error {
	args := []string{
		"rollback",
		"--postgres-url", config.DatabaseURL,
	}

	return p.run(ctx, config.Timeout, args...)
}

func (p *PGRoll) run(ctx context.Context, timeout time.Duration, args ...string) error {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.binaryPath, args...)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("pgroll command failed",
			slog.String("args", strings.Join(args, " ")),
			slog.String("output", string(output)),
			log.BBError(err),
		)
		return errors.Wrapf(err, "pgroll command failed: %s", string(output))
	}

	slog.Debug("pgroll command succeeded",
		slog.String("args", strings.Join(args, " ")),
		slog.String("output", string(output)),
	)
	return nil
}

// ─── DDL conversion ───────────────────────────────────────────

// AlterTableOperation represents a single ALTER TABLE operation that
// can be converted to a pgroll migration.
type AlterTableOperation struct {
	Type       string // "add_column", "drop_column", "alter_column", "rename_column"
	Table      string
	Column     string
	NewName    string // for rename
	ColumnType string // for add_column
	Nullable   bool
	Default    string
}

// ConvertDDLToPGRoll converts a DDL statement to a pgroll JSON migration.
// Returns nil if the DDL cannot be converted (caller should fall back to
// standard DDL execution).
func ConvertDDLToPGRoll(ddl string, migrationName string) (json.RawMessage, error) {
	upper := strings.ToUpper(strings.TrimSpace(ddl))

	if !strings.HasPrefix(upper, "ALTER TABLE") {
		// pgroll only handles ALTER TABLE operations
		return nil, nil
	}

	ops, err := parseAlterTable(ddl)
	if err != nil {
		return nil, nil // fall back to standard DDL
	}

	if len(ops) == 0 {
		return nil, nil
	}

	migration := map[string]any{
		"name": migrationName,
	}

	pgrollOps := make([]map[string]any, 0, len(ops))
	for _, op := range ops {
		pgOp := convertOperation(op)
		if pgOp != nil {
			pgrollOps = append(pgrollOps, pgOp)
		}
	}

	if len(pgrollOps) == 0 {
		return nil, nil
	}

	migration["operations"] = pgrollOps

	data, err := json.Marshal(migration)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal pgroll migration")
	}

	return data, nil
}

func convertOperation(op AlterTableOperation) map[string]any {
	switch op.Type {
	case "add_column":
		col := map[string]any{
			"name": op.Column,
			"type": op.ColumnType,
		}
		if !op.Nullable {
			col["nullable"] = false
		}
		if op.Default != "" {
			col["default"] = op.Default
		}
		return map[string]any{
			"add_column": map[string]any{
				"table":  op.Table,
				"column": col,
			},
		}
	case "drop_column":
		return map[string]any{
			"drop_column": map[string]any{
				"table":  op.Table,
				"column": op.Column,
			},
		}
	case "rename_column":
		return map[string]any{
			"rename_column": map[string]any{
				"table": op.Table,
				"from":  op.Column,
				"to":    op.NewName,
			},
		}
	case "alter_column":
		change := map[string]any{
			"table":  op.Table,
			"column": op.Column,
		}
		if op.ColumnType != "" {
			change["type"] = op.ColumnType
		}
		return map[string]any{
			"alter_column": change,
		}
	default:
		return nil
	}
}

// parseAlterTable is a simplified parser for ALTER TABLE statements.
// It handles the most common operations: ADD COLUMN, DROP COLUMN, RENAME COLUMN.
// Returns nil for complex ALTER TABLE statements that can't be decomposed.
func parseAlterTable(ddl string) ([]AlterTableOperation, error) {
	upper := strings.ToUpper(strings.TrimSpace(ddl))

	// Extract table name
	tableIdx := strings.Index(upper, "TABLE")
	if tableIdx < 0 {
		return nil, fmt.Errorf("no TABLE keyword found")
	}
	rest := strings.TrimSpace(ddl[tableIdx+5:])

	// Skip IF EXISTS
	restUpper := strings.ToUpper(rest)
	if strings.HasPrefix(restUpper, "IF EXISTS") {
		rest = strings.TrimSpace(rest[9:])
		restUpper = strings.ToUpper(rest)
	}

	// Extract table name
	spaceIdx := strings.IndexAny(rest, " \t\n")
	if spaceIdx < 0 {
		return nil, fmt.Errorf("no operation after table name")
	}
	tableName := strings.Trim(rest[:spaceIdx], "`\"")
	rest = strings.TrimSpace(rest[spaceIdx:])
	restUpper = strings.ToUpper(rest)

	var ops []AlterTableOperation

	if strings.HasPrefix(restUpper, "ADD COLUMN") {
		colDef := strings.TrimSpace(rest[10:])
		fields := strings.Fields(colDef)
		if len(fields) >= 2 {
			ops = append(ops, AlterTableOperation{
				Type:       "add_column",
				Table:      tableName,
				Column:     strings.Trim(fields[0], "`\""),
				ColumnType: fields[1],
				Nullable:   !strings.Contains(restUpper, "NOT NULL"),
			})
		}
	} else if strings.HasPrefix(restUpper, "DROP COLUMN") {
		colName := strings.TrimSpace(rest[11:])
		colName = strings.Fields(colName)[0]
		ops = append(ops, AlterTableOperation{
			Type:   "drop_column",
			Table:  tableName,
			Column: strings.Trim(colName, "`\";"),
		})
	} else if strings.HasPrefix(restUpper, "RENAME COLUMN") {
		parts := strings.Fields(rest[13:])
		if len(parts) >= 3 && strings.ToUpper(parts[1]) == "TO" {
			ops = append(ops, AlterTableOperation{
				Type:    "rename_column",
				Table:   tableName,
				Column:  strings.Trim(parts[0], "`\""),
				NewName: strings.Trim(parts[2], "`\";"),
			})
		}
	}

	return ops, nil
}
