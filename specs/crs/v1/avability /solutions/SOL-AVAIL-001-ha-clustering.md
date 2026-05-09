# Solution: HA Active-Active Clustering & Zero-Downtime Deployment

| Field          | Value                                    |
|----------------|------------------------------------------|
| **Solution ID**| SOL-AVAIL-001                            |
| **CR ID**      | CR-AVAIL-001                             |
| **Status**     | Draft                                    |
| **Created**    | 2026-05-08                               |
| **Layers**     | L2 (API Gateway), L6 (Runner), L8 (Store), L10 (Infra) |

---

## 1. Analysis — Existing Infrastructure

### 1.1 Điểm tận dụng (đã có trong codebase)

| Component | File | Capability |
|---|---|---|
| **Heartbeat Runner** | `backend/runner/heartbeat/runner.go` | 10s heartbeat loop, `UpsertReplicaHeartbeat()` |
| **Replica Heartbeat Store** | `backend/store/replica_heartbeat.go` | `UpsertReplicaHeartbeat`, `DeleteStaleReplicaHeartbeats`, `CountActiveReplicas` |
| **Advisory Locks** | `backend/store/advisory_lock.go` | `TryAdvisoryLock`, `AcquireAdvisoryLock` — session-level PG locks |
| **HA Mode Flag** | `backend/component/config/` → `profile.HA` | HA mode detection, disables embedded PG & cache |
| **ReplicaID** | `profile.ReplicaID` | Unique identifier per replica (logged at startup) |
| **Graceful Shutdown** | `backend/server/server.go:313` | 10s `gracefulShutdownPeriod`, `http.Server.Shutdown()`, `runnerWG.Wait()` |
| **Health Endpoint** | `backend/server/echo_routes.go:75` | `/healthz` returning plain "OK" |
| **PG LISTEN/NOTIFY** | `backend/runner/notifylistener/` | Cross-instance event bridge |

### 1.2 Gaps cần giải quyết

| Gap | Current State | Required State |
|---|---|---|
| Cluster awareness | `replica_heartbeat` chỉ có `replica_id` + `last_heartbeat` | Node metadata: endpoint, version, status, capabilities |
| Readiness probe | `/healthz` chỉ return "OK" (no checks) | `/readyz` kiểm tra DB, cache, migration status |
| Shutdown ordering | Cancel → HTTP shutdown → wait runners → close DB | Stop accepting → drain → notify cluster → wait → cleanup |
| Session affinity | None | Cookie-based cho WebSocket, IP-hash cho LSP |
| Rolling update | No support | K8s spec: maxUnavailable=0, maxSurge=1 |

---

## 2. Giải pháp kỹ thuật

### 2.1 Enhanced Cluster Registry — Mở rộng `replica_heartbeat`

**Approach**: Mở rộng bảng `replica_heartbeat` hiện có thay vì tạo bảng mới → backward compatible, không cần migration phức tạp.

```sql
-- Migration: extend replica_heartbeat
ALTER TABLE replica_heartbeat
    ADD COLUMN IF NOT EXISTS endpoint_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'READY',
    ADD COLUMN IF NOT EXISTS capabilities TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_replica_heartbeat_status
    ON replica_heartbeat (status);
```

**File changes**:

```go
// backend/store/replica_heartbeat.go — Enhanced
func (s *Store) UpsertReplicaHeartbeat(ctx context.Context, node *ReplicaNode) error {
    q := qb.Q().Space(`
        INSERT INTO replica_heartbeat (replica_id, endpoint_url, version, status,
            capabilities, metadata, started_at, last_heartbeat)
        VALUES (?, ?, ?, ?, ?, ?, ?, now())
        ON CONFLICT (replica_id)
        DO UPDATE SET last_heartbeat = now(),
            status = EXCLUDED.status,
            endpoint_url = EXCLUDED.endpoint_url
    `, node.ReplicaID, node.EndpointURL, node.Version, node.Status,
       pq.Array(node.Capabilities), node.Metadata, node.StartedAt)
    // ...
}

// New: Mark stale nodes
func (s *Store) MarkStaleReplicas(ctx context.Context, threshold time.Duration) (int64, error) {
    q := qb.Q().Space(`
        UPDATE replica_heartbeat
        SET status = 'UNHEALTHY'
        WHERE status IN ('STARTING', 'READY')
        AND last_heartbeat < now() - ?::INTERVAL
    `, threshold.String())
    // ...
}

// New: List active nodes
func (s *Store) ListActiveReplicas(ctx context.Context, within time.Duration) ([]*ReplicaNode, error) {
    // SELECT * FROM replica_heartbeat WHERE last_heartbeat > now() - within
}
```

### 2.2 Enhanced Heartbeat Runner — Cluster Registration

**File**: `backend/runner/heartbeat/runner.go` — Extend existing runner.

