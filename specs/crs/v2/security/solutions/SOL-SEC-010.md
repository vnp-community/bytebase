# Solution: CR-SEC-010 — Security Event Monitoring & SIEM Integration

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-010                |
| **Solution**   | SOL-SEC-010               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Xây dựng Security Event Engine (L5) tách biệt từ Audit Interceptor (L3), với event classification, enrichment, và routing. SIEM Forwarder runner (L6) sử dụng pattern tương tự existing Bus (L5) nhưng dành cho security events. Support multi-target forwarding: Syslog, Splunk HEC, ELK, webhook. Anomaly detection runner (L6) cho impossible travel, access pattern analysis.

---

## 2. Architectural Alignment

```
L3 Audit Interceptor ──► SecurityEventBus (L5, new)
L4 Service (auth/sql) ──►        │
                                  ├── SecurityEventEngine (L5)
                                  │    ├── Classifier: Event categorization
                                  │    ├── Enricher: Add context (GeoIP, risk)
                                  │    └── Router: Route to handlers
                                  │
                                  ├── SIEMForwarder (L6)
                                  │    ├── Syslog (RFC 5424)
                                  │    ├── Splunk HEC
                                  │    ├── Elasticsearch
                                  │    └── Custom Webhook
                                  │
                                  └── AnomalyDetector (L6)
                                       ├── Impossible travel
                                       ├── Access pattern analysis
                                       └── Off-hours detection
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5** | `component/security_event/` (new) | Event engine: classify, enrich, route |
| **L5** | Bus pattern: `SecurityEventChan` | Buffered channel for security events |
| **L6** | `runner/siem/` (new) | Multi-target log forwarding |
| **L6** | `runner/anomaly/` (new) | Anomaly detection |
| **L8** | `store/security_event.go` (new) | Event persistence |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Security Event Bus

**File**: `backend/component/bus/bus.go` (extend existing)

Add security event channel to existing Bus struct (TDD Section 5.1):

```go
type Bus struct {
    // Existing channels (TDD Section 5.1)
    ApprovalCheckChan       chan IssueRef
    PlanCheckTickleChan     chan int
    TaskRunTickleChan       chan int
    // ...

    // NEW: Security event channel
    SecurityEventChan       chan *SecurityEvent  // buffer: 5000
}

type SecurityEvent struct {
    ID          string
    Category    SecurityCategory  // AUTH, ACCESS, DATA, CONFIG, SCHEMA, SYSTEM
    Severity    Severity          // CRITICAL, HIGH, MEDIUM, LOW, INFO
    Timestamp   time.Time
    Actor       *ActorInfo
    Resource    string
    Action      string
    Detail      map[string]any
    SourceIP    string
    GeoLocation *GeoResult
}
```

### 3.2 L3 — Audit Interceptor Security Event Emission

**File**: `backend/api/v1/audit.go` (extend existing 25KB)

```go
func (a *AuditInterceptor) postProcess(ctx context.Context, method string, err error) {
    // Existing audit log logic...

    // NEW: Emit security events for security-relevant methods
    if se := classifySecurityEvent(method, err); se != nil {
        se.Actor = buildActorInfo(ctx)
        se.SourceIP = extractClientIP(ctx)
        // Non-blocking send to security event channel
        select {
        case a.bus.SecurityEventChan <- se:
        default:
            // Channel full — log warning, don't block request
            slog.Warn("security event channel full, dropping event", "method", method)
        }
    }
}

func classifySecurityEvent(method string, err error) *SecurityEvent {
    switch {
    case strings.Contains(method, "AuthService.Login") && err != nil:
        return &SecurityEvent{Category: AUTH, Severity: MEDIUM, Action: "login_failed"}
    case strings.Contains(method, "SQLService.AdminExecute"):
        return &SecurityEvent{Category: DATA, Severity: HIGH, Action: "admin_execute"}
    case strings.Contains(method, "SetIamPolicy"):
        return &SecurityEvent{Category: CONFIG, Severity: HIGH, Action: "iam_policy_change"}
    // ... more classifications
    }
    return nil
}
```

### 3.3 L6 — SIEM Forwarder Runner

**File**: `backend/runner/siem/forwarder.go` (new)

Add to server bootstrap (TDD Section 2, step 9):

```go
type SIEMForwarder struct {
    bus     *bus.Bus
    targets []SIEMTarget
    store   *store.Store
    queue   *RetryQueue  // persistent queue for guaranteed delivery
}

