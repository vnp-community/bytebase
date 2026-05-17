# Change Request: API Key Lifecycle Management

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-002                                               |
| **Feature ID**     | SEC-01, SEC-19                                           |
| **Title**          | API Key Lifecycle Management                             |
| **Plan**           | TEAM / ENTERPRISE                                        |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Quản lý vòng đời API Key toàn diện cho Service Accounts và machine-to-machine authentication. Bao gồm key creation, rotation, expiration, scoping, usage auditing, và revocation.

### 1.2 Bối cảnh
Bytebase hỗ trợ Service Account (UR-P05) và Workload Identity (SEC-19), nhưng thiếu enterprise-grade API key management: auto-rotation, scope restriction, usage analytics, leak detection.

---

## 2. Yêu cầu chức năng

### FR-001: API Key Creation & Scoping
- Tạo API key với scope restrictions:
  ```yaml
  api_key:
    name: "ci-cd-deploy-key"
    scopes:
      - "plans:create"
      - "rollouts:create"
      - "rollouts:read"
    allowed_ips: ["10.0.0.0/8"]
    environment_restriction: ["production"]
    expiry: "90d"
  ```
- **Acceptance Criteria**:
  - AC-1: API key với fine-grained permission scopes
  - AC-2: IP allowlist per key
  - AC-3: Environment restriction per key
  - AC-4: Mandatory expiration date (max 365 days)
  - AC-5: Key prefix for identification (`bb_live_`, `bb_test_`)

### FR-002: Automatic Key Rotation
- Scheduled rotation với grace period (2 keys active during transition)
- Notification 14 days trước expiry
- **Acceptance Criteria**:
  - AC-1: Admin configurable rotation schedule (30/60/90 days)
  - AC-2: Grace period: old key valid 24h after rotation
  - AC-3: Email/webhook notification trước expiry
  - AC-4: Bulk rotation for all keys trong workspace

### FR-003: Key Usage Auditing
- Track mỗi API call per key: timestamp, endpoint, IP, response code
- Usage dashboard với anomaly indicators
- **Acceptance Criteria**:
  - AC-1: Usage log retained 90 days
  - AC-2: Real-time usage dashboard per key
  - AC-3: Alert on unusual patterns (spike, new IP, failed auth)
  - AC-4: Export usage report (CSV/JSON)

### FR-004: Key Leak Detection
- Detect API keys trong public repositories
- Integration với GitHub secret scanning
- **Acceptance Criteria**:
  - AC-1: Auto-revoke keys detected in public repos
  - AC-2: Webhook notification on detected leak
  - AC-3: Key format designed for scanner detection (prefix + checksum)

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| API Key Service (new)        | `backend/api/v1/api_key_service.go`         | CRUD + rotation + scoping                   |
| API Key Store (new)          | `backend/store/api_key.go`                  | Key storage (hashed, never plaintext)        |
| Auth Interceptor             | `backend/api/interceptor/auth.go`           | API key validation + scope check            |
| Key Rotation Runner (new)    | `backend/runner/keyrotation/`               | Scheduled rotation + notification           |
| API Key Dashboard            | `frontend/src/views/APIKeyManagement.vue`   | Key management UI + usage analytics         |

---

## 4. Security Considerations

| Concern                     | Mitigation                                                    |
|-----------------------------|---------------------------------------------------------------|
| Key storage                  | Store only SHA-256 hash, never plaintext                      |
| Key in transit               | Only show full key once at creation                           |
| Leaked keys                  | Auto-revocation + prefix-based detection                      |
| Over-permissioned keys       | Mandatory scope restriction, no wildcard                      |
| Key enumeration              | Rate limit key listing API                                    |

---

## 5. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Create scoped API key                                | Key created with restrictions    |
| TC-002  | API call outside allowed scope                       | 403 Forbidden                    |
| TC-003  | API call from non-allowed IP                         | 403 Forbidden                    |
| TC-004  | Key expired                                          | 401 Unauthorized                 |
| TC-005  | Auto-rotation triggered                              | New key issued, old key grace    |
| TC-006  | Leaked key detected                                  | Key auto-revoked                 |
| TC-007  | Key usage spike                                      | Alert generated                  |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Key CRUD + scoping                   | Sprint 1       |
| Phase 2 | Usage auditing + dashboard           | Sprint 2       |
| Phase 3 | Auto-rotation + notifications        | Sprint 3       |
| Phase 4 | Leak detection integration           | Sprint 4       |
