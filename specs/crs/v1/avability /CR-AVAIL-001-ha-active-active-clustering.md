# Change Request: HA Active-Active Clustering & Zero-Downtime Deployment

| Field              | Value                                                       |
|--------------------|-------------------------------------------------------------|
| **CR ID**          | CR-AVAIL-001                                                |
| **Title**          | HA Active-Active Clustering & Zero-Downtime Deployment      |
| **Category**       | Availability / Infrastructure                               |
| **Priority**       | P0 — Critical                                               |
| **Status**         | Draft                                                       |
| **Created**        | 2026-05-08                                                  |
| **Author**         | VNP AI Ops Team                                             |
| **Regulatory**     | FFIEC BCM, PCI-DSS 4.0 Req 1.3.4, SBV TT09/2020 Điều 10   |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai kiến trúc **Active-Active HA Clustering** cho Bytebase, cho phép nhiều instance hoạt động đồng thời xử lý request, kết hợp **zero-downtime deployment** (rolling update) để đảm bảo SLA ≥ 99.95% theo quy định ngành tài chính.

### 1.2 Bối cảnh
Hiện tại Bytebase hỗ trợ HA mode cơ bản nhưng có các hạn chế nghiêm trọng:
- **Cache disabled** trong HA mode → latency tăng 3-5x (TDD §4.2)
- **Background runners** chạy duplicate trên mọi replica → resource waste, race conditions
- **Không có graceful shutdown** orchestration → request bị drop khi deploy
- **Không có session affinity** hoặc sticky routing → WebSocket/LSP disconnect khi failover
- **Deployment requires downtime** — không hỗ trợ rolling update strategy

### 1.3 Mục tiêu
- Đạt SLA **99.95%** (≤ 26.3 phút downtime/năm) — tiêu chuẩn bắt buộc cho hệ thống core banking
- Zero-downtime deployment qua rolling update strategy
- Active-Active clustering với request load balancing
- Session persistence cho WebSocket/LSP connections
- Graceful shutdown với connection draining

### 1.4 Tiêu chuẩn áp dụng

| Standard                          | Requirement                                          |
|-----------------------------------|------------------------------------------------------|
| FFIEC IT Handbook — BCM           | Resilient infrastructure, no single point of failure |
| PCI-DSS 4.0 — Req 1.3.4          | HA for systems processing cardholder data            |
| SBV TT09/2020 — Điều 10          | Đảm bảo tính liên tục hoạt động CNTT                |
| ISO 27001 — A.17.1               | Information security continuity                      |

---

## 2. Yêu cầu chức năng

### FR-001: Active-Active Cluster Registration
- **Mô tả**: Mỗi Bytebase instance tự register vào cluster registry khi khởi động, deregister khi shutdown.
- **Logic**:
  ```
  ON server.Start():
      nodeID = generateNodeID(hostname, port, startTime)
      clusterRegistry.Register(nodeID, {
          endpoint:    selfURL,
          version:     buildVersion,
          startTime:   now(),
          status:      STARTING,
          capabilities: [API, RUNNER, LSP, MCP]
      })
      // Heartbeat every 10s
      startHeartbeatLoop(nodeID, interval=10s)

  ON server.Shutdown():
      clusterRegistry.UpdateStatus(nodeID, DRAINING)
      waitForDraining(timeout=30s)  // drain active connections
      clusterRegistry.Deregister(nodeID)
  ```
- **Registry Backend**: PostgreSQL table `cluster_node` (leverages existing infra)
- **Acceptance Criteria**:
  - AC-1: Node registration completes within 5 seconds of startup
  - AC-2: Stale nodes (no heartbeat > 30s) auto-marked as UNHEALTHY
  - AC-3: Cluster state queryable via admin API `/v1/cluster/nodes`
  - AC-4: Node deregistration is atomic — no partial state

