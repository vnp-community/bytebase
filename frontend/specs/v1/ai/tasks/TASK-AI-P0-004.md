# TASK-AI-P0-004: Tạo `.ai-context/GLOSSARY.md` + `WORKFLOWS.md`

> **Source**: SOL-AI-010 §2.1-2.2 | **Priority**: P0 | **Effort**: 3h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P0-001  
> **Phase**: 0 — Quick Wins

## Scope
- **NEW** `frontend/.ai-context/GLOSSARY.md`
- **NEW** `frontend/.ai-context/WORKFLOWS.md`

## What
Domain knowledge glossary để AI không nhầm Plan vs Issue vs Rollout, và workflow diagrams cho top-5 business flows.

## Implementation

### 1. `GLOSSARY.md`
Sections bắt buộc:
- **Core Workflow Objects** (Plan, Issue, Rollout, Stage, Task, TaskRun, Release, Sheet, Worksheet) — mỗi object: mô tả, resource name format, lifecycle, relations, file path
- **Object Hierarchy** — ASCII tree từ Project xuống TaskRun
- **IAM Model** — workspace roles, project roles, permission scopes, check patterns
- **SQL Review Rules** — categories, severity, format, source file
- **CEL Expressions** — syntax, examples, parser location
- **Database Engines** — table 10+ engines với `Engine.{NAME}` enum value
- **Data Masking** — levels, classification, exemption

### 2. `WORKFLOWS.md`
Mermaid diagrams cho:
1. Database Change Workflow (Plan → Issue → Rollout → Stage → Task → TaskRun)
2. SQL Editor Workflow (connect → write → execute → result)
3. Approval Workflow (request → review → approve/reject → execute)
4. Schema Sync Workflow (detect drift → create plan → review → apply)
5. Data Masking Workflow (classify → mask rule → exemption → query)

## AC
- [ ] GLOSSARY.md có đủ 7 sections, mỗi object rõ resource name format
- [ ] Object Hierarchy tree chính xác với code hiện tại
- [ ] 5 Mermaid diagrams render đúng trong GitHub
- [ ] Permission scope examples khớp với PermissionStore API
