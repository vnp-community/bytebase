# Solution: CR-SEC-003 — Brute-Force & Account Lockout Protection

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-003                |
| **Solution**   | SOL-SEC-003               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Triển khai rate limiter component (L5) sử dụng sliding window algorithm backed bởi in-memory counters + PostgreSQL persistence. Integrate vào Auth Interceptor (L3) cho login endpoint, thêm account lockout logic vào AuthService (L4), CAPTCHA validation middleware (L2), và GeoIP-based suspicious login detection runner (L6).

---

## 2. Architectural Alignment

```
L2 API Gateway ──► CAPTCHA Middleware (new) ──► L3 Auth Interceptor
                                                     │
                                          Rate Limiter Check (L5)
                                                     │
L4 AuthService ──► Login logic + lockout enforcement
                       │
L5 Component ──► component/ratelimit/ (new) ── Sliding window counters
                 component/geoip/ (new) ─────── MaxMind GeoLite2
                       │
L6 Runner ──► runner/security/ (new) ────────── Suspicious login alerting
                       │
L8 Store ──► store/login_attempt.go (new) ──── Attempt history + lockout state
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L2** | Echo middleware | CAPTCHA validation (reCAPTCHA/hCaptcha/Turnstile) |
| **L3** | Auth Interceptor | Per-IP rate limit check before auth |
| **L4** | `auth_service.go` | Lockout logic, progressive delay |
| **L5** | `component/ratelimit/` (new) | Sliding window rate limiter |
| **L5** | `component/geoip/` (new) | MaxMind GeoLite2 IP→location |
| **L6** | `runner/security/` (new) | Suspicious login background alerting |
| **L8** | `store/login_attempt.go` (new) | Login attempt persistence |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Rate Limiter Component

**File**: `backend/component/ratelimit/limiter.go` (new)

```go
type SlidingWindowLimiter struct {
    mu       sync.RWMutex
    windows  map[string]*Window  // key (ip/user) → window
    config   RateLimitConfig
}

type Window struct {
    counts    []int64      // per-second buckets
    startTime time.Time
    total     int64
}

func (l *SlidingWindowLimiter) Allow(key string) (bool, *RateLimitInfo) {
    l.mu.Lock()
    defer l.mu.Unlock()

    w := l.getOrCreateWindow(key)
    w.slide(time.Now())

    if w.total >= l.config.MaxRequests {
        return false, &RateLimitInfo{
            Remaining:  0,
            RetryAfter: w.nextAvailableSlot(),
        }
    }
    w.increment()
    return true, &RateLimitInfo{Remaining: l.config.MaxRequests - w.total}
}
```

### 3.2 L4 — AuthService Lockout

**File**: `backend/api/v1/auth_service.go` (extend existing 78KB)

```go
func (s *AuthService) Login(ctx context.Context, req *v1pb.LoginRequest) (*v1pb.LoginResponse, error) {
    // Check account lockout FIRST
    lockout, _ := s.store.GetLockoutStatus(ctx, req.Email)
    if lockout != nil && lockout.IsLocked() {
        return nil, status.Errorf(codes.ResourceExhausted,
            "account locked until %s", lockout.UnlockedAt.Format(time.RFC3339))
    }

    // Existing auth logic...
    user, err := s.verifyCredentials(ctx, req.Email, req.Password)
    if err != nil {
        // Record failed attempt
        attempts := s.store.IncrementFailedAttempts(ctx, req.Email, extractClientIP(ctx))

        // Progressive lockout
        switch {
        case attempts >= 50:
            s.store.LockAccount(ctx, req.Email, 24*time.Hour)
            s.notifyAdmin(ctx, "account_lockout_critical", req.Email)
        case attempts >= 20:
            s.store.LockAccount(ctx, req.Email, 1*time.Hour)
        case attempts >= 10:
            s.store.LockAccount(ctx, req.Email, 15*time.Minute)
        case attempts >= 5:
            // Return CAPTCHA requirement flag
            return nil, status.Errorf(codes.FailedPrecondition, "captcha_required")
        case attempts >= 3:
            time.Sleep(2 * time.Second) // Progressive delay
        }
        return nil, status.Errorf(codes.Unauthenticated, "invalid credentials")
    }

    // Reset on success
    s.store.ResetFailedAttempts(ctx, req.Email)
    // Record successful login for suspicious detection
    s.store.RecordLogin(ctx, user.UID, extractClientIP(ctx), req.Header().Get("User-Agent"))

    // ... continue with session creation (SOL-SEC-001) ...
}
```

### 3.3 L8 — Login Attempt Store

```sql
CREATE TABLE login_attempt (
    id          BIGSERIAL PRIMARY KEY,
    email       TEXT NOT NULL,
    ip_address  TEXT NOT NULL,
    user_agent  TEXT,
    success     BOOLEAN NOT NULL,
    geo_country TEXT,             -- ISO country code from GeoIP
    geo_city    TEXT,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE account_lockout (
    email       TEXT PRIMARY KEY,
    failed_count INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_attempt TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_attempt_email ON login_attempt (email, created_ts DESC);
CREATE INDEX idx_login_attempt_ip ON login_attempt (ip_address, created_ts DESC);
```

### 3.4 L5 — GeoIP Component

**File**: `backend/component/geoip/geoip.go` (new)

```go
type GeoIPService struct {
    reader *maxminddb.Reader  // MaxMind GeoLite2-City
}

func (g *GeoIPService) Lookup(ip string) (*GeoResult, error) {
    var record struct {
        Country struct { ISOCode string `maxminddb:"iso_code"` } `maxminddb:"country"`
        City    struct { Names map[string]string `maxminddb:"names"` } `maxminddb:"city"`
    }
    err := g.reader.Lookup(net.ParseIP(ip), &record)
    return &GeoResult{Country: record.Country.ISOCode, City: record.City.Names["en"]}, err
}
```

### 3.5 L6 — Suspicious Login Runner

**File**: `backend/runner/security/suspicious_login.go` (new)

```go
func (r *SecurityRunner) detectSuspiciousLogins(ctx context.Context) {
    // Check recent logins for anomalies
    recentLogins, _ := r.store.GetRecentLogins(ctx, 1*time.Hour)

    for _, login := range recentLogins {
        // Impossible travel: same user, different country, < 2h apart
        previousLogin, _ := r.store.GetPreviousLogin(ctx, login.UserUID)
        if previousLogin != nil && login.GeoCountry != previousLogin.GeoCountry {
            timeDiff := login.CreatedTS.Sub(previousLogin.CreatedTS)
            if timeDiff < 2*time.Hour {
                r.alertSuspiciousLogin(ctx, login, "impossible_travel")
            }
        }

        // New device detection
        if r.isNewDevice(ctx, login) {
            r.sendNewDeviceEmail(ctx, login)
        }
    }
}
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| User enumeration via lockout | Same error message for invalid user and locked account |
| CAPTCHA bypass | Server-side validation, multiple provider support |
| Rate limiter memory | Bounded window maps with periodic cleanup |
| GeoIP accuracy | Fallback to "unknown" when lookup fails |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-001 (Session) | Lockout feeds into session management |
| CR-SEC-010 (SIEM) | Brute-force events forwarded to SIEM |
| CR-SEC-006 (IP Allow) | IP blocking reuses GeoIP component |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Rate limiter component + login integration | Sprint 1 |
| 2 | Account lockout + progressive delay | Sprint 1 |
| 3 | CAPTCHA middleware | Sprint 2 |
| 4 | GeoIP + suspicious login detection | Sprint 3 |
| 5 | Login history UI + admin controls | Sprint 3 |
