# Change Request: IP Allowlisting & Geo-Restriction

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-006                                               |
| **Feature ID**     | SEC-01                                                   |
| **Title**          | IP Allowlisting & Geo-Restriction                       |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai kiểm soát truy cập dựa trên IP address và vị trí địa lý (GeoIP) cho workspace, API, và database connections. Bao gồm IP allowlist/denylist, country-level restrictions, VPN detection, và Tor exit node blocking.

---

## 2. Yêu cầu chức năng

### FR-001: Workspace IP Allowlist
- **Configuration**:
  ```yaml
  ip_policy:
    mode: "allowlist"          # allowlist | denylist | monitor
    rules:
      - cidr: "10.0.0.0/8"
        label: "Corporate VPN"
      - cidr: "203.0.113.0/24"
        label: "Office Network"
    enforcement:
      web_ui: true
      api: true
      service_account_exempt: false
    on_violation: "block"      # block | mfa_challenge | log_only
  ```
- **Acceptance Criteria**:
  - AC-1: CIDR range support (IPv4 + IPv6)
  - AC-2: Per-workspace IP policy configuration
  - AC-3: Enforcement modes: block, MFA challenge, log-only
  - AC-4: Service account exemption option
  - AC-5: X-Forwarded-For header parsing (reverse proxy support)

### FR-002: Geo-Restriction
- Country-level access control using GeoIP database
- **Acceptance Criteria**:
  - AC-1: Allow/deny by country code (ISO 3166-1)
  - AC-2: GeoIP database auto-update (MaxMind GeoLite2)
  - AC-3: Fallback behavior when GeoIP lookup fails (configurable)
  - AC-4: Geo-restriction combinable with IP allowlist

### FR-003: Tor & Anonymous Proxy Detection
- **Acceptance Criteria**:
  - AC-1: Tor exit node detection and blocking (configurable)
  - AC-2: Known anonymous proxy/VPN service detection
  - AC-3: Configurable policy: block, MFA challenge, or log

### FR-004: Database Connection IP Restriction
- Restrict which IPs can connect through Bytebase to database instances
- **Acceptance Criteria**:
  - AC-1: Per-instance IP allowlist for database connections
  - AC-2: SQL Editor access restricted by IP
  - AC-3: Admin mode access requires stricter IP validation

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| IP Policy Middleware (new)   | `backend/api/middleware/ip_policy.go`        | IP validation, geo-restriction              |
| GeoIP Service (new)          | `backend/component/geoip/`                  | MaxMind integration                         |
| Setting Service              | `backend/api/v1/setting_service.go`         | IP policy configuration                     |
| DB Connection Filter         | `backend/plugin/db/`                        | Per-instance IP restriction                 |
| IP Policy UI (new)           | `frontend/src/views/IPPolicySettings.vue`   | IP/Geo configuration interface              |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Access from allowed IP                               | Access granted                   |
| TC-002  | Access from non-allowed IP                           | Access blocked / MFA challenge   |
| TC-003  | Access from restricted country                       | Access denied                    |
| TC-004  | Access via Tor exit node                             | Blocked per policy               |
| TC-005  | Service account exempt from IP policy                | Access granted regardless        |
| TC-006  | SQL Editor from non-allowed IP                       | Connection blocked               |
| TC-007  | X-Forwarded-For with trusted proxy                   | Real IP correctly parsed         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | IP allowlist/denylist                | Sprint 1       |
| Phase 2 | GeoIP integration                    | Sprint 2       |
| Phase 3 | Tor/proxy detection                  | Sprint 3       |
| Phase 4 | Database connection restriction      | Sprint 3       |
