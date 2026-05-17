# SOL-SHR-003 — Pipeline Sharing & Secure Links

| Metadata | Value |
|---|---|
| Solution ID | SOL-SHR-003 |
| CRs | CR-SHR-003 (Pipeline Credential Sharing), CR-SHR-103 (Secure Share Links), CR-SHR-005 (Delivery Notification) |
| Arch Layers | L4 (Service), L5 (Component), L6 (Runner), L7 (Plugin) |
| Priority | P1 — High |
| Sprints | 4–7 |
| Dependencies | SOL-SHR-001 (Provider Core), SOL-SHR-002 (Envelope Encryption) |

---

## 1. Phân tích kiến trúc hiện tại

### 1.1 TaskRun Executor (L6 — Runner)

```
taskrun/
  ├── scheduler.go                   ← Main loop
  ├── database_migrate_executor.go   ← DDL/DML migration (37KB)
  └── executor.go                    ← Executor interface
```

`DatabaseMigrateExecutor.RunOnce()` là hook point cho credential detection — thêm post-migration logic.

### 1.2 Issue & Approval (L4 — Service)

```go
// IssueService: CRUD + approval workflow
// PlanService: Plan → tasks
// RolloutService: Rollout → task execution
```

Pipeline flow: `Issue → Plan → Rollout → TaskRun → DatabaseMigrateExecutor`

### 1.3 Notification (L5 — Component + L7 — Plugin)

- `component/webhook/` — WebhookManager dispatches to IM platforms
- `plugin/webhook/` — Slack, DingTalk, Feishu, Teams formatters
- `plugin/mailer/` — SMTP email delivery
- `component/bus/` — `Bus` channels coordinate async operations

### 1.4 Echo HTTP Router (L2)

Public endpoints bypass auth interceptor — Echo middleware handles rate limiting.

---

## 2. Giải pháp chi tiết

### 2.1 Module Structure

```
backend/
├── component/sharing/
│   ├── detector.go           ← SQL credential pattern detection
│   ├── policy.go             ← Share policy enforcement
│   ├── otp.go                ← Email OTP generation/verification
│   └── notifier.go           ← Share notification coordinator
│
├── api/v1/
│   ├── sharing_service.go    ← gRPC SharingService (from SOL-SHR-001)
│   └── share_access.go       ← Public endpoint handler (Echo)
│
├── runner/
│   └── cleaner/
│       └── share_cleaner.go  ← TTL-based share cleanup
│
└── store/
    ├── share_link.go          ← share_link CRUD
    ├── share_policy.go        ← share_policy CRUD
    └── share_access_log.go    ← Access event logging
```

### 2.2 Credential Detector — SQL Pattern Matching

```go
// backend/component/sharing/detector.go
package sharing

import (
    "regexp"
    storepb "github.com/bytebase/bytebase/proto/generated-go/store"
)

type CredentialDetector struct {
    patterns map[storepb.Engine][]*regexp.Regexp
}

type DetectedCredential struct {
    Username       string
    Password       string
    Engine         storepb.Engine
    StatementIndex int
    DatabaseName   string
}

func NewCredentialDetector() *CredentialDetector {
    d := &CredentialDetector{
        patterns: make(map[storepb.Engine][]*regexp.Regexp),
    }
    
    // PostgreSQL patterns
    d.patterns[storepb.Engine_POSTGRES] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)CREATE\s+(?:ROLE|USER)\s+(\w+)\s+.*?PASSWORD\s+'([^']+)'`),
        regexp.MustCompile(`(?i)ALTER\s+(?:ROLE|USER)\s+(\w+)\s+.*?PASSWORD\s+'([^']+)'`),
    }
    
    // MySQL patterns
    d.patterns[storepb.Engine_MYSQL] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)CREATE\s+USER\s+'?(\w+)'?.*?IDENTIFIED\s+BY\s+'([^']+)'`),
        regexp.MustCompile(`(?i)ALTER\s+USER\s+'?(\w+)'?.*?IDENTIFIED\s+BY\s+'([^']+)'`),
        regexp.MustCompile(`(?i)SET\s+PASSWORD\s+FOR\s+'?(\w+)'?.*?=\s+PASSWORD\('([^']+)'\)`),
    }
    
    // Oracle patterns
    d.patterns[storepb.Engine_ORACLE] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)CREATE\s+USER\s+(\w+)\s+IDENTIFIED\s+BY\s+(\S+)`),
        regexp.MustCompile(`(?i)ALTER\s+USER\s+(\w+)\s+IDENTIFIED\s+BY\s+(\S+)`),
    }
    
    // SQL Server patterns
    d.patterns[storepb.Engine_MSSQL] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)CREATE\s+LOGIN\s+(\w+)\s+WITH\s+PASSWORD\s*=\s*'([^']+)'`),
    }
    
    return d
}

