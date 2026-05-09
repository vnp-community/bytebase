# WEAK-001 — CSP Policy with unsafe-inline and wasm-unsafe-eval

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-001                                   |
| Category       | Security                                   |
| Severity       | MEDIUM                                     |
| Affected Layer | L2 (API Gateway)                           |
| Source Files   | `backend/server/echo_routes.go:126-145`    |

---

## Mô tả

Content Security Policy (CSP) header chứa các directive yếu.

## Chi tiết

```go
// echo_routes.go:136-145
csp := "default-src 'self'; " +
    "script-src 'self' " + hashes + " 'wasm-unsafe-eval'; " +
    "style-src 'self' 'unsafe-inline'; " +      // ← WEAKNESS
    "img-src 'self' data: blob: discordapp.com; " +
    "connect-src 'self' data: ws: wss: https://api.github.com https://hub.bytebase.com; " +
    ...
```

### 1. `style-src 'unsafe-inline'`
- Cho phép inline styles → XSS vector qua style injection.
- Source code có TODO comment: `"TODO: Migrate inline styles to CSS classes and remove 'unsafe-inline'"`.
- Nguyên nhân: Vue components sử dụng inline styles.

### 2. `'wasm-unsafe-eval'`
- Cho phép WebAssembly runtime compilation.
- Cần thiết cho Monaco Editor TextMate grammar.
- Tạo attack surface nếu attacker inject WASM module.

### 3. `connect-src` quá rộng
- Cho phép `ws:` (unencrypted WebSocket) và `data:` scheme.
- External domains hardcoded: `api.github.com`, `hub.bytebase.com`.

## Impact

- XSS attacks có thể leverage `unsafe-inline` styles.
- Supply chain attacks qua WASM modules.

## Khuyến nghị

1. Migrate Vue inline styles → CSS classes (xóa `unsafe-inline`).
2. Sử dụng CSP nonces thay vì `unsafe-inline` trong migration period.
3. Restrict `connect-src` — remove `ws:` nếu chỉ cần `wss:`.