```go
// runner.go — Enhanced
type Runner struct {
    store   *store.Store
    profile *config.Profile
    node    *store.ReplicaNode  // NEW: full node metadata
}

func NewRunner(store *store.Store, profile *config.Profile) *Runner {
    return &Runner{
        store:   store,
        profile: profile,
        node: &store.ReplicaNode{
            ReplicaID:    profile.ReplicaID,
            EndpointURL:  profile.ExternalURL,
            Version:      profile.Version,
            Status:       "STARTING",
            Capabilities: []string{"API", "RUNNER", "LSP", "MCP"},
            StartedAt:    time.Now(),
        },
    }
}

func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(heartbeatInterval)
    defer ticker.Stop()

    // Register on startup
    r.node.Status = "READY"
    r.sendHeartbeat(ctx)

    // Stale check on startup
    r.store.MarkStaleReplicas(ctx, 30*time.Second)

    for {
        select {
        case <-ticker.C:
            r.sendHeartbeat(ctx)
            // Periodic stale cleanup (leader only)
            r.cleanupIfLeader(ctx)
        case <-ctx.Done():
            // Mark as DRAINING before exit
            r.node.Status = "STOPPED"
            r.sendHeartbeat(context.Background())
            return
        }
    }
}

func (r *Runner) sendHeartbeat(ctx context.Context) {
    if err := r.store.UpsertReplicaHeartbeat(ctx, r.node); err != nil {
        slog.Error("Failed to send heartbeat", log.BBError(err))
    }
}
```

### 2.3 Readiness Probe — `/readyz` Endpoint

**File**: `backend/server/echo_routes.go` — Add alongside existing `/healthz`.

```go
// echo_routes.go — Add readiness probe
e.GET("/readyz", func(c *echo.Context) error {
    checks := s.checkReadiness(c.Request().Context())
    allHealthy := true
    for _, check := range checks {
        if !check.Healthy {
            allHealthy = false
        }
    }
    status := http.StatusOK
    if !allHealthy {
        status = http.StatusServiceUnavailable
    }
    return c.JSON(status, map[string]any{
        "status": ternary(allHealthy, "ready", "not_ready"),
        "checks": checks,
    })
})

// New: /healthz/cluster endpoint (admin)
e.GET("/healthz/cluster", func(c *echo.Context) error {
    nodes, _ := s.store.ListActiveReplicas(c.Request().Context(), 30*time.Second)
    return c.JSON(http.StatusOK, map[string]any{
        "nodes": nodes,
        "total": len(nodes),
    })
})
```

**Readiness check implementation** — New file `backend/server/readiness.go`:

```go
package server

type ReadinessCheck struct {
    Name    string `json:"name"`
    Healthy bool   `json:"healthy"`
    Latency string `json:"latency,omitempty"`
    Message string `json:"message,omitempty"`
}

func (s *Server) checkReadiness(ctx context.Context) []ReadinessCheck {
    var checks []ReadinessCheck

    // 1. PostgreSQL connectivity
    start := time.Now()
    if err := s.store.GetDB().PingContext(ctx); err != nil {
        checks = append(checks, ReadinessCheck{"postgresql", false, "", err.Error()})
    } else {
        checks = append(checks, ReadinessCheck{"postgresql", true, time.Since(start).String(), ""})
    }

    // 2. Schema migration complete
    migrationOK := true // Set by migrator during startup
    checks = append(checks, ReadinessCheck{"migration", migrationOK, "", ""})

    // 3. License service initialized
    checks = append(checks, ReadinessCheck{"license", s.licenseService != nil, "", ""})

    // 4. At least one runner active in cluster (via advisory lock check)
    activeReplicas, _ := s.store.CountActiveReplicas(ctx, 30*time.Second)
    checks = append(checks, ReadinessCheck{
        "cluster", activeReplicas > 0, "",
        fmt.Sprintf("%d active replicas", activeReplicas),
    })

    return checks
}
```

### 2.4 Enhanced Graceful Shutdown

**File**: `backend/server/server.go` — Modify existing `Shutdown()`.

Key changes:
1. Mark node as DRAINING before stopping HTTP server
2. Extend drain timeout from 10s → 30s (configurable)
3. Flush audit log buffer
4. Close WebSocket/LSP connections with reconnect hint

```go
const (
    gracefulShutdownPeriod = 30 * time.Second  // Was 10s
)

func (s *Server) Shutdown(ctx context.Context) error {
    slog.Info("Stopping Bytebase...")

    // Step 1: Mark node as DRAINING in cluster registry
    if s.heartbeatRunner != nil {
        s.heartbeatRunner.SetStatus("DRAINING")
        s.heartbeatRunner.SendHeartbeat(context.Background())
    }

    ctx, cancel := context.WithTimeout(ctx, gracefulShutdownPeriod)
    defer cancel()

    // Step 2: Stop accepting new connections
    // (Echo v5 Shutdown handles this)

    // Step 3: Close LSP WebSocket connections with reconnect signal
    if s.lspServer != nil {
        s.lspServer.CloseWithReconnect()  // NEW method
    }

    // Step 4: Shutdown HTTP server (drains active connections)
    if s.httpServer != nil {
        if err := s.httpServer.Shutdown(ctx); err != nil {
            slog.Error("Failed to shutdown HTTP server", log.BBError(err))
        }
    }

    // Step 5: Cancel runners
    if s.cancel != nil {
        s.cancel()
    }

    // Step 6: Wait for all runners to exit
    s.runnerWG.Wait()

    // Step 7: Mark node as STOPPED
    if s.heartbeatRunner != nil {
        s.heartbeatRunner.SetStatus("STOPPED")
        s.heartbeatRunner.SendHeartbeat(context.Background())
    }

    // Step 8: Close DB connection
    if s.store != nil {
        if err := s.store.Close(); err != nil {
            return err
        }
    }

    // Step 9: Stop embedded PG and sample instances
    if s.sampleInstanceManager != nil {
        s.sampleInstanceManager.Stop()
    }
    for _, stopper := range s.stopper {
        stopper()
    }

    slog.Info("Bytebase stopped gracefully")
    return nil
}
```

