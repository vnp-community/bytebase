package schemasync

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/plugin/db"
	"github.com/bytebase/bytebase/backend/store"
)

const forceFullSyncInterval = 24 * time.Hour

// shouldSkipSync compares the remote schema checksum against the stored value
// to determine if a full schema sync can be skipped. A force sync is triggered
// every 24h regardless of checksum match.
func (s *Syncer) shouldSkipSync(ctx context.Context, database *store.DatabaseMessage) bool {
	if database.Metadata == nil {
		return false
	}
	lastSync := database.Metadata.GetLastSyncTime()
	if !lastSync.IsValid() || time.Since(lastSync.AsTime()) > forceFullSyncInterval {
		return false
	}

	remoteChecksum, err := s.getRemoteSchemaChecksum(ctx, database)
	if err != nil {
		slog.Debug("Failed to get remote schema checksum, will sync",
			slog.String("database", database.DatabaseName), log.BBError(err))
		return false
	}

	// Compare with stored checksum from labels (lightweight storage)
	storedChecksum := database.Metadata.GetLabels()["bb-schema-checksum"]
	return storedChecksum != "" && remoteChecksum == storedChecksum
}

// getRemoteSchemaChecksum queries the target database for a structural checksum
// derived from table names and their column counts. This provides a lightweight
// change-detection signal without a full metadata fetch.
func (s *Syncer) getRemoteSchemaChecksum(ctx context.Context, database *store.DatabaseMessage) (string, error) {
	instance, err := s.store.GetInstanceByResourceID(ctx, database.InstanceID)
	if err != nil {
		return "", err
	}
	if instance == nil {
		return "", nil
	}
	driver, err := s.dbFactory.GetAdminDatabaseDriver(ctx, instance, database, db.ConnectionContext{})
	if err != nil {
		return "", err
	}
	defer driver.Close(ctx)

	rows, err := driver.QueryConn(ctx, nil, `
		SELECT md5(string_agg(
			t.table_name || ':' || t.column_count::text,
			'|' ORDER BY t.table_name
		))
		FROM (
			SELECT c.table_name, COUNT(cols.column_name) as column_count
			FROM information_schema.tables c
			LEFT JOIN information_schema.columns cols
				ON cols.table_schema = c.table_schema AND cols.table_name = c.table_name
			WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
			  AND c.table_type = 'BASE TABLE'
			GROUP BY c.table_name
		) t`, db.QueryContext{})
	if err != nil {
		return "", err
	}
	if len(rows) > 0 && len(rows[0].Rows) > 0 {
		return rows[0].Rows[0].Values[0].GetStringValue(), nil
	}

	// Fallback: hash an empty string
	h := sha256.Sum256([]byte(""))
	return fmt.Sprintf("%x", h[:8]), nil
}
