# Change Request: Cross-Platform Secret Distribution Agent

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SHR-104                                               |
| **Title**          | Cross-Platform Secret Distribution Agent                 |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Extends**        | CR-SHR-001, CR-SHR-005 (Notification)                    |

---

## 1. Mô tả

Agent tự động phân phối secrets từ Bytebase tới hệ thống tiêu thụ (CI/CD, monitoring, K8s) khi credentials rotate. Đảm bảo zero-touch credential distribution.

### 1.1 Vấn đề

Khi DBA rotate database password:
1. Bytebase cập nhật ✅
2. Vaultwarden sync ✅ (CR-SHR-101)
3. CI/CD, Grafana, K8s, backup tools → **vẫn dùng password cũ** → failures

### 1.2 Giải pháp

```
Password Rotated → Agent detects → Distribute → Verify → Report
```

---

## 2. Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | Target adapter plugin system | Interface: Distribute, Verify, Rollback |
| FR-2 | DAG-based execution | Parallel for independent, sequential for dependent |
| FR-3 | Auto-rollback | Rollback all targets if critical verification fails |
| FR-4 | Credential versioning | Keep last 5 versions for rollback |
| FR-5 | Dry-run mode | Simulate without actual changes |

---

## 3. Target Adapters

| Target | Adapter | Update Method |
|---|---|---|
| Jenkins | `jenkins_adapter` | Credentials API |
| GitLab CI | `gitlab_adapter` | CI Variables API |
| GitHub Actions | `github_adapter` | Secrets API |
| Kubernetes | `k8s_adapter` | Secret resource |
| Grafana | `grafana_adapter` | Datasource API |
| Webhook | `webhook_adapter` | Generic HTTP POST |

### 3.1 Adapter Interface

```go
type TargetAdapter interface {
    Type() string
    Validate(ctx context.Context, config TargetConfig) error
    Distribute(ctx context.Context, config TargetConfig, creds EncryptedCredentials) (*Result, error)
    Verify(ctx context.Context, config TargetConfig, creds EncryptedCredentials) error
    Rollback(ctx context.Context, config TargetConfig, prev EncryptedCredentials) error
}
```

---

## 4. Technical Design

### 4.1 Schema

```sql
CREATE TABLE distribution_target (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    priority INT DEFAULT 0,
    criticality TEXT DEFAULT 'normal'
);

CREATE TABLE distribution_mapping (
    id SERIAL PRIMARY KEY,
    target_id INT REFERENCES distribution_target(id),
    instance_id TEXT NOT NULL,
    target_secret_path TEXT NOT NULL,
    depends_on INT[],
    UNIQUE(target_id, instance_id)
);

CREATE TABLE distribution_event (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    trigger_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    total_targets INT,
    succeeded_targets INT DEFAULT 0,
    failed_targets INT DEFAULT 0
);

CREATE TABLE credential_version (
    id SERIAL PRIMARY KEY,
    instance_id TEXT NOT NULL,
    version INT NOT NULL,
    encrypted_credentials JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_current BOOLEAN DEFAULT FALSE,
    UNIQUE(instance_id, version)
);
```

### 4.2 Pipeline Flow

```
Credential Change → Build DAG → Version current creds
  → Level 0 (parallel): Target A ✅, Target B ✅
  → Level 1 (depends on L0): Target C ✅
  → Level 2: Target D ❌ → Rollback? (check criticality)
  → Notify result
```

### 4.3 Components

| Component | File/Package |
|---|---|
| Agent Core | `backend/component/agent/distribution/agent.go` |
| Pipeline Engine | `backend/component/agent/distribution/pipeline.go` |
| Rollback Manager | `backend/component/agent/distribution/rollback.go` |
| Adapters | `backend/component/agent/distribution/adapters/*.go` |
| Distribution Runner | `backend/runner/distribution/runner.go` |

---

## 5. Security

| Concern | Mitigation |
|---|---|
| Target credentials | Encrypted with BEE envelope (CR-SHR-102) |
| Rollback window | Previous creds valid only during window (1 hour) |
| Partial failure | Per-target status tracking; manual intervention |

---

## 6. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Rotate PG → distribute to Jenkins | Jenkins updated |
| TC-002 | Verification fails → rollback | All targets reverted |
| TC-003 | DAG with deps → correct order | Level 0 before Level 1 |
| TC-004 | Dry-run → no changes | Simulation only |
| TC-005 | Target unreachable → retry 3x | Then marked failed |

---

## 7. Rollout Plan

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Agent core + pipeline + target registry | Sprint 6-7 |
| 2 | K8s + Jenkins + GitLab adapters | Sprint 7-8 |
| 3 | Rollback manager + DAG execution | Sprint 8-9 |
| 4 | UI + E2E testing | Sprint 9-10 |
