# Solution: CR-SEC-018 — Security Incident Response Automation

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-018                |
| **Solution**   | SOL-SEC-018               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Xây dựng Incident Engine (L5) kết nối với Security Event Bus (SOL-SEC-010). Playbook Runner (L6) execute automated response actions. Escalation service tích hợp existing Webhook Manager (L5: `component/webhook/`). Forensic data preservation via snapshot mechanism. Sử dụng Bus pattern (TDD Section 5.1) cho incident lifecycle events.

---

## 2. Architectural Alignment

```
SecurityEventBus (SOL-SEC-010)
      │
      ▼
L5 Component ──► component/incident/ (new)
                      │
                      ├── IncidentEngine: Classify → Create Incident
                      ├── PlaybookEngine: Match trigger → Execute actions
                      └── EscalationService: Multi-channel notification
                      │
L6 Runner ──► runner/incident/ (new)
                      │
                      ├── PlaybookRunner: Automated response execution
                      ├── EscalationRunner: SLA monitoring + auto-escalate
                      └── ForensicRunner: Evidence preservation
                      │
L5 Component ──► component/webhook/ (existing)
                      └── IM notification dispatch (Slack, DingTalk, Teams)
                      │
L8 Store ──► store/incident.go (new)
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L5** | `component/incident/` (new) | Incident engine, playbook engine |
| **L5** | `component/webhook/` (existing) | Escalation notifications |
| **L6** | `runner/incident/` (new) | Playbook execution, SLA monitor |
| **L8** | `store/incident.go` (new) | Incident + evidence persistence |

---

## 3. Chi tiết Implementation

### 3.1 L5 — Incident Engine

**File**: `backend/component/incident/engine.go` (new)

```go
type IncidentEngine struct {
    store     *store.Store
    playbooks *PlaybookEngine
    bus       *bus.Bus
}

type Incident struct {
    ID            string
    Severity      Severity        // SEV1-SEV4
    Category      string          // "account_compromise", "data_breach", etc.
    TriggerEvent  *SecurityEvent
    Status        string          // "open", "acknowledged", "mitigating", "resolved"
    Assignee      string
    Timeline      []TimelineEntry
    Evidence      []EvidenceRef
    CreatedAt     time.Time
    ResolvedAt    *time.Time
}

func (e *IncidentEngine) ProcessSecurityEvent(ctx context.Context, event *SecurityEvent) {
    // Classify incident severity
    severity := e.classifyIncident(event)
    if severity == NONE { return } // Not incident-worthy

    // Create incident
    incident := &Incident{
        ID:           generateIncidentID(),
        Severity:     severity,
        Category:     event.Category.String(),
        TriggerEvent: event,
        Status:       "open",
        CreatedAt:    time.Now(),
    }
    e.store.CreateIncident(ctx, incident)

    // Execute matching playbook
    playbook := e.playbooks.FindPlaybook(incident)
    if playbook != nil {
        e.playbooks.Execute(ctx, playbook, incident)
    }

    // Trigger escalation
    e.bus.IncidentChan <- incident
}

func (e *IncidentEngine) classifyIncident(event *SecurityEvent) Severity {
    switch {
    case event.Severity == CRITICAL && event.Category == AUTH:
        return SEV1 // Active compromise
    case event.Action == "impossible_travel":
        return SEV2
    case event.Action == "bulk_export" && event.Severity == HIGH:
        return SEV2
    case event.Severity == HIGH:
        return SEV3
    default:
        return NONE
    }
}
```

### 3.2 L5 — Playbook Engine

**File**: `backend/component/incident/playbook.go` (new)

```go
type PlaybookEngine struct {
    store         *store.Store
    authService   *AuthService
    sessionStore  *SessionStore
    ipBlocker     *IPBlocker
}

type Playbook struct {
    ID      string
    Name    string
    Trigger PlaybookTrigger // event type + conditions
    Actions []PlaybookAction
    Mode    string // "auto", "confirm" (human-in-the-loop)
}

type PlaybookAction struct {
    Type   string         // "lock_account", "revoke_sessions", "block_ip", "notify"
    Params map[string]any
    Delay  time.Duration  // Optional delay between actions
}

