# Solution: CR-ENT-015 — External Secret Manager

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-015                |
| **Solution**   | SOL-ENT-015               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Enhance existing Secret plugin (L7 `plugin/secret/` và L5 `component/secret/`) để hỗ trợ Vault, AWS SM, GCP SM. Instance credentials tham chiếu external secrets qua URI scheme. Secrets resolved at runtime bởi DBFactory (L5).

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L7 — Plugin** | `plugin/secret/` | Vault, AWS SM, GCP SM client implementations |
| **L5 — Component** | `component/secret/` | Secret resolution + caching |
| **L5 — Component** | `component/dbfactory/` | Use resolved credentials for DB connections |
| **L4 — Service** | `instance_service.go` | Store secret references instead of plaintext |
| **L4 — Service** | `setting_service.go` | Secret manager configuration |
| **L9 — Enterprise** | `feature.go` | `FeatureExternalSecretManager` gate |

---

## 3. Chi tiết Implementation

### 3.1 Secret Reference URI Scheme

```
vault://secret/data/bytebase/prod-pg#password
aws-sm://arn:aws:secretsmanager:us-east-1:123:secret:prod-pg#password
gcp-sm://projects/my-project/secrets/prod-pg/versions/latest
```

### 3.2 Secret Resolution in DBFactory

```go
func (f *DBFactory) resolveCredentials(ctx context.Context, ds *store.DataSourceMessage) (*Credentials, error) {
    if isSecretRef(ds.Password) {
        // Parse URI scheme → route to correct provider
        provider := f.secretManager.GetProvider(parseScheme(ds.Password))
        password, err := provider.GetSecret(ctx, parseRef(ds.Password))
        if err != nil {
            // Fallback to cached value if available
            return f.cache.Get(ds.Password)
        }
        f.cache.Set(ds.Password, password, 5*time.Minute) // TTL cache
        return &Credentials{Password: password}, nil
    }
    return &Credentials{Password: ds.Password}, nil // local credential
}
```

### 3.3 Secret Rotation Support

- Detect rotated secrets via Vault lease expiry / AWS rotation events
- Refresh DB connections when secrets change
- Zero-downtime rotation

### 3.4 Migration Wizard

One-click migration: local credentials → external SM
1. Verify external SM connectivity
2. Write credentials to external SM
3. Update instance configs to use secret refs
4. Wipe local credentials from Bytebase DB

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-008 | SSO client secrets stored in external SM |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Vault integration | Sprint 1 |
| 2 | AWS Secrets Manager | Sprint 2 |
| 3 | GCP Secret Manager | Sprint 2 |
| 4 | Migration wizard | Sprint 3 |
| 5 | Rotation support | Sprint 3 |
