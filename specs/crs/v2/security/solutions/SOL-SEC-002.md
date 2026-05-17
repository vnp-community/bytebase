# Solution: CR-SEC-002 — API Key Lifecycle Management

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-002                |
| **Solution**   | SOL-SEC-002               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng Service Account system hiện có (L4: `service_account_service.go`) thành full API Key lifecycle: scoped permissions, IP restrictions, auto-rotation via new Runner (L6), usage auditing qua Audit Interceptor (L3), và leak detection integration. Key storage sử dụng SHA-256 hash trong Store (L8), key prefix `bb_` cho scanner detection.

---

## 2. Architectural Alignment

```
L3 Security ──► Auth Interceptor (API Key validate + scope check)
                     │
L4 Service ──► APIKeyService (new) ──► CRUD + rotation
                     │
L5 Component ──► component/apikey/ (new) ──► Scope engine, usage tracker
                     │
L6 Runner ──► runner/keyrotation/ (new) ──► Scheduled rotation
                     │
L8 Store ──► store/api_key.go (new) ──► Key hash storage + usage log
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L3** | `backend/api/auth/` | Extend auth: API key extraction → scope validation |
| **L4** | `api_key_service.go` (new) | CRUD, rotation trigger, usage API |
| **L5** | `component/apikey/` (new) | Scope engine, usage aggregation |
| **L6** | `runner/keyrotation/` (new) | Scheduled rotation + notification |
| **L8** | `store/api_key.go` (new) | Key persistence (hashed) + usage log |

---

## 3. Chi tiết Implementation

### 3.1 L8 — Database Schema

```sql
CREATE TABLE api_key (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    prefix        TEXT NOT NULL,          -- "bb_live_" or "bb_test_"
    key_hash      TEXT NOT NULL,          -- SHA-256 of full key
    key_hint      TEXT NOT NULL,          -- Last 4 chars for identification
    principal_uid INT NOT NULL REFERENCES principal(id),
    scopes        JSONB NOT NULL,         -- ["plans:create", "rollouts:read"]
    allowed_ips   TEXT[],                 -- CIDR ranges
    env_restrict  TEXT[],                 -- Environment names
    expires_at    TIMESTAMPTZ NOT NULL,
    rotation_id   TEXT,                   -- Group keys for rotation
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_ts    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used     TIMESTAMPTZ,
    UNIQUE(key_hash)
);

CREATE TABLE api_key_usage (
    id          BIGSERIAL PRIMARY KEY,
    key_id      BIGINT NOT NULL REFERENCES api_key(id),
    endpoint    TEXT NOT NULL,
    ip_address  TEXT NOT NULL,
    status_code INT NOT NULL,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_key_principal ON api_key (principal_uid, is_active);
CREATE INDEX idx_api_key_hash ON api_key (key_hash) WHERE is_active;
CREATE INDEX idx_api_key_usage_ts ON api_key_usage (key_id, created_ts DESC);
```

### 3.2 L3 — Auth Interceptor Extension

**File**: `backend/api/auth/`

Extend existing API key flow (TDD Section 11.1: `Authorization: Bearer`):

```go
func (a *AuthInterceptor) authenticateAPIKey(ctx context.Context, bearerToken string) (*UserContext, error) {
    // Parse key format: bb_live_<random_32bytes>
    if !strings.HasPrefix(bearerToken, "bb_") {
        return a.authenticateLegacyKey(ctx, bearerToken) // backward compat
    }

    keyHash := sha256Hex(bearerToken)
    apiKey, err := a.store.GetAPIKeyByHash(ctx, keyHash)
    if err != nil { return nil, codes.Unauthenticated }

    // Validate expiry
    if time.Now().After(apiKey.ExpiresAt) {
        return nil, status.Errorf(codes.Unauthenticated, "API key expired")
    }

    // Validate IP restriction
    clientIP := extractClientIP(ctx)
    if !matchCIDR(clientIP, apiKey.AllowedIPs) {
        return nil, status.Errorf(codes.PermissionDenied, "IP not allowed for this key")
    }

    // Track usage (async)
    go a.apiKeyUsageTracker.Record(apiKey.ID, extractMethod(ctx), clientIP)

    // Return user context with scope restriction
    return &UserContext{
        User:   apiKey.Principal,
        Scopes: apiKey.Scopes,
    }, nil
}
```

### 3.3 L3 — ACL Interceptor Scope Check

**File**: `backend/api/v1/acl.go` (19.3KB)

Extend ACL check to respect API key scopes:

```go
func (a *ACLInterceptor) checkPermission(ctx context.Context, method string) error {
    userCtx := getUserContext(ctx)

    // Standard IAM check (existing)
    if err := a.iamManager.CheckPermission(ctx, ...); err != nil {
        return err
    }

    // NEW: API key scope restriction
    if userCtx.Scopes != nil {
        requiredScope := methodToScope(method) // "plans:create", "rollouts:read"
        if !containsScope(userCtx.Scopes, requiredScope) {
            return status.Errorf(codes.PermissionDenied, "API key lacks scope: %s", requiredScope)
        }
    }
    return nil
}
```

### 3.4 L6 — Key Rotation Runner

**File**: `backend/runner/keyrotation/runner.go` (new)

Add to server bootstrap (TDD Section 2, step 9):

```go
type KeyRotationRunner struct {
    store          *store.Store
    webhookManager *webhook.Manager
    interval       time.Duration // Check every hour
}

func (r *KeyRotationRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    for {
        select {
        case <-ticker.C:
            r.checkExpiringKeys(ctx)   // Notify 14 days before expiry
            r.executeScheduledRotations(ctx) // Auto-rotate if configured
        case <-ctx.Done():
            return
        }
    }
}
```

### 3.5 L4 — API Key Service

**Proto**: `api_key_service.proto` (new)

```protobuf
service APIKeyService {
    rpc CreateAPIKey(CreateAPIKeyRequest) returns (CreateAPIKeyResponse);    // Returns full key ONCE
    rpc ListAPIKeys(ListAPIKeysRequest) returns (ListAPIKeysResponse);      // Returns metadata only
    rpc RevokeAPIKey(RevokeAPIKeyRequest) returns (google.protobuf.Empty);
    rpc RotateAPIKey(RotateAPIKeyRequest) returns (RotateAPIKeyResponse);
    rpc GetAPIKeyUsage(GetAPIKeyUsageRequest) returns (GetAPIKeyUsageResponse);
}
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Key stored plaintext | Only SHA-256 hash stored, full key shown once at creation |
| Key enumeration | Rate limit on ListAPIKeys, no key value in responses |
| Leaked key detection | Key prefix `bb_live_` / `bb_test_` designed for GitHub scanner |
| Over-scoped key | Mandatory scope restriction, no wildcard `*` |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-015 (Secret Manager) | API key encryption at rest |
| CR-SEC-001 (Session) | API keys bypass session limits (exempt) |
| CR-SEC-010 (SIEM) | Key usage anomalies forwarded to SIEM |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Database schema + Store layer | Sprint 1 |
| 2 | Auth interceptor key validation + scope check | Sprint 1 |
| 3 | APIKeyService CRUD + UI | Sprint 2 |
| 4 | Usage tracking + dashboard | Sprint 2 |
| 5 | Rotation runner + notification | Sprint 3 |
| 6 | Leak detection integration | Sprint 4 |
