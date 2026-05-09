# ARCH-WEAK-001 — No Interface Contracts in Core Layers

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Weakness (Needs Fix)                       |
| **Layer**      | L4 (Service), L5 (Component), L8 (Store)   |
| **Impact**     | Testability, Decoupling                    |
| **Severity**   | High                                       |

---

## 1. Description

Core layers (L4-L8) depend on **concrete types** exclusively. No interface definitions exist for Store, IAM Manager, or License Service. This prevents unit testing, makes dependency injection impossible, and violates Dependency Inversion Principle.

### Evidence

```go
// ZERO interfaces found in:
// - backend/store/     → no interfaces.go
// - backend/component/iam/  → no interfaces.go  
// - backend/enterprise/     → no interfaces.go

// Only L7 (Plugins) has proper interfaces:
// - plugin/db/driver.go     → db.Driver interface ✓
// - plugin/advisor/advisor.go → Advisor interface ✓
// - plugin/mailer/mail.go   → Mailer interface ✓
```

### Coupling Metrics

| Layer | Concrete Dependencies | Interface Dependencies |
|-------|----------------------|----------------------|
| L4 (Service) | `*store.Store`, `*iam.Manager`, `*enterprise.LicenseService` | 0 |
| L5 (Component) | `*store.Store`, `*enterprise.LicenseService` | 0 |
| L6 (Runner) | `*store.Store`, `*bus.Bus`, `*enterprise.LicenseService` | 0 |
| L7 (Plugin) | — | `db.Driver`, `Advisor`, `Mailer` ✓ |

**L7 is the only layer following interface-based design** — the rest use concrete types.

---

## 2. Impact Analysis

### Test File Statistics (verified)

```
Source files (non-test, non-generated): 905
Test files:                             353
Test ratio:                             39%    ← appears OK...

BUT:
  - 353 test files are 90%+ in plugin/ (parser, schema, advisor)
  - backend/api/v1/ (36,812 lines): ~3 test files
  - backend/store/ (74 files): changelog_test.go = 14 bytes (EMPTY)
  - backend/component/bus/: 0 test files
  - backend/runner/: 0 unit test files (only integration)
```

**Root cause**: Without interfaces, the only way to test services is via integration tests with real database (testcontainers) → slow, flaky, CI-dependent.

---

## 3. Comparison: L7 (Good) vs L4-L8 (Bad)

```go
// L7 — GOOD: Interface-based design
type Driver interface {
    Open(ctx, dbType, config) (Driver, error)
    Execute(ctx, statement, opts) (int64, error)
    QueryConn(ctx, conn, statement, queryContext) ([]*QueryResult, error)
}
// → 23 implementations, all independently testable

// L4 — BAD: Concrete-type coupling
type AuthService struct {
    store          *store.Store        // 200+ methods accessible
    licenseService *enterprise.LicenseService  // concrete
    iamManager     *iam.Manager        // concrete
}
// → Cannot test Login() without real DB, license key, IAM setup
```
