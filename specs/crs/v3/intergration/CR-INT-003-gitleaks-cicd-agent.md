# Change Request: GitLeaks CI/CD Integration Agent

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INT-003                                               |
| **Gap ID**         | G7                                                       |
| **Title**          | GitLeaks CI/CD Integration Agent                         |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **External Tool**  | GitLeaks / TruffleHog                                    |

---

## 1. Tổng quan

### 1.1 Mô tả
Agent tích hợp GitLeaks/TruffleHog vào CI/CD pipeline và đồng bộ kết quả scan vào Bytebase. Agent chủ động:
- Trigger GitLeaks scan khi phát hiện repository changes liên quan DB credentials
- Import scan results vào Bytebase compliance dashboard
- Auto-create Bytebase credential rotation Issues khi phát hiện leaked secrets
- Maintain custom `.gitleaks.toml` rules synchronized with Bytebase DB user registry

### 1.2 Bối cảnh
Gap G7 đề xuất GitLeaks cho hardcode password scanning trong source code. CR-INS-007 covers SQL pipeline scanning bên trong Bytebase. Agent này mở rộng ra **source code repositories** — nơi developers có thể hardcode DB credentials.

### 1.3 Mục tiêu
- Automated secret scanning cho all application repos
- Bytebase-aware custom rules (scan cho known DB users/passwords)
- Scan results → Bytebase compliance dashboard
- Auto-remediation: credential rotation Issues

---

## 2. Yêu cầu chức năng

### FR-001: GitLeaks Rule Synchronization
- Agent reads Bytebase DB user registry → generates custom GitLeaks rules
- Rules target:
  - Known DB usernames trong connection strings
  - Known database hostnames/ports
  - DB-engine specific patterns (Oracle TNS, PG connection URI, MongoDB URI)
- Auto-update `.gitleaks.toml` khi Bytebase user registry changes
- Push updated rules to GitLab/GitHub shared config repo

### FR-002: Scan Trigger & Execution
- **Webhook trigger**: GitLab/GitHub webhook → Agent → run GitLeaks scan
- **Scheduled trigger**: Daily full-repo scan
- **On-demand**: DBA trigger scan từ Bytebase UI
- Scan modes:
  - `detect`: Scan current state
  - `protect`: Scan only new commits (pre-commit hook)
- Support multiple repos: configurable repo registry

### FR-003: Results Import & Correlation
- Parse GitLeaks SARIF/JSON output → import vào Bytebase
- Correlate findings với Bytebase DB user registry:
  - "Found credential for user `QR_CONNECTOR_APP` in file `config/db.yaml`"
- Deduplication: same finding across scans → single entry
- Status tracking: NEW → INVESTIGATING → REMEDIATED → FALSE_POSITIVE

### FR-004: Auto-Remediation Workflow
- When credential detected in repo:
  1. Agent creates Bytebase Issue: "Credential rotation required for user X"
  2. Issue pre-populated with:
     - ALTER USER SQL for password change
     - Affected repository and file path
     - Recommended remediation: move to Secret Manager
  3. Notify DBA + application team
  4. Track remediation status

### FR-005: CI/CD Pipeline Integration
- Generate GitLab CI / GitHub Actions workflow snippets
- Pipeline stage: scan → if findings → block merge + notify Bytebase
- SARIF upload to GitLab Security Dashboard

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| GitLeaks Agent Core | `backend/component/agent/gitleaks/agent.go` | Agent lifecycle |
| Rule Generator | `backend/component/agent/gitleaks/rule_gen.go` | Auto-generate rules |
| Scan Executor | `backend/component/agent/gitleaks/executor.go` | Run GitLeaks binary |
| Results Importer | `backend/component/agent/gitleaks/importer.go` | SARIF parsing |
| Remediation Creator | `backend/component/agent/gitleaks/remediation.go` | Auto-create Issues |
| Webhook Handler | `backend/component/agent/gitleaks/webhook.go` | Git webhook receiver |
| Agent Config | `backend/component/agent/gitleaks/config.go` | YAML config |
| Scan Results UI | `frontend/src/views/Integration/GitLeaks/` | Findings dashboard |

### 3.1 Agent Configuration

```yaml
agent:
  name: gitleaks-integration
  enabled: true

gitleaks:
  binary_path: "/usr/local/bin/gitleaks"
  version: "8.x"
  
repositories:
  - name: "qr-connector-app"
    url: "git@gitlab.vnpay.vn:backend/qr-connector.git"
    branch: "main"
    scan_schedule: "0 2 * * *"  # daily 2am
  - name: "payment-service"
    url: "git@gitlab.vnpay.vn:backend/payment-service.git"
    branch: "main"

webhook:
  listen_port: 8090
  secret: "${WEBHOOK_SECRET}"

remediation:
  auto_create_issue: true
  issue_assignee: "dba-team"
  notify_channels: ["slack:#security-alerts"]
```

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Repo có `password=secret123` → scan detects | Finding imported to Bytebase |
| TC-002 | Known DB user in connection string → correlated | Shows "user QR_APP found" |
| TC-003 | New DB user added to Bytebase → rules updated | `.gitleaks.toml` regenerated |
| TC-004 | Finding detected → auto-create Issue | Issue with rotation SQL |
| TC-005 | Webhook from GitLab push → scan triggered | Scan runs, results imported |
| TC-006 | Finding marked FALSE_POSITIVE → not re-alerted | Suppressed in future scans |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Agent core + manual scan execution | Sprint 1 |
| Phase 2 | Rule sync + scheduled scanning | Sprint 2 |
| Phase 3 | Results import + auto-remediation | Sprint 3 |
| Phase 4 | CI/CD pipeline integration | Sprint 4 |
