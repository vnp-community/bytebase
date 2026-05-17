# Change Request: Secure Share Link Generator with TTL & Access Control

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SHR-103                                               |
| **Title**          | Secure Share Link Generator with TTL & Access Control    |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Extends**        | CR-SHR-002, CR-SHR-003                                   |

---

## 1. Mô tả

Bổ sung CR-SHR-002 (Vaultwarden Send) với:
- **Public access endpoint** cho external users (vendors, contractors)
- **Email OTP verification** xác thực người nhận
- **Share policies** — Workspace/project-level TTL, password, IP allowlist
- **Bytebase-native fallback** khi Vaultwarden unavailable

---

## 2. Requirements

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-1 | Public endpoint `/share/{token}` | No Bytebase auth needed |
| FR-2 | Email OTP verification | 6-digit, 5min TTL, single-use |
| FR-3 | Share policies (workspace/project) | Max TTL, require password, IP allowlist |
| FR-4 | Native storage fallback | BEE envelope in `share_link` table |
| FR-5 | Management dashboard | List, revoke, extend, bulk operations |

---

## 3. Technical Design

### 3.1 Schema

```sql
CREATE TABLE share_link (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    creator_id INT REFERENCES principal(id),
    content_type TEXT NOT NULL,
    encrypted_envelope JSONB NOT NULL,
    vaultwarden_send_id TEXT,
    password_hash TEXT,
    allowed_emails TEXT[],
    max_accesses INT,
    current_accesses INT DEFAULT 0,
    ttl_seconds INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE share_policy (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    project_id TEXT,
    max_ttl_seconds INT DEFAULT 604800,
    require_password BOOLEAN DEFAULT FALSE,
    require_email_restriction BOOLEAN DEFAULT FALSE,
    max_active_per_user INT DEFAULT 10,
    ip_allowlist INET[]
);
```

### 3.2 Components

| Component | File/Package |
|---|---|
| Share Access Handler | `backend/api/v1/share_access_handler.go` |
| Share Policy Engine | `backend/component/sharing/policy.go` |
| OTP Service | `backend/component/sharing/otp.go` |
| Share Cleaner | `backend/runner/cleaner/share_cleaner.go` |

---

## 4. Security

| Concern | Mitigation |
|---|---|
| Token brute-force | 256-bit token; rate limiting |
| Password brute-force | Argon2id + 5 attempts max |
| Content caching | `Cache-Control: no-store` |
| XSS | Content escaped, never raw HTML |

---

## 5. Rollout

| Phase | Tasks | Sprint |
|---|---|---|
| 1 | Public endpoint + native storage | Sprint 5-6 |
| 2 | Email OTP + password protection | Sprint 6-7 |
| 3 | Share policies + dashboard | Sprint 7-8 |
