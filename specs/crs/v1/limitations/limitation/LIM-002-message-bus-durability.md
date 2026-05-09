# LIM-002 — Message Bus Durability & Reliability

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | LIM-002                                    |
| Category       | Reliability                                |
| Severity       | HIGH                                       |
| Affected Layer | L5 (Component — Bus), L6 (Runner)          |
| Source Files   | `backend/component/bus/bus.go`             |

---

## Mô tả

Message bus nội bộ sử dụng **buffered Go channels** cho tất cả inter-component communication. Đây là thiết kế đơn giản nhưng tạo ra nhiều rủi ro reliability.

## Chi tiết hạn chế

### 1. No Durability — Messages mất khi crash

```go
ApprovalCheckChan:       make(chan IssueRef, 1000),
PlanCheckTickleChan:     make(chan int, 1000),
TaskRunTickleChan:       make(chan int, 1000),
RolloutCreationChan:     make(chan PlanRef, 100),
PlanCompletionCheckChan: make(chan PlanRef, 1000),
```

- Khi server crash hoặc restart, **tất cả messages trong channel buffer bị mất**.
- Không có persistence layer, write-ahead log, hoặc retry mechanism.
- Đặc biệt nguy hiểm cho `TaskRunTickleChan` — migration tasks có thể bị bỏ sót.

### 2. Buffer Overflow — Silent message drop

- `RolloutCreationChan` có buffer size chỉ **100** (nhỏ hơn các channel khác là 1000).
- Khi channel đầy, goroutine ghi sẽ **block** — không có backpressure strategy rõ ràng.
- Không có metrics/monitoring cho channel fill level.

### 3. No Dead-Letter Queue

- Messages không được retry nếu consumer xử lý thất bại.
- Không có mechanism để track messages đã bị drop hoặc xử lý lỗi.

### 4. Single Consumer Pattern

- Mỗi channel chỉ có **một consumer** (runner tương ứng).
- Không hỗ trợ fan-out hoặc multiple consumers cho load distribution.

## Impact

- **Migration tasks có thể bị bỏ quên** sau server restart — cần manual intervention.
- **Approval flows có thể bị stale** — issues chờ approval không được process.
- **Burst load** có thể gây channel blocking → toàn bộ pipeline bị trì hoãn.

## Mitigation trong codebase

- PostgreSQL LISTEN/NOTIFY (`runner/notifylistener/`) bridge messages từ DB → bus channels.
- Store ghi trạng thái vào DB trước khi publish → runners có thể poll DB nếu miss channel message.
- Nhưng polling interval tạo delay, không real-time.

## Khuyến nghị

1. Persistent message queue (NATS JetStream, Redis Streams) cho critical paths.
2. Channel metrics exported to Prometheus.
3. Dead-letter handling cho failed messages.
4. Idempotent consumers với deduplication.
