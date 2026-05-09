# ARCH-WEAK-005 — Service Layer Bloat (L4)

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Weakness (Needs Fix)                       |
| **Layer**      | L4 (Service)                               |
| **Impact**     | Maintainability, Code Review, Merge Conflicts |
| **Severity**   | Medium                                     |

---

## 1. Description

Service layer (`backend/api/v1/`) chứa **79 files, 36,812 lines** trong cùng một Go package. Nhiều service files vượt quá 1,500 lines, kết hợp nhiều business domains trong cùng file.

### Top Offenders (verified)

| File | Lines | Concerns Mixed |
|------|-------|----------------|
| `auth_service.go` | 1,930 | Login + Signup + MFA + SSO + Password + Email + Token |
| `sql_service.go` | 1,876 | Query + Check + AI + Export + Conversion |
| `document_masking.go` | 1,385 | MongoDB + CosmosDB + Elasticsearch masking |
| `rollout_service.go` | 1,278 | CRUD + Task creation + Execution |
| `user_service.go` | 948 | CRUD + Groups + Preferences + MFA |

### Package-Level Metrics

```
Total files in backend/api/v1/:   79
Total lines of code:              36,812
Average file size:                466 lines
Files > 1,000 lines:              6
Files > 500 lines:                18
```

---

## 2. Consequences

| Consequence | Description |
|------------|-------------|
| **Merge Conflicts** | 2+ developers editing same 1,900-line file → conflict guaranteed |
| **Code Review** | Reviewing a PR touching auth_service.go requires understanding ALL auth flows |
| **IDE Performance** | Large files slow down LSP, autocomplete, and go-to-definition |
| **Cognitive Load** | New developer must scan 1,930 lines to find password reset logic |
| **SRP Violation** | Login, MFA, SSO, email config all in one struct → Single Responsibility violated |

---

## 3. Root Cause

Go's package system encourages all files in same package to avoid import cycles. As ConnectRPC service interfaces grew, methods were added to existing service structs rather than split into sub-domains. The pattern is:

```go
// ConnectRPC generates interface with ALL methods
type AuthServiceHandler interface {
    Login(...)
    Signup(...)
    SetupMFA(...)
    ResetPassword(...)
    ExchangeToken(...)
    // ... 30+ methods
}

// Single struct implements all methods → single file grows unbounded
type AuthService struct { ... }
func (s *AuthService) Login(...)          { /* 100 lines */ }
func (s *AuthService) Signup(...)         { /* 200 lines */ }
func (s *AuthService) SetupMFA(...)       { /* 150 lines */ }
// ... all in auth_service.go → 1,930 lines
```

### Fix is Safe

Go allows methods on a struct to be defined across multiple files in the same package. Splitting is a **zero-impact refactor** — no API changes, no import changes, no behavior changes.

---

## 4. Measurement

| Metric | Current | Target |
|--------|---------|--------|
| Max file size | 1,930 lines | < 800 lines |
| Files > 1,000 lines | 6 | 0 |
| Average file size | 466 lines | < 400 lines |
