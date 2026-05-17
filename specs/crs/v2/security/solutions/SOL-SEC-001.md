# Solution: CR-SEC-001 — Session Security Hardening

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-001                |
| **Solution**   | SOL-SEC-001               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Tăng cường bảo mật session bằng cách: (1) chuyển JWT signing từ HMAC-SHA256 sang RS256 asymmetric, (2) implement refresh token rotation với reuse detection trong `web_refresh_token` table, (3) thêm session fingerprint vào JWT claims và validate trong Auth Interceptor (L3), (4) xây dựng `SessionStore` (L8) để track concurrent sessions, (5) thêm idle timeout detection ở frontend với activity heartbeat, (6) triển khai in-memory token blacklist (L5) synced với PostgreSQL.

---

## 2. Architectural Alignment

```
L1 Presentation ──► L2 API Gateway ──► L3 Security Layer ──► L4 Service ──► L8 Store
     │                    │                    │                                │
     │ IdleDetector        │ Cookie Hardening   │ Auth Interceptor              │
     │ ActivityHeartbeat   │ Security Headers   │  ├─ Fingerprint Validate      │
     │ SessionManager      │                    │  ├─ Blacklist Check           │
     │                    │                    │  └─ Concurrent Session Check  │
     │                    │                    │                                │
     └────────────────────┴────────────────────┴── SessionStore (new)          │
                                                   TokenBlacklist (new) ───────┘
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L3 — Security** | `backend/api/auth/` | RS256 signing, fingerprint validation, blacklist check |
| **L4 — Service** | `auth_service.go` (78KB) | Refresh rotation, concurrent session enforcement |
| **L5 — Component** | `component/auth/blacklist.go` (new) | In-memory token blacklist + PG sync |
| **L8 — Store** | `store/session.go` (new) | Session registry, fingerprint storage |
| **L8 — Store** | `store/web_refresh_token.go` | Extend: rotation tracking, family detection |
| **L1 — Presentation** | `frontend/src/utils/session.ts` (new) | Idle detection, activity heartbeat |

---

## 3. Chi tiết Implementation

### 3.1 L3 — JWT Signing Migration (HMAC → RS256)

**File**: `backend/api/auth/`

Hiện tại sử dụng HMAC-SHA256 với shared secret từ DB (TDD Section 11.1). Chuyển sang RS256 cho asymmetric signing:

```go
// Key management
type TokenSigner struct {
    privateKey *rsa.PrivateKey  // Signing (server only)
    publicKey  *rsa.PublicKey   // Verification (can be shared)
    keyID      string           // Key version for rotation
}

// JWT Claims extension
type SessionClaims struct {
    jwt.RegisteredClaims
    UserUID     int    `json:"uid"`
    Fingerprint string `json:"fp"`   // NEW: session fingerprint hash
    KeyID       string `json:"kid"`  // NEW: key version
}
```

**Migration strategy**: Dual-validation period — accept both HMAC and RS256 tokens for 30 days, then deprecate HMAC.

### 3.2 L3 — Auth Interceptor Enhancement

**File**: `backend/api/auth/` (Auth Interceptor — step 2 in chain)

Extend interceptor chain (TDD Section 3.1) với fingerprint + blacklist checks:

```go
func (a *AuthInterceptor) authenticate(ctx context.Context, req connect.AnyRequest) error {
    token := extractToken(req) // Existing: Cookie or Bearer header

    claims, err := a.validateToken(token)  // Existing: signature check
    if err != nil { return err }

    // NEW: Token blacklist check
    if a.blacklist.IsBlacklisted(claims.ID) {
        return status.Errorf(codes.Unauthenticated, "token revoked")
    }

    // NEW: Fingerprint validation
    if a.config.FingerprintMode != "off" {
        fp := computeFingerprint(req.Header())
        if !validateFingerprint(claims.Fingerprint, fp, a.config.FingerprintMode) {
            a.securityEventChan <- SecurityEvent{Type: "fingerprint_mismatch", UserUID: claims.UserUID}
            return status.Errorf(codes.Unauthenticated, "session fingerprint mismatch")
        }
    }

    // Existing: Load user, set context
    return nil
}

func computeFingerprint(headers http.Header) string {
    data := headers.Get("User-Agent") + "|" +
            headers.Get("Accept-Language") + "|" +
            extractIPSubnet(headers) // /24 for IPv4, /48 for IPv6
    return sha256Hex(data)
}
```

### 3.3 L5 — Token Blacklist Component

**File**: `backend/component/auth/blacklist.go` (new)

```go
type TokenBlacklist struct {
    mu        sync.RWMutex
    memory    map[string]time.Time  // JTI → expiry
    store     *store.Store
    syncTick  *time.Ticker          // 30s sync to DB
}

func (b *TokenBlacklist) Revoke(jti string, expiry time.Time) {
    b.mu.Lock()
    b.memory[jti] = expiry
    b.mu.Unlock()
    // Async persist to store
    go b.store.InsertBlacklistedToken(context.Background(), jti, expiry)
}

func (b *TokenBlacklist) IsBlacklisted(jti string) bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    _, found := b.memory[jti]
    return found
}