// Detect scans SQL statements for credential creation/modification.
func (d *CredentialDetector) Detect(engine storepb.Engine, statements []string) []DetectedCredential {
    var results []DetectedCredential
    patterns, ok := d.patterns[engine]
    if !ok {
        return nil
    }
    for i, stmt := range statements {
        for _, p := range patterns {
            matches := p.FindStringSubmatch(stmt)
            if len(matches) >= 3 {
                results = append(results, DetectedCredential{
                    Username:       matches[1],
                    Password:       matches[2],
                    Engine:         engine,
                    StatementIndex: i,
                })
            }
        }
    }
    return results
}
```

### 2.3 TaskRun Post-Migration Hook

```go
// backend/runner/taskrun/database_migrate_executor.go
// Added to RunOnce() after successful migration:

func (e *DatabaseMigrateExecutor) postMigrationSharing(
    ctx context.Context,
    task *store.TaskMessage,
    instance *store.InstanceMessage,
    statements []string,
) {
    if e.sharingManager == nil {
        return // Sharing not configured
    }
    
    // 1. Detect credentials in executed SQL
    creds := e.credentialDetector.Detect(instance.Engine, statements)
    if len(creds) == 0 {
        return
    }
    
    // 2. Load auto-share config
    config, err := e.store.GetSharingConfig(ctx, task.WorkspaceID)
    if err != nil || !config.AutoShareEnabled {
        return
    }
    
    // 3. Get issue → resolve recipients
    issue, _ := e.store.GetIssueV2(ctx, &store.FindIssueMessage{UID: &task.IssueUID})
    recipients := e.resolveRecipients(ctx, issue, config.RecipientPolicy)
    
    for _, cred := range creds {
        // 4. Build payload (never log the password!)
        payload, _ := json.Marshal(map[string]string{
            "username": cred.Username,
            "password": cred.Password,
            "host":     instance.Host,
            "port":     instance.Port,
            "database": task.DatabaseName,
            "engine":   instance.Engine.String(),
        })
        
        // 5. Create share via SharingManager (SOL-SHR-001)
        resp, err := e.sharingManager.CreateShare(ctx, &sharing.ShareRequest{
            Payload:        payload,
            CredentialType: sharing.CredentialTypePassword,
            Name:           fmt.Sprintf("%s@%s/%s", cred.Username, instance.Host, task.DatabaseName),
            MaxAccessCount: config.DefaultMaxAccess,
            ExpiresAt:      timePtr(time.Now().Add(config.DefaultTTL)),
            CreatorUID:     task.CreatorUID,
            RecipientUIDs:  recipients,
            ProjectID:      task.Project,
            IssueUID:       task.IssueUID,
            WorkspaceID:    task.WorkspaceID,
        })
        if err != nil {
            slog.Error("sharing: auto-share failed", "error", err, "instance", instance.ResourceID)
            continue
        }
        
        // 6. Post issue comment with access URL (mask actual credential)
        e.postShareComment(ctx, issue, cred.Username, resp)
        
        // 7. Emit notification via Bus
        e.bus.ShareEventChan <- bus.ShareEvent{
            Type:    bus.ShareEventCreated,
            ShareID: resp.ShareID,
        }
    }
}

func (e *DatabaseMigrateExecutor) postShareComment(
    ctx context.Context,
    issue *store.IssueMessage,
    username string,
    resp *sharing.ShareResponse,
) {
    comment := fmt.Sprintf(
        "🔐 **Credentials shared securely**\n\n"+
            "Username: `%s`\n"+
            "Share URL: %s\n"+
            "Expires: %s\n"+
            "Max access: %d times\n\n"+
            "⚠️ This link will expire automatically.",
        username,
        resp.AccessURL,
        resp.ExpiresAt.Format(time.RFC3339),
        resp.MaxAccessCount,
    )
    
    e.store.CreateIssueComment(ctx, &store.IssueCommentMessage{
        IssueUID: issue.UID,
        Payload: &storepb.IssueCommentPayload{
            Comment: comment,
        },
        CreatorUID: api.SystemBotID,
    })
}
```

### 2.4 Public Share Access Endpoint (CR-SHR-103)

```go
// backend/api/v1/share_access.go
// Registered on Echo router (L2) — bypasses auth interceptor.

