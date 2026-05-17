# Solution: CR-SEC-014 — Mutual TLS (mTLS) for Service Communication

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-014                |
| **Solution**   | SOL-SEC-014               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Xây dựng TLS Manager component (L5) để quản lý certificate lifecycle. Extend DB Driver plugin interface (L7) thêm mTLS configuration per engine. Harden Echo server TLS config (L2). Client certificate authentication thêm vào Auth Interceptor (L3) như authentication method bổ sung.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L2** | `server.go` (11.4KB) | TLS server config hardening |
| **L3** | Auth Interceptor | Client cert as auth method |
| **L5** | `component/tls/` (new) | Certificate management |
| **L7** | `plugin/db/*/` (22 drivers) | Per-engine mTLS config |
| **L8** | `store/instance.go` | Certificate storage per instance |

---

## 3. Chi tiết Implementation

### 3.1 L5 — TLS Manager Component

```go
type TLSManager struct {
    store         *store.Store
    certCache     *lru.Cache[string, *tls.Certificate]
    watchTicker   *time.Ticker
}

func (m *TLSManager) GetServerTLSConfig() *tls.Config {
    return &tls.Config{
        MinVersion:               tls.VersionTLS12,
        PreferServerCipherSuites: true,
        CipherSuites: []uint16{
            tls.TLS_AES_128_GCM_SHA256,       // TLS 1.3
            tls.TLS_AES_256_GCM_SHA384,       // TLS 1.3
            tls.TLS_CHACHA20_POLY1305_SHA256,  // TLS 1.3
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,   // TLS 1.2
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, // TLS 1.2
        },
        GetCertificate: m.getCertificate,
        ClientAuth:     tls.VerifyClientCertIfGiven, // Optional client cert
        ClientCAs:      m.loadTrustedCAs(),
    }
}

func (m *TLSManager) MonitorExpiry(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    for {
        select {
        case <-ticker.C:
            certs := m.store.ListCertificates(ctx)
            for _, cert := range certs {
                daysUntilExpiry := time.Until(cert.NotAfter).Hours() / 24
                if daysUntilExpiry < 30 {
                    m.alertCertExpiry(ctx, cert, int(daysUntilExpiry))
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 3.2 L7 — DB Driver mTLS Extension

Extend DB driver connection config (TDD Section 6.1):

```go
// Extend existing connection config for all 22 drivers
type ConnectionConfig struct {
    // Existing fields...
    Host     string
    Port     string
    Username string
    Password string

    // NEW: mTLS configuration
    TLS *TLSConfig
}

type TLSConfig struct {
    Mode       string // "disable", "require", "verify-ca", "verify-full"
    CACert     []byte // Custom CA certificate
    ClientCert []byte // Client certificate for mTLS
    ClientKey  []byte // Client private key
    ServerName string // Expected server CN for verification
}

// Per-engine implementation example (PostgreSQL)
func (d *Driver) buildPGXConfig(config ConnectionConfig) *pgx.ConnConfig {
    pgConfig, _ := pgx.ParseConfig(dsn)
    if config.TLS != nil && config.TLS.Mode != "disable" {
        tlsConfig := &tls.Config{
            MinVersion: tls.VersionTLS12,
            ServerName: config.TLS.ServerName,
        }
        if config.TLS.CACert != nil {
            pool := x509.NewCertPool()
            pool.AppendCertsFromPEM(config.TLS.CACert)
            tlsConfig.RootCAs = pool
        }
        if config.TLS.ClientCert != nil {
            cert, _ := tls.X509KeyPair(config.TLS.ClientCert, config.TLS.ClientKey)
            tlsConfig.Certificates = []tls.Certificate{cert}
        }
        pgConfig.TLSConfig = tlsConfig
    }
    return pgConfig
}
```

### 3.3 L3 — Client Certificate Authentication

```go
func (a *AuthInterceptor) authenticateClientCert(ctx context.Context, req *http.Request) (*UserContext, error) {
    if req.TLS == nil || len(req.TLS.PeerCertificates) == 0 {
        return nil, nil // No client cert, try other methods
    }

    clientCert := req.TLS.PeerCertificates[0]

    // Map certificate to service account
    sa, err := a.store.GetServiceAccountByCertFingerprint(ctx,
        sha256Hex(clientCert.Raw))
    if err != nil {
        return nil, status.Errorf(codes.Unauthenticated, "unknown client certificate")
    }

    return &UserContext{User: sa.Principal}, nil
}
```

### 3.4 L2 — Server TLS Hardening

**File**: `backend/server/server.go` (extend existing 11.4KB)

```go
func (s *Server) Run(ctx context.Context, port int) error {
    // Extend existing HTTP server setup with TLS config
    httpServer := &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: h2cHandler, // existing
        TLSConfig: s.tlsManager.GetServerTLSConfig(),
    }

    // Add security headers (extend existing middleware)
    // HSTS with preload
    c.Response().Header().Set("Strict-Transport-Security",
        "max-age=63072000; includeSubDomains; preload")
}
```

---

## 4. Database Changes

```sql
CREATE TABLE certificate (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL,     -- "server", "client", "ca"
    fingerprint  TEXT NOT NULL UNIQUE,
    not_before   TIMESTAMPTZ NOT NULL,
    not_after    TIMESTAMPTZ NOT NULL,
    subject_cn   TEXT,
    issuer_cn    TEXT,
    cert_pem     TEXT NOT NULL,
    key_pem      TEXT,             -- encrypted via SOL-SEC-007
    instance_uid INT,             -- if per-instance
    created_ts   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-007 (Encryption) | Certificate private keys encrypted at rest |
| CR-ENT-015 (Secret Manager) | Keys stored in Vault |
| CR-SEC-008 (Credential Rotation) | Certificate rotation pattern |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | TLS server config hardening | Sprint 1 |
| 2 | DB driver mTLS extension (PostgreSQL, MySQL) | Sprint 2 |
| 3 | Certificate lifecycle management | Sprint 3 |
| 4 | Client certificate authentication | Sprint 4 |
