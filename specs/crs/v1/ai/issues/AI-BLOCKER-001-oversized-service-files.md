# AI-BLOCKER-001: Oversized Service Files Exceed LLM Context Window

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-001 |
| **Severity** | 🔴 Critical |
| **Category** | File Density / Context Window |
| **Layer** | L4 Service (`backend/api/v1/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

Multiple API service files in `backend/api/v1/` exceed 1000+ lines, creating severe LLM context window pressure. When an AI agent needs to modify a single function, it must load the entire file, consuming 30-60% of available context tokens on a single file and leaving insufficient room for dependent types, interfaces, and test context.

## Impact on AI Operations

- **Hallucination Risk**: LLM must hold 1900 lines of auth logic simultaneously; probability of generating incorrect code increases linearly with file size.
- **Token Budget Starvation**: A single `auth_service.go` (1930 LOC) consumes ~12K tokens, leaving minimal room for the store interfaces, proto definitions, and test fixtures needed for accurate code generation.
- **Merge Conflict Amplification**: Large files attract concurrent AI edits, increasing merge failure rates.

## Evidence

| File | Lines | Concern |
|------|-------|---------|
| `auth_service.go` | 1930 | Session, MFA, OAuth2, SSO, rate limiting — at least 5 distinct domains |
| `sql_service.go` | 1876 | Query execution, masking, access control, admin streams |
| `document_masking.go` | 1385 | Complex masking rules with deep nesting |
| `rollout_service.go` | 1278 | Rollout lifecycle, task state machines |
| `project_service.go` | 1275 | Project CRUD, IAM, webhooks |
| `plan_service.go` | 1259 | Plan creation, spec management |
| `database_service.go` | 1247 | Schema sync, metadata, catalog |
| `issue_service.go` | 1242 | Issue lifecycle, comments, approvals |
| `instance_service.go` | 1181 | Instance management, activation |
| `database_converter.go` | 1095 | Proto conversion helpers |

> **Total**: 36,953 LOC across all `api/v1/*.go` files.

## Recommended Remediation

1. **Extract Domain Sub-Services**: Split `auth_service.go` into:
   - `auth_session.go` — session creation/validation
   - `auth_mfa.go` — MFA enrollment/verification
   - `auth_oauth.go` — OAuth2/SSO flows
   - `auth_ratelimit.go` — rate limiting logic

2. **Target Guideline**: Each service file should be ≤500 LOC (fits comfortably in 4K-token LLM chunk).

3. **Preserve gRPC Contract**: Extract only internal implementation; the gRPC handler method signatures stay in the main service file as thin dispatchers.

## Files to Modify

```
backend/api/v1/auth_service.go → split into 4 files
backend/api/v1/sql_service.go → split into 3 files
backend/api/v1/rollout_service.go → split into 2 files
backend/api/v1/database_service.go → split into 2 files
backend/api/v1/project_service.go → split into 2 files
```