### FR-002: Zero-Downtime Rolling Update
- **Mô tả**: Hỗ trợ rolling update strategy — deploy instance mới trước khi shutdown instance cũ.
- **Logic**:
  ```
  Rolling Update Sequence (N replicas):
  FOR i = 0 to N-1:
      1. Start new instance (version V+1)
      2. Wait for health check READY (readiness probe)
      3. Add new instance to load balancer
      4. Mark old instance[i] as DRAINING
      5. Wait for connection drain (max 30s)
      6. Remove old instance[i] from load balancer
      7. Shutdown old instance[i]
      8. Verify cluster health (minimum N-1 healthy nodes)
  ```
- **Acceptance Criteria**:
  - AC-1: Không có request nào bị drop trong quá trình rolling update
  - AC-2: API response latency tăng ≤ 20% trong quá trình update
  - AC-3: WebSocket/LSP connections gracefully migrated (reconnect protocol)
  - AC-4: Rollback tự động nếu instance mới fail health check trong 60s

### FR-003: Graceful Shutdown & Connection Draining
- **Mô tả**: Implement graceful shutdown sequence đảm bảo không mất request khi instance shutdown.
- **Logic**:
  ```
  ON SIGTERM/SIGINT:
      1. Stop accepting new connections (readiness=false)
      2. Complete in-flight requests (max wait: 30s)
      3. Close WebSocket/LSP connections with reconnect signal
      4. Flush audit log buffer to database
      5. Release leader election locks (if held)
      6. Deregister from cluster registry
      7. Close database connections
      8. Exit cleanly
  ```
- **Acceptance Criteria**:
  - AC-1: Graceful shutdown completes within 60 seconds
  - AC-2: Zero in-flight request loss during shutdown
  - AC-3: WebSocket clients receive close frame with reconnect hint
  - AC-4: Background task runs complete or hand off cleanly

### FR-004: Session Affinity cho Stateful Connections
- **Mô tả**: Stateful connections (WebSocket, LSP, MCP streaming) route tới cùng một node.
- **Logic**:
  ```
  Connection routing strategy:
  - HTTP API requests: Round-robin load balancing (stateless)
  - WebSocket (/v1:adminExecute): Cookie-based session affinity
  - LSP (/lsp): IP-hash based routing
  - MCP SSE (/mcp/sse): Connection-ID based routing
  ```
- **Acceptance Criteria**:
  - AC-1: WebSocket connections maintain affinity across multiple requests
  - AC-2: LSP sessions survive connection blips (< 5s reconnect)
  - AC-3: MCP SSE streams auto-reconnect to same or new node
  - AC-4: Load distribution variance ≤ 15% across nodes

### FR-005: Cluster-Aware Readiness & Liveness Probes
- **Mô tả**: Kubernetes-compatible probes cho orchestrated deployments.
- **Endpoints**:
  ```
  GET /healthz         → Liveness probe (basic process health)
  GET /readyz          → Readiness probe (full service ready)
  GET /healthz/cluster → Cluster health overview
  ```
- **Readiness Conditions**:
  - PostgreSQL connection active
  - Schema migration completed
  - License validated
  - Cache initialized (Redis if HA mode)
  - At least 1 leader-elected runner active in cluster
- **Acceptance Criteria**:
  - AC-1: Liveness probe responds < 100ms
  - AC-2: Readiness probe accurately reflects service readiness
  - AC-3: Cluster health endpoint shows all node statuses
  - AC-4: Probes compatible with Kubernetes deployment spec

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|---------------------------------------------------|
| Cluster Registry             | `backend/component/cluster/registry.go`   | PG-based cluster node registration                |
| Cluster Node Model           | `backend/component/cluster/node.go`       | Node status, capabilities, health                 |
| Heartbeat Runner             | `backend/runner/heartbeat/heartbeat.go`   | Enhanced heartbeat with cluster registration      |
| Graceful Shutdown            | `backend/server/shutdown.go`              | Connection draining, ordered shutdown             |
| Readiness Probe              | `backend/server/probe.go`                 | /readyz endpoint with dependency checks           |
| Cluster Health API           | `backend/api/v1/cluster_service.go`       | Cluster status API (admin only)                   |
| Echo Routes                  | `backend/server/echo_routes.go`           | Add /readyz, /healthz/cluster endpoints           |
| Server Lifecycle             | `backend/server/server.go`               | Wire cluster registry into server lifecycle       |
| WebSocket Reconnect          | `backend/server/ws_handler.go`            | Close frame with reconnect hint                   |