### 2.5 Kubernetes Deployment Spec

**File**: `deploy/k8s/deployment.yaml` (new)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bytebase
  labels:
    app: bytebase
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0    # Zero downtime: never remove a pod before new is ready
      maxSurge: 1           # One extra pod during rolling update
  selector:
    matchLabels:
      app: bytebase
  template:
    metadata:
      labels:
        app: bytebase
    spec:
      terminationGracePeriodSeconds: 60  # Must > gracefulShutdownPeriod (30s)
      containers:
      - name: bytebase
        image: bytebase/bytebase:latest
        ports:
        - containerPort: 8080
        env:
        - name: PG_URL
          valueFrom:
            secretKeyRef:
              name: bytebase-secret
              key: pg-url
        - name: BB_HA
          value: "true"
        - name: REPLICA_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name  # Use pod name as replica ID
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 5
          failureThreshold: 3
          successThreshold: 1
        lifecycle:
          preStop:
            exec:
              # Sleep allows K8s to update endpoints before traffic stops
              command: ["/bin/sh", "-c", "sleep 5"]
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "2"
            memory: "2Gi"
```

### 2.6 WebSocket/LSP Reconnect Protocol

**File**: `backend/api/lsp/server.go` — Add reconnect signaling.

```go
// CloseWithReconnect sends WebSocket close frames with a specific code
// indicating clients should reconnect (e.g., during rolling update).
func (s *Server) CloseWithReconnect() {
    s.mu.Lock()
    defer s.mu.Unlock()
    for _, conn := range s.activeConns {
        // 4000 = custom close code indicating "server going away, please reconnect"
        msg := websocket.FormatCloseMessage(4000, "server_restarting")
        conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(5*time.Second))
        conn.Close()
    }
}
```

**Frontend**: `frontend/src/` — Add exponential backoff reconnect.

```typescript
// LSP/WebSocket reconnect logic
class ReconnectingWebSocket {
  private retryDelay = 1000; // Start 1s
  private maxDelay = 30000;  // Max 30s
  
  private onClose(event: CloseEvent) {
    if (event.code === 4000) {
      // Server restarting — reconnect immediately
      this.retryDelay = 500;
    }
    setTimeout(() => this.connect(), this.retryDelay);
    this.retryDelay = Math.min(this.retryDelay * 2, this.maxDelay);
  }
}
```

---

## 3. File Change Summary

| Layer | File | Change Type | Description |
|---|---|---|---|
| L8 | `backend/store/replica_heartbeat.go` | **Modify** | Add node metadata, `MarkStaleReplicas`, `ListActiveReplicas` |
| L8 | `backend/store/model/replica.go` | **New** | `ReplicaNode` struct with full metadata |
| L6 | `backend/runner/heartbeat/runner.go` | **Modify** | Cluster registration, DRAINING status on shutdown |
| L2 | `backend/server/echo_routes.go` | **Modify** | Add `/readyz`, `/healthz/cluster` |
| L2 | `backend/server/readiness.go` | **New** | Readiness check logic |
| L2 | `backend/server/server.go` | **Modify** | Enhanced graceful shutdown (30s, ordered steps) |
| L1 | `backend/api/lsp/server.go` | **Modify** | `CloseWithReconnect()` method |
| L10 | `backend/migrator/migration/X.Y.Z/` | **New** | ALTER TABLE replica_heartbeat |
| L10 | `deploy/k8s/deployment.yaml` | **New** | K8s deployment with zero-downtime spec |
| L1 | `frontend/src/` | **Modify** | WebSocket reconnect with backoff |

---

## 4. Backward Compatibility

| Scenario | Behavior |
|---|---|
| Single-node (non-HA) | Heartbeat runner still sends heartbeats (with defaults), /readyz works, no cluster overhead |
| HA mode existing | Extends `replica_heartbeat` table transparently via migration |
| Rolling update without K8s | Manual process: start new → verify /readyz → stop old. Graceful shutdown prevents request loss |
| Older clients | `/healthz` unchanged. New endpoints are additive |

---

## 5. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Migration alters `replica_heartbeat` during upgrade | Migration uses `ADD COLUMN IF NOT EXISTS` — safe for re-runs |
| Shutdown timeout too short | Configurable via `BB_GRACEFUL_SHUTDOWN_SECONDS` env var (default 30) |
| Stale node cleanup race | Only leader (via advisory lock) runs cleanup |
