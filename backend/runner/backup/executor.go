package backup

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/store"
)

type BackupResult struct {
	BackupID string
	Path     string
	Checksum string
	Size     int64
}

type Executor struct {
	store   *store.Store
	profile *config.Profile
}

func NewExecutor(st *store.Store, prof *config.Profile) *Executor {
	return &Executor{
		store:   st,
		profile: prof,
	}
}

func (e *Executor) ExecuteFullBackup(ctx context.Context) (*BackupResult, error) {
	backupID := "backup-" + time.Now().Format("20060102150405") + "-" + uuid.NewString()[:8]
	baseDir := e.profile.BackupPath
	if baseDir == "" {
		baseDir = filepath.Join(e.profile.DataDir, "backups")
	}
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup dir: %w", err)
	}

	backupPath := filepath.Join(baseDir, backupID+".dump")
	pgURL := e.profile.PgURL
	if pgURL == "" {
		// Embed DB. Use fallback or error.
		return nil, fmt.Errorf("embedded DB backup not implemented in this executor")
	}

	// 1. pg_dump
	cmd := exec.CommandContext(ctx, "pg_dump",
		"--dbname="+pgURL,
		"--format=custom",
		"--compress=9",
		"--exclude-table=replica_heartbeat",
		"--exclude-table=bus_message",
		"--file="+backupPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pg_dump failed: %w, output: %s", err, output)
	}

	// Ensure we clean up if we encrypt and move
	defer func() {
		// We could clean up temp files if needed
	}()

	finalPath := backupPath
	encryption := "NONE"

	// 3. Encrypt if key is set
	if key := e.profile.BackupEncryptionKey; key != "" {
		encryptedPath := backupPath + ".enc"
		if err := encryptFile(backupPath, encryptedPath, key); err != nil {
			os.Remove(backupPath)
			return nil, fmt.Errorf("failed to encrypt backup: %w", err)
		}
		os.Remove(backupPath)
		finalPath = encryptedPath
		encryption = "AES-256-GCM"
	}

	// 2. SHA256
	checksum, err := sha256File(finalPath)
	if err != nil {
		os.Remove(finalPath)
		return nil, fmt.Errorf("failed to checksum backup: %w", err)
	}

	fi, err := os.Stat(finalPath)
	if err != nil {
		os.Remove(finalPath)
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	// 4. CreateBackupRecord
	record := &store.BackupRecord{
		BackupID:       backupID,
		BackupType:     "FULL",
		Status:         "SUCCESS",
		SizeBytes:      fi.Size(),
		ChecksumSHA256: &checksum,
		StoragePath:    finalPath,
		StorageType:    "LOCAL",
		Encryption:     encryption,
		DataTimestamp:  time.Now(),
		ExpiresAt:      time.Now().AddDate(0, 1, 0), // 30 days
	}
	now := time.Now()
	record.CompletedAt = &now

	if err := e.store.CreateBackupRecord(ctx, record); err != nil {
		os.Remove(finalPath)
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	return &BackupResult{
		BackupID: backupID,
		Path:     finalPath,
		Checksum: checksum,
		Size:     fi.Size(),
	}, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func encryptFile(srcPath, dstPath, keyStr string) error {
	// keyStr should ideally be 32 bytes for AES-256.
	// For simplicity, we hash it to get exactly 32 bytes.
	keyHash := sha256.Sum256([]byte(keyStr))
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	ciphertext := gcm.Seal(nonce, nonce, srcData, nil)
	return os.WriteFile(dstPath, ciphertext, 0600)
}