### 3.2 Configuration

| Environment Variable       | Default     | Mô tả                                                |
|----------------------------|-------------|-------------------------------------------------------|
| `CLUSTER_ENABLED`          | `false`     | Enable cluster mode                                   |
| `CLUSTER_NODE_ID`          | _(auto)_    | Unique node identifier (auto: hostname+port+pid)      |
| `CLUSTER_SELF_URL`         | _(auto)_    | Advertised URL for this node                          |
| `CLUSTER_HEARTBEAT_SEC`    | `10`        | Heartbeat interval in seconds                         |
| `CLUSTER_DRAIN_TIMEOUT_SEC`| `30`        | Connection drain timeout on shutdown                  |
| `CLUSTER_STALE_THRESHOLD`  | `30`        | Seconds before marking node as UNHEALTHY              |

### 3.3 Database Changes

```sql
-- Cluster node registry
CREATE TABLE IF NOT EXISTS cluster_node (
    node_id         TEXT        PRIMARY KEY,
    endpoint_url    TEXT        NOT NULL,
    version         TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'STARTING',
    -- Status: STARTING, READY, DRAINING, UNHEALTHY, STOPPED
    capabilities    TEXT[]      NOT NULL DEFAULT '{}',
    metadata        JSONB       NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_heartbeat  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cluster_node_status ON cluster_node (status);
CREATE INDEX idx_cluster_node_heartbeat ON cluster_node (last_heartbeat);

-- Auto-mark stale nodes
CREATE OR REPLACE FUNCTION mark_stale_cluster_nodes()
RETURNS void AS $$
BEGIN
    UPDATE cluster_node
    SET status = 'UNHEALTHY',
        updated_at = NOW()
    WHERE status IN ('STARTING', 'READY')
      AND last_heartbeat < NOW() - INTERVAL '30 seconds';
END;
$$ LANGUAGE plpgsql;
```

