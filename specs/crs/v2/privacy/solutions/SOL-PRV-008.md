# Solution: CR-PRV-008 — User Activity Privacy

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-PRV-008                |
| **Solution**   | SOL-PRV-008               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Implement **user-scoped activity controls** bằng cách enhance existing store queries (L8) với visibility filtering, thêm **data minimization** pipeline trong SQLService (L4), và tạo **privacy settings** API mới (L4). Watermark enhancement tại L5 (`component/watermark/`) chuyển từ PII-based sang pseudonym-based tokens. Giải pháp chủ yếu modify existing code, ít component mới.

---

## 2. Architectural Alignment

```
L4 Service (user_privacy_service.go — NEW)
  │  Privacy settings CRUD
  ▼
L8 Store (user_setting) — existing settings table, new privacy keys
  │
  ├─── L4 Service (sql_service.go) — data minimization in query storage
  ├─── L4 Service (activity_service.go / existing queries) — scoped visibility
  ├─── L5 Component (watermark/) — pseudonymized watermark content
  └─── L8 Store (query_history, activity) — enhanced with privacy filters
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `api/v1/user_privacy_service.go` | Privacy settings CRUD (NEW) |
| **L4 — Service** | `api/v1/sql_service.go` (77KB) | Data minimization for query text storage |
| **L5 — Component** | `component/watermark/` | Pseudonymized watermark content |
| **L8 — Store** | `store/activity.go` | Scoped activity queries |
| **L8 — Store** | `store/query_history.go` | User-scoped query history |
| **L8 — Store** | `store/user_setting.go` | Privacy preferences storage (existing) |
| **L9 — Enterprise** | `feature.go` | `FeatureUserPrivacy` gate |

---

## 3. Chi tiết Implementation

### 3.1 L8 — User-Scoped Activity Queries

**File**: `backend/store/activity.go` (modify existing)

```go
// Enhance existing ListActivities with privacy-aware filtering
func (s *Store) ListActivities(ctx context.Context, find *FindActivityMessage) ([]*ActivityMessage, error) {
    // Existing query builder...
    
    // NEW: Apply visibility scoping based on requester
    requester := find.RequesterUID
    if !find.IsAdmin {
        // Non-admin users can only see their own activities
        where = append(where, "activity.creator_uid = $N")
        args = append(args, requester)
    } else if find.RequireJustification {
        // Admin accessing other user's data — requires justification
        // Logged as meta-audit (SOL-PRV-005)
        if find.Justification == "" {
            return nil, errors.New("justification required to access other user's activity")
        }
    }
    
    // Aggregated view for management dashboards
    if find.AggregatedOnly {
        // Return counts/trends, not individual records
        return s.listAggregatedActivities(ctx, find)
    }
    
    // ... existing query execution ...
}
```

### 3.2 L8 — Scoped Query History

**File**: `backend/store/query_history.go` (modify existing)

```go
func (s *Store) ListQueryHistories(ctx context.Context, 
    find *FindQueryHistoryMessage) ([]*QueryHistoryMessage, error) {
    
    // Existing logic...
    
    // NEW: Enforce user scoping
    // Default: users can ONLY see their own query history
    if !find.IsPrivilegedAccess {
        where = append(where, "query_history.creator_uid = $N")
        args = append(args, find.RequesterUID)
    }
    
    // Apply retention preference
    userSetting, _ := s.GetUserPrivacySetting(ctx, find.RequesterUID)
    if userSetting.QueryHistoryRetentionDays > 0 {
        cutoff := time.Now().AddDate(0, 0, -userSetting.QueryHistoryRetentionDays)
        where = append(where, "query_history.created_ts > $N")
        args = append(args, cutoff)
    }
    
    // ... existing query execution ...
}
```

### 3.3 L4 — Data Minimization

**File**: `backend/api/v1/sql_service.go` (modify existing)

```go
// When storing query to history, apply minimization
func (s *SQLService) storeQueryHistory(ctx context.Context, 
    stmt string, database string, result *v1pb.QueryResult) {
    
    // Check if privacy feature enabled
    if s.licenseService.IsFeatureEnabled(ctx, FeatureUserPrivacy) {
        privacySetting, _ := s.store.GetUserPrivacySetting(ctx, user.UID)
        
        if privacySetting.MinimizeQueryText {
            // Store query structure + hash, not full literal values
            redacted, hash := s.redactor.Redact(engine, stmt, RedactionLiteralsOnly)
            stmt = redacted
            // Hash stored for forensic correlation (if needed)
            _ = hash
        }
        
        // Minimize result metadata
        // Store: row count, column count, execution time
        // Do NOT store: IP geolocation, user agent (by default)
    }
    
    s.store.CreateQueryHistory(ctx, &store.QueryHistoryMessage{
        CreatorUID: user.UID,
        Statement:  stmt,
        Database:   database,
        RowCount:   len(result.Rows),
        // ... existing fields ...
    })
}
```

### 3.4 L5 — Pseudonymized Watermark

**File**: `backend/component/watermark/privacy.go` (new, extends existing watermark/)

```go
type PrivacyWatermark struct {
    pseudonym *privacy.PseudonymEngine
}