// Cleanup goroutine removes expired JTIs
func (b *TokenBlacklist) cleanup() {
    now := time.Now()
    b.mu.Lock()
    for jti, exp := range b.memory {
        if now.After(exp) { delete(b.memory, jti) }
    }
    b.mu.Unlock()
}
```

**Initialization**: Added to server bootstrap sequence (TDD Section 2) after step 5 (IAM Manager).

### 3.4 L8 — Session Store

**File**: `backend/store/session.go` (new)

```sql
-- Migration: Create session tracking table
CREATE TABLE user_session (
    id          BIGSERIAL PRIMARY KEY,
    user_uid    INT NOT NULL REFERENCES principal(id),
    fingerprint TEXT NOT NULL,
    device_info JSONB,            -- User-Agent parsed info
    ip_address  TEXT NOT NULL,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked     BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_session_user ON user_session (user_uid, revoked);
CREATE INDEX idx_session_expiry ON user_session (expires_at) WHERE NOT revoked;

-- Token blacklist persistence
CREATE TABLE token_blacklist (
    jti        TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_blacklist_expiry ON token_blacklist (expires_at);
```

### 3.5 L4 — Concurrent Session Enforcement

**File**: `backend/api/v1/auth_service.go`

```go
func (s *AuthService) Login(ctx context.Context, req *v1pb.LoginRequest) (*v1pb.LoginResponse, error) {
    // ... existing auth logic ...

    // NEW: Concurrent session check
    activeSessions, _ := s.store.CountActiveSessions(ctx, user.UID)
    maxSessions := s.getSessionPolicy(ctx).MaxConcurrentSessions // default: 5

    if activeSessions >= maxSessions {
        switch policy.OnLimitExceeded {
        case "terminate_oldest":
            s.store.TerminateOldestSession(ctx, user.UID)
        case "deny_new":
            return nil, status.Errorf(codes.ResourceExhausted, "max concurrent sessions reached")
        }
    }

    // Create session record
    session := &store.SessionMessage{
        UserUID:     user.UID,
        Fingerprint: computeFingerprint(req.Header()),
        IPAddress:   extractClientIP(ctx),
        ExpiresAt:   time.Now().Add(refreshTokenTTL),
    }
    s.store.CreateSession(ctx, session)

    // ... generate tokens with fingerprint claim ...
}
```

### 3.6 L2 — Cookie Hardening

**File**: `backend/server/echo_routes.go`

Extend `securityHeadersMiddleware` (Architecture L2, Middleware Stack item 2):

```go
func setAuthCookie(c echo.Context, token string, maxAge int) {
    cookie := &http.Cookie{
        Name:     "__Host-bb-session",  // Host-prefix for production
        Value:    token,
        Path:     "/",
        MaxAge:   maxAge,
        HttpOnly: true,
        Secure:   true,                 // Always true in production
        SameSite: http.SameSiteStrictMode,
    }
    c.SetCookie(cookie)
}
```

### 3.7 L1 — Frontend Idle Detection

**File**: `frontend/src/utils/session.ts` (new)

```typescript
export class SessionManager {
    private idleTimer: number;
    private warningTimer: number;
    private readonly idleTimeout: number; // from workspace settings
    private readonly warningBefore = 5 * 60 * 1000; // 5 min

    start() {
        const events = ['mousedown', 'keydown', 'scroll', 'touchstart'];
        events.forEach(e => document.addEventListener(e, () => this.resetTimer()));
        this.resetTimer();
    }

    private resetTimer() {
        clearTimeout(this.idleTimer);
        clearTimeout(this.warningTimer);
        this.warningTimer = setTimeout(() => this.showWarning(), this.idleTimeout - this.warningBefore);
        this.idleTimer = setTimeout(() => this.logout(), this.idleTimeout);
    }
}
```

---

## 4. Database Changes

```sql
CREATE TABLE user_session ( /* see 3.4 */ );
CREATE TABLE token_blacklist ( /* see 3.4 */ );
-- Extend web_refresh_token with family tracking
ALTER TABLE web_refresh_token ADD COLUMN family_id TEXT;
ALTER TABLE web_refresh_token ADD COLUMN rotation_count INT DEFAULT 0;
```

---

## 5. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| RS256 key compromise | Keys stored via External Secret Manager (CR-ENT-015) |
| Blacklist memory growth | Auto-cleanup of expired JTIs, max memory cap |
| Fingerprint false positives | Configurable strictness modes (strict/relaxed/off) |
| Cookie SameSite + SSO | Use `Lax` for OAuth callback cookie, `Strict` for session |

---

## 6. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-009 (2FA) | 2FA verification integrated into session creation |
| CR-ENT-015 (Secret Manager) | RS256 signing keys stored in secret manager |
| CR-SEC-003 (Brute-Force) | Failed login tracking feeds into session security |

---

## 7. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Cookie hardening + RS256 migration | Sprint 1 |
| 2 | Session store + concurrent session control | Sprint 1 |
| 3 | Fingerprint validation in Auth Interceptor | Sprint 2 |
| 4 | Token blacklist component | Sprint 2 |
| 5 | Refresh token rotation + reuse detection | Sprint 3 |
| 6 | Frontend idle detection + session management UI | Sprint 3 |
