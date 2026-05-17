# T-001-01: ReplicaNode Model + Migration

| Field | Value |
|---|---|
| **Task ID** | T-001-01 |
| **Solution** | SOL-AVAIL-001 |
| **Depends On** | None |
| **Target Files** | `backend/store/model/replica.go` (NEW), migration SQL |

---

## Objective

Tạo `ReplicaNode` struct và migration mở rộng bảng `replica_heartbeat`.

## Implementation

### 1. New: `backend/store/model/replica.go`

```go
package model

import "time"

type ReplicaNode struct {
    ReplicaID    string    `json:"replica_id"`
    EndpointURL  string    `json:"endpoint_url"`
    Version      string    `json:"version"`
    Status       string    `json:"status"`       // STARTING, READY, DRAINING, STOPPED, UNHEALTHY
    Capabilities []string  `json:"capabilities"` // API, RUNNER, LSP, MCP
    Metadata     string    `json:"metadata"`     // JSONB string
    StartedAt    time.Time `json:"started_at"`
    LastHeartbeat time.Time `json:"last_heartbeat"`
}
```

### 2. Migration SQL

```sql
ALTER TABLE replica_heartbeat
    ADD COLUMN IF NOT EXISTS endpoint_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'READY',
    ADD COLUMN IF NOT EXISTS capabilities TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_replica_heartbeat_status ON replica_heartbeat (status);
```

## Acceptance Criteria

- [x] `ReplicaNode` struct in `store/model/`
- [x] Migration extends `replica_heartbeat` with 6 new columns
- [x] `ADD COLUMN IF NOT EXISTS` for idempotent migration
- [x] `go build ./backend/store/...` passes
