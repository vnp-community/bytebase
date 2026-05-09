# WEAK-002 — CORS Configuration Weakness in Dev Mode

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-002                                   |
| Category       | Security                                   |
| Severity       | LOW (dev only) / HIGH (if misconfigured)   |
| Affected Layer | L2 (API Gateway)                           |
| Source Files   | `backend/server/echo_routes.go:39-49`      |

---

## Mô tả

Dev mode CORS cho phép **bất kỳ origin nào** — nguy hiểm nếu dev mode bị enable trong production.

## Chi tiết

```go
if profile.Mode == common.ReleaseModeDev {
    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        UnsafeAllowOriginFunc: func(_ *echo.Context, origin string) (string, bool, error) {
            return origin, true, nil  // ← Cho phép MỌI origin
        },
        AllowCredentials: true,  // ← Kèm credentials
    }))
}
```

- `UnsafeAllowOriginFunc` trả về `true` cho mọi origin.
- `AllowCredentials: true` kết hợp wildcard origin → classic CORS misconfiguration.
- Production mode **không có CORS middleware** → API chỉ accessible từ same-origin.

## Risk

- Nếu production server bị misconfigure với `ReleaseModeDev` → full CORS bypass.
- Attacker website có thể gọi API với user's cookies.

## Khuyến nghị

1. Thêm startup warning nếu `ReleaseModeDev` detected.
2. Cân nhắc configurable CORS origins cho production behind reverse proxy.
3. Add integration test verifying CORS not enabled in release mode.
