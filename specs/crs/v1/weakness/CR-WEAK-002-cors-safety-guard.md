# Change Request: CORS Safety Guard & Production Protection

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-002                                              |
| **Weakness ID**    | WEAK-002                                                 |
| **Title**          | CORS Safety Guard — Prevent Dev Mode in Production       |
| **Category**       | Security (SEC)                                           |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SEC-01 (IAM), ADM-08 (API Integration)                   |

---

## 1. Tổng quan

### 1.1 Mô tả
Ngăn chặn CORS wildcard misconfiguration trong production bằng runtime safety guards, configurable CORS origins cho reverse proxy deployments, và automated testing.

### 1.2 Bối cảnh
Dev mode CORS hiện tại sử dụng `UnsafeAllowOriginFunc` cho phép **bất kỳ origin nào** kèm credentials. Nếu production server bị misconfigure với `ReleaseModeDev`, attacker website có thể gọi API với user's cookies — full CORS bypass.

### 1.3 Mục tiêu
- Startup warning + metric khi dev mode detected
- Configurable allowed origins cho production reverse proxy scenarios
- Automated test đảm bảo CORS không active trong release mode
- Audit log cho CORS violations

---

## 2. Yêu cầu chức năng

### FR-001: Dev Mode Runtime Warning
- **Mô tả**: Log CRITICAL warning và emit metric khi server starts trong dev mode.
- **Logic**:
  ```go
  if profile.Mode == common.ReleaseModeDev {
      slog.Warn("⚠️  SERVER RUNNING IN DEV MODE — CORS allows ALL origins",
          slog.String("mode", "dev"),
          slog.Bool("cors_wildcard", true))
      metrics.DevModeActive.Set(1)
      // Thêm banner trên UI: "Development Mode — Not for Production"
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Startup log chứa WARNING level message khi dev mode
  - AC-2: Prometheus metric `bytebase_dev_mode_active` = 1 khi dev mode
  - AC-3: UI banner hiển thị khi dev mode

### FR-002: Configurable CORS Origins cho Production
- **Mô tả**: Cho phép configure allowed CORS origins qua environment variable cho deployments behind reverse proxy.
- **Logic**:
  ```go
  if profile.Mode == common.ReleaseModeRelease {
      if allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); allowedOrigins != "" {
          origins := strings.Split(allowedOrigins, ",")
          e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
              AllowOrigins:     origins,
              AllowCredentials: true,
              AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
              AllowHeaders:     []string{"Authorization", "Content-Type", "X-Request-Id"},
          }))
      }
      // Nếu không có CORS_ALLOWED_ORIGINS → no CORS middleware (same-origin only)
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Production mode mặc định không có CORS (same-origin)
  - AC-2: `CORS_ALLOWED_ORIGINS` cho phép specific origins
  - AC-3: Wildcard `*` bị reject trong production mode
  - AC-4: Origin validation chống regex injection

### FR-003: CORS Violation Audit Logging
- **Mô tả**: Log CORS violations (rejected origins) vào audit trail.
- **Acceptance Criteria**:
  - AC-1: Rejected CORS requests logged với origin, path, timestamp
  - AC-2: Metric `bytebase_cors_rejected_total` counter

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| CORS Middleware        | `backend/server/echo_routes.go`       | Refactor CORS config, add production origins |
| Dev Mode Guard         | `backend/server/server.go`            | Startup warning + Prometheus metric          |
| Config                 | `backend/component/config/profile.go` | Add `CORSAllowedOrigins` config field        |
| Audit Logger           | `backend/server/cors_audit.go`        | CORS violation logging middleware            |

### 3.2 Configuration

| Environment Variable      | Default      | Mô tả                              |
|---------------------------|--------------|-------------------------------------|
| `CORS_ALLOWED_ORIGINS`    | _(empty)_    | Comma-separated allowed origins     |
| `CORS_MAX_AGE`            | `3600`       | Preflight cache duration (seconds)  |

### 3.3 Không có Database Changes

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                  |
|------------|---------------------------------------------------------------|----------------------------------|
| TC-001     | Release mode, no CORS config → cross-origin request          | Rejected (no CORS headers)       |
| TC-002     | Release mode, CORS_ALLOWED_ORIGINS=https://app.example.com   | Only that origin allowed         |
| TC-003     | Release mode, CORS_ALLOWED_ORIGINS=* (wildcard)              | Startup error, server refuses    |
| TC-004     | Dev mode → startup logs contain WARNING                      | Warning logged, metric = 1       |
| TC-005     | Release mode → no dev mode warning                           | No warning, metric = 0           |
| TC-006     | CORS rejection → audit log entry created                     | Origin + path logged             |
| TC-007     | Preflight OPTIONS request with valid origin                   | 204 with CORS headers            |
| TC-008     | Credential request from non-allowed origin                    | No Access-Control-Allow headers  |

---

## 5. Rollout Plan

| Phase   | Mô tả                                   | Timeline     |
|---------|------------------------------------------|--------------|
| Phase 1 | Dev mode warning + metric                | Sprint 1     |
| Phase 2 | Configurable CORS origins                | Sprint 1     |
| Phase 3 | CORS violation audit logging             | Sprint 2     |
| Phase 4 | UI dev mode banner                       | Sprint 2     |
| Phase 5 | Automated integration tests              | Sprint 2     |

---

## 6. Risks & Mitigations

| Risk                                     | Impact | Mitigation                               |
|------------------------------------------|--------|------------------------------------------|
| Existing deployments rely on no-CORS     | LOW    | Default unchanged (no CORS in production)|
| Reverse proxy CORS conflicts             | MEDIUM | Document CORS_ALLOWED_ORIGINS usage      |
| Dev mode banner confusion in dev env     | LOW    | Only show in browser, not API responses  |