func (f *SIEMForwarder) Run(ctx context.Context) {
    for {
        select {
        case event := <-f.bus.SecurityEventChan:
            // Enrich event
            event.GeoLocation = f.geoIP.Lookup(event.SourceIP)

            // Store event
            f.store.CreateSecurityEvent(ctx, event)

            // Forward to all configured targets
            for _, target := range f.targets {
                formatted := target.Format(event) // CEF, JSON, OCSF
                if err := target.Send(formatted); err != nil {
                    f.queue.Enqueue(target.ID(), formatted) // Retry later
                }
            }

        case <-ctx.Done():
            return
        }
    }
}

// SIEM target implementations
type SyslogTarget struct { conn net.Conn }
type SplunkHECTarget struct { httpClient *http.Client; token string; url string }
type ElasticsearchTarget struct { client *elasticsearch.Client; index string }
type WebhookTarget struct { url string; secret string }
```

### 3.4 L6 — Anomaly Detection Runner

**File**: `backend/runner/anomaly/detector.go` (new)

```go
type AnomalyDetector struct {
    store  *store.Store
    geoIP  *geoip.Service
    bus    *bus.Bus
    rules  []DetectionRule
}

func (d *AnomalyDetector) Run(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ticker.C:
            d.detectImpossibleTravel(ctx)
            d.detectAnomalousAccess(ctx)
            d.detectOffHoursActivity(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (d *AnomalyDetector) detectImpossibleTravel(ctx context.Context) {
    // Query recent logins grouped by user
    recentLogins, _ := d.store.GetRecentLoginPairs(ctx, 2*time.Hour)
    for _, pair := range recentLogins {
        distance := haversineDistance(pair.Login1.Lat, pair.Login1.Lon, pair.Login2.Lat, pair.Login2.Lon)
        timeDiff := pair.Login2.Time.Sub(pair.Login1.Time)
        maxSpeed := distance / timeDiff.Hours() // km/h

        if maxSpeed > 1000 { // Faster than commercial flight
            d.emitAlert(ctx, &SecurityEvent{
                Category: AUTH, Severity: CRITICAL,
                Action: "impossible_travel",
                Detail: map[string]any{
                    "distance_km": distance, "time_diff_min": timeDiff.Minutes(),
                },
            })
        }
    }
}
```

---

## 4. Database Changes

```sql
CREATE TABLE security_event (
    id          BIGSERIAL PRIMARY KEY,
    category    TEXT NOT NULL,
    severity    TEXT NOT NULL,
    action      TEXT NOT NULL,
    actor_uid   INT,
    actor_email TEXT,
    resource    TEXT,
    detail      JSONB,
    source_ip   TEXT,
    geo_country TEXT,
    geo_city    TEXT,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_security_event_ts ON security_event (created_ts DESC);
CREATE INDEX idx_security_event_category ON security_event (category, severity);
CREATE INDEX idx_security_event_actor ON security_event (actor_uid);

CREATE TABLE siem_target (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,     -- "syslog", "splunk", "elasticsearch", "webhook"
    config      JSONB NOT NULL,    -- target-specific configuration
    filter      JSONB,             -- event filter (categories, severity threshold)
    is_active   BOOLEAN NOT NULL DEFAULT TRUE
);
```

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-003 (Audit Log) | Security events extend audit log |
| CR-SEC-003 (Brute-Force) | Auth events detected by anomaly engine |
| CR-SEC-011 (Tamper-Proof) | Events stored with integrity protection |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Security event bus + classification | Sprint 1 |
| 2 | SIEM forwarder (syslog + webhook) | Sprint 2 |
| 3 | Anomaly detection rules | Sprint 3 |
| 4 | Security dashboard + Splunk/ELK targets | Sprint 4 |
