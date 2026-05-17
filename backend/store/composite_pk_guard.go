package store

import (
	"log/slog"

	"github.com/pkg/errors"
)

// validateCompositePKQuery validates that a composite PK query has at least one
// meaningful filter (projectID or id). Tables with composite PKs (project, id)
// require either ProjectID for project-scoped queries or ID for direct lookups.
//
// Rules:
//   - Empty query (no projectID AND no id) → returns error to prevent full scans.
//   - ID-only query (no projectID) → logs warning, proceeds using unique id index.
//   - ProjectID set → normal scoped query, no warning.
func validateCompositePKQuery(entity string, projectID string, id *int64) error {
	hasProjectID := projectID != ""
	hasID := id != nil

	if !hasProjectID && !hasID {
		return errors.Errorf("%s query requires at least ProjectID or ID", entity)
	}

	if !hasProjectID && hasID {
		slog.Warn("composite PK query without ProjectID — using unique id index",
			slog.String("entity", entity),
			slog.Int64("id", *id),
		)
	}

	return nil
}