func (s *Server) registerShareAccessRoutes(e *echo.Echo) {
    g := e.Group("/share")
    g.Use(s.rateLimitMiddleware(10, time.Minute)) // 10 req/min per IP
    g.GET("/:token", s.handleShareAccess)
    g.POST("/:token/access", s.handleShareAccessAPI)
    g.POST("/:token/otp/request", s.handleShareOTPRequest)
    g.POST("/:token/otp/verify", s.handleShareOTPVerify)
}

func (s *Server) handleShareAccessAPI(c echo.Context) error {
    token := c.Param("token")
    
    // 1. Look up share link
    link, err := s.store.GetShareLink(c.Request().Context(), token)
    if err != nil || link == nil || link.Status != "active" {
        return c.JSON(http.StatusNotFound, map[string]string{
            "error": "Share not found or expired",
        })
    }
    
    // 2. Check expiry
    if time.Now().After(link.ExpiresAt) {
        return c.JSON(http.StatusGone, map[string]string{"error": "Share expired"})
    }
    
    // 3. Check max access
    if link.MaxAccesses > 0 && link.CurrentAccesses >= link.MaxAccesses {
        return c.JSON(http.StatusGone, map[string]string{"error": "Share access limit reached"})
    }
    
    // 4. Check IP allowlist (from share_policy)
    clientIP := c.RealIP()
    if err := s.sharingManager.CheckIPAllowlist(c.Request().Context(), link, clientIP); err != nil {
        return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
    }
    
    // 5. Password verification (if required)
    var req ShareAccessRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, nil)
    }
    if link.PasswordHash != "" {
        if !verifyArgon2id(req.Password, link.PasswordHash) {
            s.logAccessAttempt(c, link, "denied_wrong_password")
            return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Wrong password"})
        }
    }
    
    // 6. Email OTP check (if required)
    if len(link.AllowedEmails) > 0 && !req.OTPVerified {
        return c.JSON(http.StatusUnauthorized, map[string]string{
            "error": "Email verification required",
            "requires_otp": "true",
        })
    }
    
    // 7. Decrypt and return content
    var content []byte
    if link.VaultwardenSendID != "" {
        // Delegate to Vaultwarden
        content, err = s.sharingManager.AccessVaultwardenSend(c.Request().Context(), link)
    } else {
        // Native fallback: decrypt BEE envelope
        env, _ := envelope.UnmarshalEnvelope(link.EncryptedEnvelope)
        content, err = s.encryptor.Open(c.Request().Context(), env)
    }
    if err != nil {
        return c.JSON(http.StatusInternalServerError, nil)
    }
    
    // 8. Increment access count
    s.store.IncrementShareAccess(c.Request().Context(), link.ID)
    
    // 9. Log access
    s.logAccessAttempt(c, link, "success")
    
    // 10. Set no-cache headers
    c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
    c.Response().Header().Set("X-Robots-Tag", "noindex, nofollow")
    
    return c.JSON(http.StatusOK, map[string]interface{}{
        "content":        json.RawMessage(content),
        "remaining_access": link.MaxAccesses - link.CurrentAccesses - 1,
    })
}
```

### 2.5 Email OTP Service

```go
// backend/component/sharing/otp.go
package sharing

import (
    "crypto/rand"
    "fmt"
    "math/big"
    "time"
    
    lru "github.com/hashicorp/golang-lru/v2/expirable"
)

type OTPService struct {
    // In-memory store with TTL (pattern from TDD Section 4.2)
    store  *lru.LRU[string, *OTPEntry]
    mailer plugin.Mailer
}

type OTPEntry struct {
    Code      string
    Email     string
    ShareID   string
    Attempts  int
    CreatedAt time.Time
}

func NewOTPService(mailer plugin.Mailer) *OTPService {
    return &OTPService{
        store: lru.NewLRU[string, *OTPEntry](1024, nil, 5*time.Minute),
        mailer: mailer,
    }
}

// GenerateAndSend creates a 6-digit OTP and sends via email.
func (s *OTPService) GenerateAndSend(ctx context.Context, email, shareID string) error {
    // Rate limit: max 3 OTPs per share per 15 min
    key := fmt.Sprintf("%s:%s", shareID, email)
    if entry, ok := s.store.Get(key); ok && entry.Attempts >= 3 {
        return fmt.Errorf("otp: rate limit exceeded")
    }
    
    // Generate 6-digit code
    code := generateSecureCode(6)
    
    s.store.Add(key, &OTPEntry{
        Code:      code,
        Email:     email,
        ShareID:   shareID,
        CreatedAt: time.Now(),
    })
    
    // Send via Mailer plugin (L7)
    return s.mailer.Send(ctx, &mailer.Message{
        To:      []string{email},
        Subject: "Bytebase — Your verification code",
        Body:    fmt.Sprintf("Your verification code is: %s\nThis code expires in 5 minutes.", code),
    })
}

