# TASK-SEC-026 — SIEM Forwarder Runner

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-026                               |
| **Source**       | SOL-SEC-010 §3.3                           |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 2                                   |

---

## Mô tả

Implement SIEM Forwarder runner (L6) consuming SecurityEventChan, enriching events, và forwarding to configured targets.

## Scope

1. **SIEMForwarder**: `runner/siem/forwarder.go` — consume SecurityEventChan, enrich with GeoIP, persist to store, forward to targets
2. **SIEMTarget interface**: `Format(event) → []byte`, `Send(data) → error`
3. **Implementations**:
   - `SyslogTarget`: RFC 5424 format, TCP/UDP
   - `SplunkHECTarget`: HTTP POST to Splunk HEC endpoint
   - `ElasticsearchTarget`: Bulk index API
   - `WebhookTarget`: HTTP POST with HMAC signature
4. **Retry queue**: Persistent queue for failed deliveries, exponential backoff
5. **Event formats**: CEF (Common Event Format), JSON, OCSF
6. **Bootstrap**: Register in server startup

## Acceptance Criteria

- [ ] Events forwarded to configured targets
- [ ] Failed deliveries retried with backoff
- [ ] Syslog RFC 5424 compliant
- [ ] Splunk HEC integration verified
- [ ] Webhook HMAC signed

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/siem/forwarder.go` | New file |
| `backend/runner/siem/syslog.go` | Syslog target |
| `backend/runner/siem/splunk.go` | Splunk HEC target |
| `backend/runner/siem/elasticsearch.go` | ES target |
| `backend/runner/siem/webhook.go` | Webhook target |
| `backend/server/server.go` | Bootstrap |

## Definition of Done

- At least Syslog + Webhook targets verified
