# T-005-02: Backup Executor (pg_dump + Encryption)

| Field | Value |
|---|---|
| **Task ID** | T-005-02 |
| **Solution** | SOL-AVAIL-005 |
| **Depends On** | T-005-01 |
| **Target File** | `backend/runner/backup/executor.go` (NEW) |

---

## Objective

pg_dump wrapper: execute full backup, SHA256 checksum, optional AES-256-GCM encryption.

## Implementation

Xem SOL-AVAIL-005 §2.2. Key functions:

```go
type Executor struct { store *store.Store; profile *config.Profile }

func (e *Executor) ExecuteFullBackup(ctx) (*BackupResult, error)
// 1. pg_dump --format=custom --compress=9 --exclude-table=replica_heartbeat,bus_message
// 2. sha256File(path) → checksum
// 3. If encryption key set → encryptFile(src, dst, key) using AES-256-GCM
// 4. CreateBackupRecord to store

func sha256File(path string) (string, error)
func encryptFile(srcPath, dstPath, keyBase64 string) error
```

## Acceptance Criteria

- [ ] Calls `pg_dump` with custom format + compression
- [ ] SHA256 checksum for integrity
- [ ] AES-256-GCM encryption when `BB_BACKUP_ENCRYPTION_KEY` set
- [ ] Records result in backup_registry
- [ ] `go build ./backend/runner/backup/...` passes