// Verify checks OTP validity (single-use).
func (s *OTPService) Verify(shareID, email, code string) bool {
    key := fmt.Sprintf("%s:%s", shareID, email)
    entry, ok := s.store.Get(key)
    if !ok {
        return false
    }
    entry.Attempts++
    if entry.Attempts > 5 {
        s.store.Remove(key) // Lock out
        return false
    }
    if entry.Code == code {
        s.store.Remove(key) // Single-use
        return true
    }
    return false
}

func generateSecureCode(digits int) string {
    max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
    n, _ := rand.Int(rand.Reader, max)
    return fmt.Sprintf("%0*d", digits, n)
}
```

### 2.6 Share Policy Engine

```go
// backend/component/sharing/policy.go
package sharing

// SharePolicyEngine validates share creation against workspace/project policies.
type SharePolicyEngine struct {
    store *store.Store
}

// Validate checks the request against applicable policies.
func (e *SharePolicyEngine) Validate(ctx context.Context, req *ShareRequest) error {
    // 1. Load workspace-level policy
    wsPolicy, _ := e.store.GetSharePolicy(ctx, req.WorkspaceID, "")
    
    // 2. Load project-level policy (stricter overrides)
    projPolicy, _ := e.store.GetSharePolicy(ctx, req.WorkspaceID, req.ProjectID)
    
    policy := e.merge(wsPolicy, projPolicy)
    
    // 3. Validate TTL
    if policy.MaxTTLSeconds > 0 {
        requestedTTL := time.Until(*req.ExpiresAt)
        if requestedTTL > time.Duration(policy.MaxTTLSeconds)*time.Second {
            return fmt.Errorf("share TTL %v exceeds maximum allowed %v", requestedTTL, policy.MaxTTLSeconds)
        }
    }
    
    // 4. Validate password requirement
    if policy.RequirePassword && req.Password == "" {
        return fmt.Errorf("share policy requires password protection")
    }
    
    // 5. Validate email restriction
    if policy.RequireEmailRestriction && len(req.AllowedEmails) == 0 {
        return fmt.Errorf("share policy requires email restriction")
    }
    
    // 6. Check active share limit per user
    activeCount, _ := e.store.CountActiveSharesByUser(ctx, req.CreatorUID)
    if policy.MaxActivePerUser > 0 && activeCount >= policy.MaxActivePerUser {
        return fmt.Errorf("active share limit reached (%d/%d)", activeCount, policy.MaxActivePerUser)
    }
    
    return nil
}
```

### 2.7 Notification Coordinator (CR-SHR-005)

```go
// backend/component/sharing/notifier.go
package sharing

// ShareNotifier sends share access URLs via multiple channels.
// Integrates with existing WebhookManager (L5) and Mailer (L7).
type ShareNotifier struct {
    webhookManager *webhook.Manager // Existing (L5)
    mailer         plugin.Mailer    // Existing (L7)
    store          *store.Store
}

// NotifyShareCreated dispatches notifications via configured channels.
func (n *ShareNotifier) NotifyShareCreated(ctx context.Context, share *ShareResponse, req *ShareRequest) error {
    // 1. In-app: Issue comment (always — if issue exists)
    if req.IssueUID > 0 {
        // Already handled in postShareComment (see 2.3)
    }
    
    // 2. Email notification
    for _, recipientUID := range req.RecipientUIDs {
        user, _ := n.store.GetUserByID(ctx, recipientUID)
        if user.Email != "" {
            n.mailer.Send(ctx, &mailer.Message{
                To:      []string{user.Email},
                Subject: fmt.Sprintf("Bytebase — Shared credentials: %s", req.Name),
                Body:    n.formatEmailBody(share, req),
            })
        }
    }
    
    // 3. IM notification via WebhookManager (Slack, DingTalk, etc.)
    n.webhookManager.PostShareNotification(ctx, &webhook.ShareNotificationPayload{
        ShareName: req.Name,
        AccessURL: share.AccessURL,
        ExpiresAt: share.ExpiresAt,
        Project:   req.ProjectID,
    })
    
    return nil
}