func (p *PlaybookEngine) Execute(ctx context.Context, playbook *Playbook, incident *Incident) {
    for i, action := range playbook.Actions {
        // Log action execution
        incident.Timeline = append(incident.Timeline, TimelineEntry{
            Time:   time.Now(),
            Action: fmt.Sprintf("Playbook %s: executing action %s", playbook.Name, action.Type),
        })

        if playbook.Mode == "confirm" {
            // Human-in-the-loop: wait for admin confirmation
            p.requestConfirmation(ctx, incident, action)
            continue
        }

        // Execute action
        switch action.Type {
        case "lock_account":
            p.authService.LockAccount(ctx, incident.TriggerEvent.Actor.Email)
        case "revoke_sessions":
            p.sessionStore.RevokeAllSessions(ctx, incident.TriggerEvent.Actor.UID)
        case "block_ip":
            p.ipBlocker.BlockIP(ctx, incident.TriggerEvent.SourceIP, 24*time.Hour)
        case "notify":
            p.notifyTeam(ctx, action.Params["channel"].(string), incident)
        }

        if action.Delay > 0 {
            time.Sleep(action.Delay)
        }
    }
}
```

### 3.3 L6 — Escalation Runner

**File**: `backend/runner/incident/escalation.go` (new)

```go
type EscalationRunner struct {
    store          *store.Store
    webhookManager *webhook.Manager // Existing L5 component
    bus            *bus.Bus
    slaConfig      map[Severity]time.Duration
}

func (r *EscalationRunner) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            r.checkSLABreaches(ctx)
        case incident := <-r.bus.IncidentChan:
            r.triggerInitialEscalation(ctx, incident)
        case <-ctx.Done():
            return
        }
    }
}

func (r *EscalationRunner) checkSLABreaches(ctx context.Context) {
    openIncidents, _ := r.store.ListOpenIncidents(ctx)
    for _, incident := range openIncidents {
        sla := r.slaConfig[incident.Severity]
        age := time.Since(incident.CreatedAt)

        if age > sla && incident.Status == "open" {
            // Auto-escalate
            r.escalate(ctx, incident)
        }
    }
}

func (r *EscalationRunner) escalate(ctx context.Context, incident *Incident) {
    // Use existing Webhook Manager for notification dispatch
    // Supports: Slack, DingTalk, Feishu, Teams (Architecture L5)
    r.webhookManager.Send(ctx, webhook.Message{
        Type:    "incident_escalation",
        Title:   fmt.Sprintf("[%s] Incident %s — SLA breach", incident.Severity, incident.ID),
        Content: formatIncidentSummary(incident),
    })
}
```

### 3.4 L6 — Forensic Data Runner

```go
type ForensicRunner struct {
    store *store.Store
}

func (r *ForensicRunner) PreserveEvidence(ctx context.Context, incident *Incident) error {
    actor := incident.TriggerEvent.Actor

    // Snapshot audit logs for this user (last 72h)
    auditLogs, _ := r.store.SearchAuditLogs(ctx, &store.AuditLogFilter{
        UserUID:   actor.UID,
        StartTime: time.Now().Add(-72 * time.Hour),
    })

    // Snapshot active sessions
    sessions, _ := r.store.GetActiveSessions(ctx, actor.UID)

    // Snapshot query history
    queries, _ := r.store.GetQueryHistory(ctx, actor.UID, 72*time.Hour)

    evidence := &store.IncidentEvidence{
        IncidentID: incident.ID,
        AuditLogs:  auditLogs,
        Sessions:   sessions,
        Queries:    queries,
        CapturedAt: time.Now(),
    }

    return r.store.SaveIncidentEvidence(ctx, evidence)
}
```

### 3.5 L8 — Database Schema

```sql
CREATE TABLE incident (
    id          TEXT PRIMARY KEY,
    severity    TEXT NOT NULL,
    category    TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open',
    assignee    TEXT,
    timeline    JSONB NOT NULL DEFAULT '[]',
    trigger_event JSONB NOT NULL,
    created_ts  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE incident_evidence (
    id          BIGSERIAL PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incident(id),
    evidence    JSONB NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE playbook (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    trigger     JSONB NOT NULL,
    actions     JSONB NOT NULL,
    mode        TEXT NOT NULL DEFAULT 'auto',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_incident_status ON incident (status, created_ts DESC);
CREATE INDEX idx_incident_severity ON incident (severity) WHERE status = 'open';
```

---

## 4. Pre-Built Playbooks

| Playbook | Trigger | Actions |
|----------|---------|---------|
| Account Compromise | impossible_travel + data_access | lock_account → revoke_sessions → preserve_evidence → notify_soc |
| Credential Leak | api_key_leaked | rotate_credential → block_old_key → notify_admin |
| Brute Force | >50 failed_login/5min | block_ip → lock_account → notify_soc |
| Data Exfiltration | bulk_export from prod | block_export → freeze_account → preserve_logs → notify_soc |
| Unauthorized Schema | ddl_without_approval | freeze_pipeline → notify_admin → preserve_evidence |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-010 (SIEM) | Security events trigger incidents |
| CR-SEC-001 (Session) | Session revocation in playbooks |
| CR-SEC-003 (Brute-Force) | Brute-force events trigger playbooks |
| CR-SEC-011 (Tamper-Proof) | Evidence stored with integrity protection |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Incident engine + classification | Sprint 1 |
| 2 | Pre-built playbooks (auto mode) | Sprint 2 |
| 3 | Escalation runner + SLA monitoring | Sprint 3 |
| 4 | Forensic preservation + incident UI | Sprint 4 |
