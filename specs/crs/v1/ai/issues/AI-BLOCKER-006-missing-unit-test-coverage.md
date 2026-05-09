# AI-BLOCKER-006: Missing Unit Test Coverage for Service Layer

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-006 |
| **Severity** | 🟠 High |
| **Category** | Test Infrastructure / AI Safety Net |
| **Layer** | L4 Service (`backend/api/v1/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

The `backend/api/v1/` directory contains 109 test functions across ~356 total test files in the backend. However, the service layer tests are concentrated in:
- `catalog_masking_cosmosdb_test.go` (648 LOC)
- `catalog_masking_test.go` (530 LOC)
- `project_service_test.go` (496 LOC)

Core services like `auth_service.go`, `sql_service.go`, `rollout_service.go`, and `database_service.go` have **zero unit tests**. The `store/mock/mock_store.go` file has not been generated (0 bytes), making mock-based testing impossible.

## Impact on AI Operations

- **No Regression Safety Net**: When AI refactors `auth_service.go` (1930 LOC), there are no tests to validate the change. AI must rely entirely on compilation success, which doesn't catch logic errors.
- **Cannot Verify AI-Generated Code**: AI tools that generate code with accompanying tests cannot produce valid tests because mock infrastructure doesn't exist.
- **Confidence Collapse**: Without tests, AI agents must adopt ultra-conservative strategies (smaller changes, more manual review), reducing throughput by 3-5x.

## Evidence

```bash
# Mock file exists but has NOT been generated:
$ ls -la backend/store/mock/mock_store.go
# File does not exist (0 bytes / missing)

# Test coverage for critical services:
$ grep -c "func Test" backend/api/v1/auth_service_test.go
# No such file

$ grep -c "func Test" backend/api/v1/sql_service_test.go
# No such file

$ grep -c "func Test" backend/api/v1/rollout_service_test.go
# No such file
```

## Recommended Remediation

1. **Generate Mocks Immediately**:
   ```bash
   go install go.uber.org/mock/mockgen@latest
   go generate ./backend/store/mock/...
   ```

2. **Scaffold Test Files**: Create test files for the top 5 services:
   - `auth_service_test.go` — test session creation, MFA, rate limiting
   - `sql_service_test.go` — test query execution, masking
   - `rollout_service_test.go` — test state transitions
   - `database_service_test.go` — test schema sync
   - `plan_service_test.go` — test plan creation

3. **AI Test Template**: Establish a standard test template that AI agents can follow:
   ```go
   func TestAuthService_Login(t *testing.T) {
       ctrl := gomock.NewController(t)
       defer ctrl.Finish()
       
       mockUsers := mock.NewMockUserStore(ctrl)
       mockUsers.EXPECT().GetUser(gomock.Any(), gomock.Any()).Return(&store.UserMessage{...}, nil)
       
       svc := NewAuthService(mockUsers, ...)
       // ... test logic
   }
   ```

## Files to Modify

```
backend/store/mock/mock_store.go → generate from interfaces
NEW: backend/api/v1/auth_service_test.go
NEW: backend/api/v1/sql_service_test.go
NEW: backend/api/v1/rollout_service_test.go
NEW: backend/api/v1/database_service_test.go
```

## Dependencies

- Depends on: AI-BLOCKER-002 (interface-based DI enables mock generation)