// Split Delivery: URL via one channel, decryption key via another.
func (n *ShareNotifier) SplitDelivery(ctx context.Context, share *ShareResponse, req *ShareRequest) error {
    // Channel 1 (IM): Access URL only
    n.webhookManager.PostMessage(ctx, fmt.Sprintf(
        "🔐 Credentials for %s: %s", req.Name, share.AccessURL,
    ))
    
    // Channel 2 (Email): Decryption hint/key
    // (Only relevant when using additional password beyond provider encryption)
    if share.DecryptionHint != "" {
        for _, uid := range req.RecipientUIDs {
            user, _ := n.store.GetUserByID(ctx, uid)
            n.mailer.Send(ctx, &mailer.Message{
                To:      []string{user.Email},
                Subject: "Bytebase — Decryption key for shared credentials",
                Body:    fmt.Sprintf("Key: %s\nUse this with the share link sent via IM.", share.DecryptionHint),
            })
        }
    }
    return nil
}
```

### 2.8 Share Cleaner Runner (L6)

```go
// backend/runner/cleaner/share_cleaner.go
// Extension of existing DataCleaner runner pattern.

type ShareCleaner struct {
    store          *store.Store
    sharingManager *sharing.SharingManager
    interval       time.Duration // Every 15 minutes (same as DataCleaner)
}

func (c *ShareCleaner) Run(ctx context.Context) {
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.cleanExpiredShares(ctx)
        }
    }
}

func (c *ShareCleaner) cleanExpiredShares(ctx context.Context) {
    // 1. Find expired but still ACTIVE shares
    expired, _ := c.store.ListExpiredShares(ctx)
    for _, share := range expired {
        // 2. Revoke on provider side
        c.sharingManager.RevokeShare(ctx, share.ProviderShareID)
        // 3. Update status
        c.store.UpdateShareStatus(ctx, share.ID, "EXPIRED")
        // 4. Audit event
        c.store.CreateSharingAuditLog(ctx, &store.SharingAuditLogMessage{
            EventType: "SHARE_EXPIRED",
            ShareID:   share.ProviderShareID,
        })
    }
}
```

### 2.9 Database Migration

```sql
-- Share links (native fallback storage)
CREATE TABLE share_link (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    creator_id INT NOT NULL REFERENCES principal(id),
    content_type TEXT NOT NULL,
    encrypted_envelope JSONB NOT NULL,
    vaultwarden_send_id TEXT,
    password_hash TEXT,
    allowed_emails TEXT[],
    max_accesses INT,
    current_accesses INT DEFAULT 0,
    ttl_seconds INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    source_project TEXT,
    source_issue_uid BIGINT,
    status TEXT NOT NULL DEFAULT 'active'
);

CREATE INDEX idx_share_link_status ON share_link(status, expires_at)
    WHERE status = 'active';

-- Share policies
CREATE TABLE share_policy (
    id SERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    project_id TEXT,
    max_ttl_seconds INT DEFAULT 604800,
    require_password BOOLEAN DEFAULT FALSE,
    require_email_restriction BOOLEAN DEFAULT FALSE,
    max_active_per_user INT DEFAULT 10,
    ip_allowlist INET[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, COALESCE(project_id, ''))
);

-- Share access log
CREATE TABLE share_access_log (
    id BIGSERIAL PRIMARY KEY,
    share_link_id TEXT NOT NULL,
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accessor_email TEXT,
    accessor_ip INET NOT NULL,
    accessor_user_agent TEXT,
    accessor_user_id INT REFERENCES principal(id),
    access_result TEXT NOT NULL
);

CREATE INDEX idx_share_access_link ON share_access_log(share_link_id, accessed_at DESC);
```

---

## 3. Echo Router Registration

```go
// backend/server/server.go — in configureEchoRouters()
func (s *Server) configureEchoRouters(e *echo.Echo) {
    // ... existing routes ...
    
    // Public share access (no auth interceptor)
    s.registerShareAccessRoutes(e)
}
```

---

## 4. Test Strategy

| Test | CRs | Method |
|---|---|---|
| Credential detection (PG, MySQL, Oracle, MSSQL) | CR-SHR-003 | Unit: regex + sample SQL |
| Auto-share post-migration | CR-SHR-003 | Integration: mock executor + mock provider |
| Public endpoint access flow | CR-SHR-103 | HTTP test: token → content |
| OTP generation + verification | CR-SHR-103 | Unit: timing, rate limit, single-use |
| Share policy enforcement | CR-SHR-103 | Unit: TTL, password, IP rules |
| Native fallback (no Vaultwarden) | CR-SHR-103 | Integration: BEE encrypt → store → decrypt |
| Notification dispatch | CR-SHR-005 | Mock webhook + mailer |
| Share expiry cleanup | All | Runner test: expired shares cleaned |