### 3.4 Kubernetes Deployment Spec (Reference)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bytebase
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0        # Zero downtime
      maxSurge: 1              # One extra pod during update
  template:
    spec:
      terminationGracePeriodSeconds: 60
      containers:
      - name: bytebase
        env:
        - name: CLUSTER_ENABLED
          value: "true"
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
          initialDelaySeconds: 10
          periodSeconds: 5
          failureThreshold: 3
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 5"]  # Allow LB to deregister
```

### 3.5 Frontend Changes

| Component                    | Thay đổi                                          |
|------------------------------|---------------------------------------------------|
| WebSocket reconnect logic    | Auto-reconnect with exponential backoff (1s→30s)  |
| LSP client reconnect        | Reconnect on close frame, preserve editor state   |
| API error handling           | Retry 503 (Service Unavailable) with backoff      |
| Connection status indicator  | UI badge showing connection health                |

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| CR-LIM-001              | Distributed cache (Redis) — prerequisite for HA cache           |
| CR-LIM-002              | Persistent message bus — complements cluster coordination       |
| PostgreSQL 14+          | Cluster registry, advisory locks                                |
| Kubernetes 1.28+        | Orchestration for rolling updates (recommended)                 |
| Load Balancer (L7)      | Nginx/Envoy/HAProxy with health check support                   |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                         |
|------------|-----------------------------------------------------------------|-----------------------------------------|
| TC-001     | Start 3 instances, verify all register in cluster_node         | 3 READY nodes in registry               |
| TC-002     | Kill 1 instance, verify auto-marked UNHEALTHY                 | Status → UNHEALTHY within 30s           |
| TC-003     | Rolling update: deploy v2 while v1 serving traffic             | Zero dropped requests                   |
| TC-004     | Graceful shutdown: send SIGTERM during active requests         | All in-flight requests complete          |
| TC-005     | WebSocket reconnect: kill node with active WS connections      | Clients reconnect to healthy node       |
| TC-006     | Readiness probe: DB unavailable                                | /readyz returns 503                      |
| TC-007     | Readiness probe: all dependencies healthy                      | /readyz returns 200                      |
| TC-008     | Load test: 1000 req/s across 3 nodes for 1 hour               | P99 < 200ms, zero errors                |
| TC-009     | Connection drain: SIGTERM with 50 active connections           | All 50 complete within 30s              |
| TC-010     | Cluster health API: mixed healthy/unhealthy nodes              | Accurate status for all nodes           |
| TC-011     | Stale node cleanup: node crashed without deregister            | Auto-cleaned after threshold            |
| TC-012     | SLA validation: simulate monthly uptime                        | ≥ 99.95% availability                   |

---

## 6. Performance Targets

| Metric                        | Current              | Target (HA Cluster)     |
|-------------------------------|----------------------|-------------------------|
| Availability SLA              | ~99% (estimated)     | ≥ 99.95%               |
| Rolling update downtime       | Minutes (manual)     | 0 seconds              |
| Shutdown drain time           | Immediate (abort)    | ≤ 30 seconds           |
| Health check response         | N/A (/readyz)        | < 100ms                |
| Cross-node latency            | N/A                  | < 2ms (same DC)        |
| Node registration time        | N/A                  | < 5 seconds            |
| Failover detection time       | N/A                  | < 30 seconds           |

---

## 7. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Cluster registry + heartbeat runner              | Sprint 1       |
| Phase 2 | Readiness/liveness probes enhancement            | Sprint 1       |
| Phase 3 | Graceful shutdown with connection draining        | Sprint 2       |
| Phase 4 | WebSocket/LSP reconnect protocol                 | Sprint 2       |
| Phase 5 | Kubernetes deployment spec + Helm chart          | Sprint 3       |
| Phase 6 | Load testing & SLA validation                   | Sprint 3-4     |
| Phase 7 | Documentation & runbook                          | Sprint 4       |

---

## 8. Risks & Mitigations

| Risk                                         | Impact | Mitigation                                                |
|----------------------------------------------|--------|-----------------------------------------------------------|
| Split-brain trong cluster registry           | HIGH   | PG advisory locks + serial transaction isolation          |
| WebSocket reconnect gây duplicate operations | MEDIUM | Idempotency keys cho SQL execution                        |
| Rolling update thời gian kéo dài            | MEDIUM | Parallel surge strategy (maxSurge > 1)                    |
| Connection drain timeout exceeded            | LOW    | Force-close after timeout + alert                         |
| Load balancer health check lag               | MEDIUM | Short probe interval (5s) + immediate deregister          |

---

## 9. Compliance Checklist

| # | Requirement                                        | Status  | Evidence                        |
|---|----------------------------------------------------|---------|---------------------------------|
| 1 | SLA ≥ 99.95% documented and measurable            | PENDING | TC-012, monitoring dashboard    |
| 2 | Zero single point of failure                       | PENDING | Architecture diagram            |
| 3 | Rolling update with zero downtime                  | PENDING | TC-003, deployment spec         |
| 4 | Graceful shutdown procedure                        | PENDING | TC-004, TC-009                  |
| 5 | Health monitoring probes                           | PENDING | TC-006, TC-007                  |
| 6 | Cluster status visibility                          | PENDING | TC-010, admin API               |
| 7 | Automated stale node detection                     | PENDING | TC-011                          |
