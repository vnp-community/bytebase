# LIM-001 — Monolith Horizontal Scaling Limitation

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | LIM-001                                    |
| Category       | Architecture                               |
| Severity       | HIGH                                       |
| Affected Layer | L2 (API Gateway), L6 (Runner), L8 (Store)  |
| Source Files   | `backend/server/server.go`, `backend/store/store.go` |

---

## Mô tả

Bytebase sử dụng kiến trúc **modular monolith** — toàn bộ 30+ services, 8 background runners, message bus, và LRU caches chạy trong **single Go process**. Điều này tạo ra giới hạn cốt lõi trong horizontal scaling.

## Chi tiết hạn chế

### 1. In-process Cache — Phải disable trong HA mode
```go
// backend/store/store.go
storeInstance, err := store.New(ctx, pgURL, !profile.HA)
// HA mode → enableCache=false → mỗi request đọc trực tiếp từ PostgreSQL
```

- **Single-node**: LRU caches (32K entries cho user/instance/database/project) cung cấp hiệu suất cao.
- **HA mode**: Cache bị tắt hoàn toàn → tất cả requests phải hit DB → tăng latency và load DB đáng kể.
- **Không có distributed cache** (Redis/Memcached) làm cache layer trung gian.

### 2. Message Bus — Go channels, không persistent
```go
// backend/component/bus/bus.go
ApprovalCheckChan:   make(chan IssueRef, 1000),
PlanCheckTickleChan: make(chan int, 1000),
TaskRunTickleChan:   make(chan int, 1000),
```

- Buffered Go channels dung lượng tối đa 1000 messages.
- **Messages bị mất khi server crash** — không có durability guarantee.
- Không thể chia workload giữa nhiều replicas.

### 3. Background Runners — Monolithic goroutines
```go
// backend/server/server.go — Run()
go s.taskScheduler.Run(ctx, &s.runnerWG)      // 1 instance
go s.schemaSyncer.Run(ctx, &s.runnerWG)        // 1 instance
go s.approvalRunner.Run(ctx, &s.runnerWG)      // 1 instance
go s.planCheckScheduler.Run(ctx, &s.runnerWG)  // 1 instance
```

- 8 runners chạy trong mỗi replica → duplicate work trong HA mode.
- Không có leader election hoặc distributed coordination.
- Schema sync chạy trên tất cả replicas đồng thời → load không cần thiết lên managed databases.

### 4. Single PostgreSQL — Bottleneck
- Toàn bộ metadata, audit logs, changelogs, task runs đều ghi vào **cùng một PostgreSQL instance**.
- Không hỗ trợ read replicas cho metadata DB.
- Connection pool capped ở `maxConns - reservedConns`, tối đa 50.

## Impact

| Metric                    | Single-node    | HA mode           |
|---------------------------|----------------|-------------------|
| Query latency             | Low (cache)    | High (no cache)   |
| Max concurrent users      | ~100-200       | ~50-100 (no cache)|
| Message durability        | None           | None              |
| Runner work distribution  | N/A            | Duplicated        |

## Workaround hiện tại

- HA mode sử dụng **PostgreSQL LISTEN/NOTIFY** cho event coordination.
- Heartbeat runner report replica health để license service quản lý.
- Advisory locks (`backend/store/advisory_lock.go`) cho một số task critical.

## Khuyến nghị cải tiến

1. **Distributed cache layer** — Redis/Memcached giữa store và service.
2. **External message queue** — NATS/Kafka thay thế Go channels cho durability.
3. **Leader election** — etcd/consul cho runner coordination trong HA.
4. **Read replica support** — cho metadata queries.
