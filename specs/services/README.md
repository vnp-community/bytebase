# Gateway + Services Migration — Specs

## Documentation Map

| Document | Description |
|----------|-------------|
| [architecture-overview.md](./architecture-overview.md) | High-level architecture, multi-protocol communication |
| [gateway-service.md](./gateway-service.md) | Gateway HTTP reverse proxy specification |
| [core-services.md](./core-services.md) | DCM, SQL, Admin service specs (RESTful/ConnectRPC) |
| [runner-service.md](./runner-service.md) | Runner service + NATS subscriber specification |
| [migration-plan.md](./migration-plan.md) | 5-phase migration plan (9-13 days) |
| [adr-001-gateway-services.md](./adr-001-gateway-services.md) | Architecture Decision Record |
| [tasks/](./tasks/) | Detailed implementation tasks |

## Key Decisions

1. **Single Binary** — All modules run in 1 Go binary
2. **RESTful/ConnectRPC** (via bufconn) — Services keep existing HTTP handlers unchanged
3. **NATS** (embedded) — Async events replace Go channels
4. **gRPC** (bufconn) — Available for cross-service calls
5. **Zero `api/v1/` changes** — Business logic untouched
6. **Zero runner changes** — NATSBus implements EventBus interface
7. **Zero frontend changes** — Same external API

## Communication Protocol Map

```
External Client ──HTTP──→ Gateway ──HTTP(bufconn)──→ Service
                                                       │
                                                  NATS pub
                                                       │
                                                       ▼
                                                  NATS(embed) → Runners
```
