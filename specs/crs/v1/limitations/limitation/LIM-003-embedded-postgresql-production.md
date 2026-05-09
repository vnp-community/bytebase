# LIM-003 — Embedded PostgreSQL Not Suitable for Production

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | LIM-003                                    |
| Category       | Deployment / Infrastructure                |
| Severity       | MEDIUM                                     |
| Affected Layer | L10 (Infrastructure)                       |
| Source Files   | `backend/resources/postgres/`, `backend/server/server.go` |

---

## Mô tả

Bytebase hỗ trợ **embedded PostgreSQL** cho quick-start deployment. Tuy nhiên, mode này có nhiều giới hạn nghiêm trọng cho production.

## Chi tiết hạn chế

### 1. Không hỗ trợ HA mode

```go
// backend/server/server.go:118
if profile.HA && profile.UseEmbedDB() {
    return nil, errors.New("HA mode requires external PostgreSQL (set PG_URL environment variable)")
}
```

- Embedded PG chạy single instance, không có replication.
- HA mode bắt buộc external PostgreSQL.

### 2. Data isolation giới hạn

- PG data directory nằm trong `{dataDir}/pgdata` — cùng filesystem với binary.
- Không có point-in-time recovery (PITR), WAL archiving, hoặc streaming replication.
- Backup phải tự triển khai bằng filesystem-level snapshot.

### 3. Performance tuning hạn chế

- Embedded PG sử dụng default configuration.
- Không có khả năng tuning `shared_buffers`, `work_mem`, `effective_cache_size` riêng.
- Connection pool bị giới hạn bởi embedded PG default `max_connections`.

### 4. Upgrade path phức tạp

- Major PG version upgrade (e.g., PG 15 → PG 16) yêu cầu dump/restore.
- Embedded PG tự quản lý version — không thể upgrade incrementally.

## Impact

| Scenario               | Embedded PG        | External PG          |
|------------------------|--------------------|----------------------|
| High Availability      | ❌ Không hỗ trợ   | ✅ Streaming replication |
| Backup/Recovery        | ❌ Manual          | ✅ pg_basebackup, PITR |
| Performance tuning     | ❌ Limited         | ✅ Full control       |
| Multi-instance scaling | ❌ Impossible      | ✅ Read replicas      |

## Khuyến nghị

- Sử dụng embedded PG chỉ cho **evaluation, demo, development**.
- Production luôn dùng **external managed PostgreSQL** (RDS, Cloud SQL, etc.).
- Cần documentation rõ ràng về migration path: embedded → external PG.
