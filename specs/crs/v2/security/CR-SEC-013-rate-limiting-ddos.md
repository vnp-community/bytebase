# Change Request: Rate Limiting & DDoS Protection

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-013                                               |
| **Feature ID**     | ADM-08                                                   |
| **Title**          | Rate Limiting & DDoS Protection                         |
| **Plan**           | ALL (graduated controls)                                 |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai rate limiting toàn diện cho tất cả API endpoints và DDoS protection layer. Bao gồm per-user, per-IP, per-endpoint rate limits, adaptive rate limiting, và integration với external WAF.

### 1.2 Bối cảnh
Bytebase expose 30+ gRPC services qua REST API (ADM-08). Hiện chưa có rate limiting layer, tạo rủi ro DoS và API abuse.

---

## 2. Yêu cầu chức năng

### FR-001: Multi-Tier Rate Limiting
- **Configuration**:
  ```yaml
  rate_limits:
    global:
      requests_per_second: 10000
    per_ip:
      requests_per_minute: 300
      burst: 50
    per_user:
      requests_per_minute: 600
      burst: 100
    per_endpoint:
      "/v1/sql/execute":
        requests_per_minute: 30
        burst: 5
      "/v1/auth/login":
        requests_per_minute: 10
        burst: 3
      "/v1/export/*":
        requests_per_minute: 5
        burst: 1
  ```
- **Acceptance Criteria**:
  - AC-1: Sliding window algorithm (token bucket or leaky bucket)
  - AC-2: Per-IP, per-user, per-endpoint, global tiers
  - AC-3: Configurable limits per endpoint
  - AC-4: HTTP 429 response with `Retry-After` header
  - AC-5: Rate limit headers in response (`X-RateLimit-*`)
  - AC-6: Service accounts can have elevated limits

### FR-002: Adaptive Rate Limiting
- **Acceptance Criteria**:
  - AC-1: Auto-reduce limits under high load
  - AC-2: Detect and throttle bursty clients
  - AC-3: Priority queue: authenticated > anonymous requests
  - AC-4: Circuit breaker for database connection endpoints

### FR-003: DDoS Protection
- **Acceptance Criteria**:
  - AC-1: Connection-level rate limiting
  - AC-2: Slowloris attack detection and mitigation
  - AC-3: Request body size limits (already 100MB, but tighter per endpoint)
  - AC-4: Concurrent request limit per client
  - AC-5: Integration with external WAF (Cloudflare, AWS WAF) via headers

### FR-004: API Abuse Detection
- **Acceptance Criteria**:
  - AC-1: Pattern-based abuse detection (scraping, enumeration)
  - AC-2: Auto-block abusive clients (configurable duration)
  - AC-3: Alert admin on detected abuse
  - AC-4: Abuse patterns dashboard

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Rate Limiter Middleware (new)| `backend/api/middleware/ratelimit.go`        | Multi-tier rate limiting                    |
| Rate Limit Store (new)       | `backend/component/ratelimit/`              | In-memory + Redis backed counters           |
| DDoS Protection (new)        | `backend/api/middleware/ddos.go`             | Connection-level protection                 |
| API Gateway Config           | `backend/server/server.go`                  | Middleware registration                     |
| Rate Limit Dashboard         | `frontend/src/views/RateLimitSettings.vue`  | Configuration UI                            |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Exceed per-user rate limit                           | 429 with Retry-After header      |
| TC-002  | SQL execute endpoint burst                           | Throttled after burst limit      |
| TC-003  | Anonymous vs authenticated priority                  | Authenticated gets through       |
| TC-004  | Slowloris attack simulation                          | Connection terminated             |
| TC-005  | Service account elevated limits                      | Higher threshold applied         |
| TC-006  | Adaptive limiting under load                         | Limits auto-reduced              |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Multi-tier rate limiting             | Sprint 1       |
| Phase 2 | DDoS protection                      | Sprint 2       |
| Phase 3 | Adaptive rate limiting               | Sprint 3       |
| Phase 4 | Abuse detection + dashboard          | Sprint 4       |
