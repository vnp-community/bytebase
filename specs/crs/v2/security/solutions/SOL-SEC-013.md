# Solution: CR-SEC-013 — Rate Limiting & DDoS Protection

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-013                |
| **Solution**   | SOL-SEC-013               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Triển khai multi-tier rate limiting middleware (L2) trong Echo middleware stack. Token bucket algorithm backed bởi in-memory storage (single-node) hoặc Redis (HA mode — tương tự cache strategy, TDD Section 4.2). Per-IP, per-user, per-endpoint configs. Adaptive throttling dựa trên system load metrics (Prometheus, L10). DDoS protection via connection-level limits.

---

## 2. Architectural Alignment

```
L2 Echo Middleware Stack (Architecture Section 3):
  1. recoverMiddleware (existing)
  2. ipPolicyMiddleware (SOL-SEC-006)
  3. ★ rateLimitMiddleware (NEW) ← Insert here
  4. securityHeadersMiddleware (existing)
  5. CORS (dev mode)
  6. Request Logger
  7. Prometheus metrics

L2 ──► L5 component/ratelimit/ ── Rate limit engine
                │
                ├── InMemoryStore (single-node, like existing LRU cache)
                └── RedisStore (HA mode, when cache disabled)
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L2** | `echo_routes.go` | Rate limit middleware registration |
| **L5** | `component/ratelimit/` (new) | Multi-tier token bucket engine |
| **L8** | `store/setting.go` | Rate limit configuration |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Rate Limit Engine

**File**: `backend/component/ratelimit/engine.go` (new)

```go
type RateLimitEngine struct {
    globalLimiter    *TokenBucket
    ipLimiters       *sync.Map     // IP → *TokenBucket
    userLimiters     *sync.Map     // UserUID → *TokenBucket
    endpointConfigs  map[string]*EndpointLimit
    store            RateLimitStore // In-memory or Redis
}

type TokenBucket struct {
    mu       sync.Mutex
    tokens   float64
    capacity float64
    rate     float64   // tokens per second
    lastTime time.Time
}

func (b *TokenBucket) Allow() (bool, *RateLimitInfo) {
    b.mu.Lock()
    defer b.mu.Unlock()

    now := time.Now()
    elapsed := now.Sub(b.lastTime).Seconds()
    b.tokens = min(b.capacity, b.tokens + elapsed * b.rate)
    b.lastTime = now

    if b.tokens < 1 {
        retryAfter := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
        return false, &RateLimitInfo{RetryAfter: retryAfter}
    }
    b.tokens--
    return true, &RateLimitInfo{Remaining: int(b.tokens), Limit: int(b.capacity)}
}
```

### 3.2 L2 — Rate Limit Middleware

**File**: `backend/server/echo_routes.go`

```go
func rateLimitMiddleware(engine *ratelimit.RateLimitEngine) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            clientIP := resolveClientIP(c)
            method := c.Path()

            // Tier 1: Global rate limit
            if allowed, info := engine.CheckGlobal(); !allowed {
                return rateLimitResponse(c, info)
            }

            // Tier 2: Per-IP rate limit
            if allowed, info := engine.CheckIP(clientIP); !allowed {
                return rateLimitResponse(c, info)
            }

            // Tier 3: Per-endpoint rate limit (for sensitive endpoints)
            if allowed, info := engine.CheckEndpoint(method); !allowed {
                return rateLimitResponse(c, info)
            }

            // Tier 4: Per-user (after auth, checked in auth interceptor)
            // Deferred to L3 Auth Interceptor

            // Set rate limit headers
            c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
            c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))

            return next(c)
        }
    }
}

func rateLimitResponse(c echo.Context, info *ratelimit.RateLimitInfo) error {
    c.Response().Header().Set("Retry-After", strconv.Itoa(int(info.RetryAfter.Seconds())))
    return c.JSON(429, map[string]string{"error": "rate limit exceeded"})
}
```

### 3.3 Endpoint-Specific Limits

```go
var defaultEndpointLimits = map[string]*EndpointLimit{
    "/bytebase.v1.AuthService/Login":         {RPM: 10, Burst: 3},
    "/bytebase.v1.SQLService/Query":          {RPM: 120, Burst: 20},
    "/bytebase.v1.SQLService/AdminExecute":   {RPM: 30, Burst: 5},
    "/bytebase.v1.SQLService/Export":          {RPM: 5, Burst: 1},
    "/bytebase.v1.DatabaseService/":          {RPM: 300, Burst: 50},
}
```

### 3.4 Adaptive Rate Limiting

```go
func (e *RateLimitEngine) AdaptiveCheck() {
    // Read system metrics (from existing Prometheus, Architecture L10)
    cpuUsage := getCurrentCPUUsage()
    memUsage := getCurrentMemoryUsage()

    // Reduce capacity under high load
    loadFactor := 1.0
    if cpuUsage > 0.8 || memUsage > 0.8 {
        loadFactor = 0.5 // Halve all limits
    } else if cpuUsage > 0.6 || memUsage > 0.6 {
        loadFactor = 0.75
    }
    e.globalLimiter.SetCapacity(e.baseGlobalCapacity * loadFactor)
}
```

### 3.5 DDoS Protection

```go
func ddosProtectionMiddleware(maxConnsPerIP int) echo.MiddlewareFunc {
    connTracker := &sync.Map{} // IP → *atomic.Int64
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            ip := resolveClientIP(c)
            counter := getOrCreate(connTracker, ip)
            current := counter.Add(1)
            defer counter.Add(-1)

            if current > int64(maxConnsPerIP) {
                return c.JSON(429, map[string]string{"error": "too many concurrent connections"})
            }
            return next(c)
        }
    }
}
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-003 (Brute-Force) | Login endpoint rate limits |
| CR-SEC-006 (IP Allow) | IP blocking complements rate limiting |
| CR-SEC-010 (SIEM) | Rate limit violations as security events |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Token bucket engine + global/IP limits | Sprint 1 |
| 2 | Per-endpoint limits + response headers | Sprint 1 |
| 3 | DDoS protection (concurrent connection limit) | Sprint 2 |
| 4 | Adaptive rate limiting + admin UI | Sprint 3 |