// Current: watermark shows "john.doe@company.com — 2026-05-13 10:30"
// Enhanced: watermark shows "USR_a7f3b2c1 — 2026-05-13 10:30"
func (w *PrivacyWatermark) Generate(ctx context.Context, user *store.UserMessage, 
    settings *WatermarkSettings) string {
    
    switch settings.ContentMode {
    case WatermarkContentToken:
        // Pseudonymized token (traceable by admin with audit)
        token, _ := w.pseudonym.Pseudonymize(ctx, user.Email)
        return fmt.Sprintf("%s — %s", token[:12], time.Now().Format("2006-01-02 15:04"))
        
    case WatermarkContentInitials:
        // User initials only
        initials := extractInitials(user.Name) // "ND" for "Nguyen Duc"
        return fmt.Sprintf("%s — %s", initials, time.Now().Format("2006-01-02 15:04"))
        
    case WatermarkContentDepartment:
        return fmt.Sprintf("%s — %s", user.Department, time.Now().Format("2006-01-02 15:04"))
        
    default: // Legacy mode (email-based, for backward compatibility)
        return fmt.Sprintf("%s — %s", user.Email, time.Now().Format("2006-01-02 15:04"))
    }
}
```

### 3.5 L4 — User Privacy Settings Service

**File**: `backend/api/v1/user_privacy_service.go`

```go
type UserPrivacyService struct {
    store *store.Store
}

// Privacy settings stored in existing user_setting table with new keys
type PrivacySetting struct {
    QueryHistoryRetentionDays int    // 1, 7, 30, 90 (default: 90)
    TelemetryOptOut           bool   // default: false
    ActivityVisibility        string // PRIVATE, TEAM, PROJECT (default: PRIVATE)
    LoginNotification         bool   // default: true
    MinimizeQueryText         bool   // default: true
}

func (s *UserPrivacyService) GetPrivacySettings(ctx context.Context, 
    req *v1pb.GetPrivacySettingsRequest) (*v1pb.PrivacySettings, error) {
    
    user := extractUser(ctx)
    // Users can only read/write their own privacy settings
    settings, _ := s.store.GetUserSetting(ctx, user.UID, "privacy")
    return convertPrivacySettings(settings), nil
}

func (s *UserPrivacyService) UpdatePrivacySettings(ctx context.Context,
    req *v1pb.UpdatePrivacySettingsRequest) (*v1pb.PrivacySettings, error) {
    
    user := extractUser(ctx)
    // Validate setting values
    if req.QueryHistoryRetentionDays < 1 || req.QueryHistoryRetentionDays > 365 {
        return nil, status.Errorf(codes.InvalidArgument, "retention must be 1-365 days")
    }
    
    settings := &store.UserSettingMessage{
        UserUID: user.UID,
        Key:     "privacy",
        Value:   marshalPrivacySettings(req),
    }
    s.store.UpsertUserSetting(ctx, settings)
    
    return convertPrivacySettings(settings), nil
}

func (s *UserPrivacyService) ExportMyData(ctx context.Context,
    req *v1pb.ExportMyDataRequest) (*v1pb.ExportMyDataResponse, error) {
    
    user := extractUser(ctx)
    // GDPR Article 20: Right to Data Portability
    // Export all user's own data in machine-readable format
    data := &UserDataExport{
        Profile:      s.store.GetUser(ctx, user.UID),
        QueryHistory: s.store.ListQueryHistories(ctx, &FindQueryHistoryMessage{CreatorUID: user.UID}),
        Activities:   s.store.ListActivities(ctx, &FindActivityMessage{CreatorUID: user.UID}),
        Settings:     s.store.ListUserSettings(ctx, user.UID),
    }
    
    jsonData, _ := json.Marshal(data)
    return &v1pb.ExportMyDataResponse{Data: jsonData, Format: "JSON"}, nil
}
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Admin privacy override | Admin access requires justification + audit trail |
| Watermark token reversal | Pseudonym reverse-lookup requires `bb.privacy.reidentify` permission |
| User data export abuse | Rate limited to 1 export per day |
| Telemetry opt-out enforcement | Application-level check before any telemetry emission |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-021 (Watermark) | Pseudonymized watermark content |
| CR-PRV-005 (Privacy Audit) | Meta-audit for admin access to user data |
| CR-PRV-002 (Anonymization) | Pseudonymization for watermark tokens |
| CR-PRV-003 (DSR) | ExportMyData implements GDPR portability right |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | User-scoped activity queries + store changes | Sprint 1 |
| 2 | Data minimization in SQLService | Sprint 2 |
| 3 | Privacy-respecting watermark | Sprint 2 |
| 4 | User privacy settings UI + ExportMyData | Sprint 3 |
