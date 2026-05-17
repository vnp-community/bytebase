# Bytebase Frontend — AI Context Index

> Read this file FIRST before any task. Then read the module-specific `.ai-context.md` in the target directory.

---

## Decision Tree

**"I need to add a new page"**
→ Read [NEW_PAGE_PLAYBOOK.md](./NEW_PAGE_PLAYBOOK.md)
→ Copy template from `src/react/templates/new-page.tsx`

**"I need to call an API"**
→ Read [CONNECTRPC_GUIDE.md](./CONNECTRPC_GUIDE.md)
→ Find the correct service client in the lookup table
→ DO NOT use `fetch()` — always use ConnectRPC clients

**"I need to manage state"**
→ Read [STATE_GUIDE.md](./STATE_GUIDE.md)
→ Server data = TanStack Query hook / Pinia store via `useVueState`
→ Client state = Zustand store

**"I need to understand a domain object (Plan, Issue, Rollout, etc.)"**
→ Read [GLOSSARY.md](./GLOSSARY.md)
→ Read [WORKFLOWS.md](./WORKFLOWS.md) for lifecycle diagrams

**"I need to fix a bug in a component"**
→ Check the module-level `.ai-context.md` in the component's directory first
→ Read [BRIDGE_CONTRACT.md](./BRIDGE_CONTRACT.md) if it involves Vue↔React

**"I need to add/edit an overlay (Dialog, Sheet, Drawer)"**
→ Read `AGENTS.md` § "Overlay layering policy" + § "Dialog vs Sheet"
→ Use `getLayerRoot("overlay")` — NEVER portal to `document.body`

**"I need to add a translation key"**
→ Vue file → `src/locales/{lang}.json` + `useI18n()`
→ React file → `src/react/locales/{lang}/` + `useTranslation()`

---

## Module Quick Reference

| Module | Entry Path | File Count | Framework | Key Stores |
|---|---|---|---|---|
| Settings | `src/react/pages/settings/` | 53 .tsx | React | authStore, settingV1Store, subscriptionStore |
| Project | `src/react/pages/project/` | 117 .tsx | React | projectV1Store, issueStore, planStore |
| Auth | `src/react/pages/auth/` | 18 .tsx | React | authStore |
| Workspace | `src/react/pages/workspace/` | 3 .tsx | React | — |
| SQL Editor | `src/views/sql-editor/` + `src/react/components/sql-editor/` | ~80 | Vue + React | sqlEditorStore, worksheetStore |
| Shared Components | `src/react/components/` | ~70 .tsx | React | — |
| Store (Legacy) | `src/store/modules/v1/` | 33 .ts | Vue (Pinia) | — |
| ConnectRPC Clients | `src/connect/index.ts` | 1 file (182 LOC) | — | 31 service clients |
| Router | `src/router/` | 13 files | Vue Router | — |

---

## Files to NEVER Read Directly

These files are auto-generated and will waste context window budget:

| Path | LOC | Use Instead |
|---|---|---|
| `src/types/proto-es/**` | ~38,000 | `src/types/ai-ref/` (when created) |
| `src/plugins/agent/logic/tools/gen/openapi-index.ts` | ~15,000 | `CONNECTRPC_GUIDE.md` |
| `src/react/plugins/agent/logic/tools/gen/openapi-index.ts` | ~14,000 | `CONNECTRPC_GUIDE.md` |
| `pnpm-lock.yaml` | ~30,000 | `package.json` |

---

## Context File Index

| File | Purpose |
|---|---|
| [FRAMEWORK_MAP.md](./FRAMEWORK_MAP.md) | Vue vs React file ownership rules |
| [STATE_GUIDE.md](./STATE_GUIDE.md) | When to use Pinia / Zustand / useVueState |
| [BRIDGE_CONTRACT.md](./BRIDGE_CONTRACT.md) | Vue↔React bridge mechanics |
| [CONNECTRPC_GUIDE.md](./CONNECTRPC_GUIDE.md) | API client lookup table + patterns |
| [ERROR_POLICY.md](./ERROR_POLICY.md) | When to catch / not catch errors |
| [GLOSSARY.md](./GLOSSARY.md) | Domain object definitions |
| [WORKFLOWS.md](./WORKFLOWS.md) | Business workflow diagrams |
| [NEW_PAGE_PLAYBOOK.md](./NEW_PAGE_PLAYBOOK.md) | Step-by-step add-page guide |
| [GUARD_FLOWCHART.md](./GUARD_FLOWCHART.md) | Navigation guard decision flow |

---

## Quick Rules Reminder

1. **All new code → React** (`.tsx`). No new `.vue` files.
2. **Named exports only** — `export function MembersPage()` NOT `export default`.
3. **Semantic color tokens** — `bg-accent` NOT `bg-blue-500`.
4. **`gap-*`** NOT `space-x-*` / `space-y-*`.
5. **`cn()`** for conditional classes, never manual ternaries.
6. **No raw z-index** in overlay code — use overlay layering policy.
7. **updateMask required** on all ConnectRPC update calls.
8. **`create(Schema, {...})`** NOT `new Constructor()` for Proto-ES types.
