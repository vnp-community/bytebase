# TASK-AI-P0-007: Update `AGENTS.md` — Thêm Checklists + Decision Trees

> **Source**: SOL-AI-006 §2.5 + SOL-AI-001 §2.1 | **Priority**: P0 | **Effort**: 2h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P0-001 thru P0-005  
> **Phase**: 0 — Quick Wins

## Scope
- **EDIT** `frontend/AGENTS.md` — append new sections

## What
Nâng cấp AGENTS.md từ "prose guide" thành "machine-actionable checklists" cho 5 common AI tasks.

## Implementation

### Append các sections sau vào cuối `frontend/AGENTS.md`

#### Section: AI Context System
```markdown
## AI Context System

Before any task, read `.ai-context/INDEX.md` to orient yourself.
For domain tasks: read the relevant `.ai-context.md` in the target module directory.
For API calls: read `.ai-context/CONNECTRPC_GUIDE.md`.
For domain objects: read `.ai-context/GLOSSARY.md`.
NEVER read `src/types/proto-es/` — use `src/types/ai-ref/` instead.
```

#### Section: Task Checklists
5 checklists (Add New Page, Add Edit Sheet, Add Dialog, Add API Call, Add Translation Key) — mỗi checklist dạng `- [ ]` items, mỗi item 1 dòng rõ ràng.

**Add New Page checklist** (7 items):
file location, named export, router registration, props.page value, glob check, permission, verify.

**Add Edit Sheet checklist** (6 items):
template copy, outer useRef freeze, inner key reset, isDirty, Update disabled until dirty, fetch full entity.

**Add API Call checklist** (5 items):
use client from CONNECTRPC_GUIDE, use `create(Schema, {...})` not `new Constructor()`, updateMask for updates, TanStack Query hook preferred, no try/catch for standard errors.

**Add Translation Key checklist** (4 items):
identify Vue vs React file, add to correct locale directory, all 5 language files, use correct hook.

**Add Dialog/Sheet checklist** (5 items):
Dialog vs Sheet decision, no raw z-index, use getLayerRoot, include accessible title, AlertDialog for destructive.

## AC
- [ ] 5 checklists appended to AGENTS.md
- [ ] Each checklist item is a single, verifiable action
- [ ] Links to .ai-context files are correct paths
- [ ] AGENTS.md still passes `pnpm check` (no syntax issues)
