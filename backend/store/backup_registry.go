package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/bytebase/bytebase/backend/common/qb"
	"github.com/pkg/errors"
)

type BackupRecord struct {
	ID             int64
	BackupID       string
	BackupType     string
	Status         string
	SizeBytes      int64
	ChecksumSHA256 *string
	StoragePath    string
	StorageType    string
	Encryption     string
	DataTimestamp  time.Time
	VerifiedAt     *time.Time
	VerifyResult   *string
	ExpiresAt      time.Time
	CompletedAt    *time.Time
	ErrorMessage   *string
	CreatedAt      time.Time
}

func (s *Store) CreateBackupRecord(ctx context.Context, record *BackupRecord) error {
	q := qb.Q().Space(`
		INSERT INTO backup_registry (
			backup_id, backup_type, status, size_bytes, checksum_sha256,
			storage_path, storage_type, encryption, data_timestamp, expires_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id, created_at
	`, record.BackupID, record.BackupType, record.Status, record.SizeBytes, record.ChecksumSHA256,
		record.StoragePath, record.StorageType, record.Encryption, record.DataTimestamp, record.ExpiresAt)

	query, args, err := q.ToSQL()
	if err != nil {
		return errors.Wrap(err, "failed to build sql")
	}

	err = s.GetDB().QueryRowContext(ctx, query, args...).Scan(&record.ID, &record.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to insert backup record")
	}

	return nil
}

func (s *Store) GetLatestSuccessfulBackup(ctx context.Context) (*BackupRecord, error) {
	q := qb.Q().Space(`
		SELECT id, backup_id, backup_type, status, size_bytes, checksum_sha256,
		       storage_path, storage_type, encryption, data_timestamp, verified_at,
		       verify_result, expires_at, completed_at, error_message, created_at
		FROM backup_registry
		WHERE status = 'SUCCESS'
		ORDER BY data_timestamp DESC
		LIMIT 1
	`)

	query, args, err := q.ToSQL()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build sql")
	}

	var record BackupRecord
	err = s.GetDB().QueryRowContext(ctx, query, args...).Scan(
		&record.ID, &record.BackupID, &record.BackupType, &record.Status,
		&record.SizeBytes, &record.ChecksumSHA256, &record.StoragePath,
		&record.StorageType, &record.Encryption, &record.DataTimestamp,
		&record.VerifiedAt, &record.VerifyResult, &record.ExpiresAt,
		&record.CompletedAt, &record.ErrorMessage, &record.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get latest successful backup")
	}

	return &record, nil
}

func (s *Store) GetBackupRecord(ctx context.Context, backupID string) (*BackupRecord, error) {
	q := qb.Q().Space(`
		SELECT id, backup_id, backup_type, status, size_bytes, checksum_sha256,
		       storage_path, storage_type, encryption, data_timestamp, verified_at,
		       verify_result, expires_at, completed_at, error_message, created_at
		FROM backup_registry
		WHERE backup_id = ?
	`, backupID)

	query, args, err := q.ToSQL()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build sql")
	}

	var record BackupRecord
	err = s.GetDB().QueryRowContext(ctx, query, args...).Scan(
		&record.ID, &record.BackupID, &record.BackupType, &record.Status,
		&record.SizeBytes, &record.ChecksumSHA256, &record.StoragePath,
		&record.StorageType, &record.Encryption, &record.DataTimestamp,
		&record.VerifiedAt, &record.VerifyResult, &record.ExpiresAt,
		&record.CompletedAt, &record.ErrorMessage, &record.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get backup record")
	}

	return &record, nil
}

func (s *Store) UpdateBackupVerification(ctx context.Context, backupID string, status string, result string) error {
	q := qb.Q().Space(`
		UPDATE backup_registry
		SET verified_at = NOW(), status = ?, verify_result = ?
		WHERE backup_id = ?
	`, status, result, backupID)

	query, args, err := q.ToSQL()
	if err != nil {
		return errors.Wrap(err, "failed to build sql")
	}

	_, err = s.GetDB().ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update backup verification")
	}

	return nil
}
