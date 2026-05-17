# ADR-001: Gateway + Services Architecture (Multi-Protocol)

| Metadata | Value |
|----------|-------|
| Status | Accepted |
| Date | 2026-05-15 |
| Decision | Gateway + Internal Services with RESTful/ConnectRPC + NATS + gRPC |

## Context

Bytebase backend là monolith với ~30 gRPC services, ~10 background runners, all wired trong `server.go` + `grpc_routes.go`. Cần tách thành modules có ranh giới rõ ràng, nhưng giữ single binary deployment.

## Decision

Chọn **Gateway + Internal Services** với **multi-protocol communication**:

| Protocol | Transport | Role |
|----------|-----------|------|
| **RESTful/ConnectRPC** | HTTP via bufconn | Gateway → Services (giữ nguyên existing handlers) |
| **NATS** | Embedded server | Async events (Services → Runners) |
| **gRPC** | bufconn | Available cho cross-service calls khi cần |

### Why RESTful/ConnectRPC for Services?
- Services đã có sẵn ConnectRPC handlers + REST gateway → **zero code changes** cho `api/v1/`
- Gateway chỉ cần HTTP reverse proxy (httputil.ReverseProxy) → đơn giản nhất
- Không cần implement gRPC server interfaces, adapters, hay dual-interface compliance
- curl-friendly cho debugging

### Why NATS for Async?
- Replace Go channels (`bus.Bus`) → durable, observable messaging
- NATSBus implements existing `EventBus` interface → **zero runner changes**
- Extract-ready: embedded NATS → external NATS cluster khi cần scale

### Why gRPC Available?
- Cross-service calls khi cần type-safe, high-perf communication
- Protobuf IDL đã có sẵn (generated code)
- bufconn → TCP migration trivial

## Alternatives Considered

| Alternative | Why Rejected |
|-------------|-------------|
| Pure gRPC internal | Requires all services implement gRPC server interfaces (high code change) |
| True microservices | Premature complexity, operational overhead |
| Message queue only (RabbitMQ/Kafka) | Overkill for current scale, heavy dependency |
| Keep monolith | No module boundaries, hard to maintain |
| Function calls (in-process) | Not extract-ready, no protocol contract |

## Consequences

### Positive
- **Near-zero code changes** to `api/v1/` business logic
- **Zero changes** to runner packages (EventBus interface preserved)
- **Zero changes** to frontend
- **Extract-ready**: bufconn → TCP, embedded NATS → cluster
- **Observable**: HTTP metrics, NATS monitoring
- **Debuggable**: curl to internal services, nats-cli for events

### Negative
- New dependency: NATS (~15MB binary size increase)
- Slight latency overhead: bufconn HTTP ~10-50μs per request
- More initialization code in server.go (mitigated by ServiceRouter)
