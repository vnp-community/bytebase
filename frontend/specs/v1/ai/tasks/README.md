# Bytebase Frontend — AI Development Tasks Index

> **Version**: 1.0.0 | **Date**: 2026-05-14  
> **Source**: `specs/v1/ai/solutions/` | **Coverage**: 100% (10/10 solutions)

---

## Task Registry

### Phase 0 — Quick Wins (Week 1–2) `~14h total`
> Không thay đổi code — chỉ tạo tài liệu + markers. AI accuracy +30% ngay lập tức.

| Task | What | Effort | Deps |
|---|---|---|---|
| [TASK-AI-P0-001](./TASK-AI-P0-001.md) | `.ai-context/INDEX.md` + `FRAMEWORK_MAP.md` | 2h | — |
| [TASK-AI-P0-002](./TASK-AI-P0-002.md) | `STATE_GUIDE.md` + `BRIDGE_CONTRACT.md` | 2h | P0-001 |
| [TASK-AI-P0-003](./TASK-AI-P0-003.md) | `CONNECTRPC_GUIDE.md` + `ERROR_POLICY.md` | 2h | P0-001 |
| [TASK-AI-P0-004](./TASK-AI-P0-004.md) | `GLOSSARY.md` + `WORKFLOWS.md` (domain knowledge) | 3h | P0-001 |
| [TASK-AI-P0-005](./TASK-AI-P0-005.md) | `NEW_PAGE_PLAYBOOK.md` + `GUARD_FLOWCHART.md` | 2h | P0-001 |
| [TASK-AI-P0-006](./TASK-AI-P0-006.md) | `.aiignore` + `@ai-exclude` markers (88 files) | 1h | — |
| [TASK-AI-P0-007](./TASK-AI-P0-007.md) | `AGENTS.md` checklists + decision trees | 2h | P0-001–005 |

### Phase 1 — Tooling & Lint (Week 2–4) `~18h total`
> Guardrails tự động ngăn AI violations. -60% convention errors.

| Task | What | Effort | Deps |
|---|---|---|---|
| [TASK-AI-P1-001](./TASK-AI-P1-001.md) | `src/types/ai-ref/` — service-map + 3 core types | 4h | — |
| [TASK-AI-P1-002](./TASK-AI-P1-002.md) | `src/types/ai-ref/` — 7 remaining domain types | 3h | P1-001 |
| [TASK-AI-P1-003](./TASK-AI-P1-003.md) | `src/react/templates/` — 7 scaffold templates | 3h | — |
| [TASK-AI-P1-004](./TASK-AI-P1-004.md) | ESLint: `no-proto-constructor` + `no-fetch-for-grpc` + `require-update-mask` | 3h | — |
| [TASK-AI-P1-005](./TASK-AI-P1-005.md) | ESLint: `react-page-named-export` + `correct-i18n-system` + semantic token check | 3h | P1-004 |
| [TASK-AI-P1-006](./TASK-AI-P1-006.md) | Per-module `.ai-context.md` cho 5 modules | 3h | P0-001 |

### Phase 2 — Component Decomposition (Month 2–3) `~5.5 days total`
> Tách 18 god components → files < 250 LOC. AI edit precision +85%.

| Task | What | Effort | Deps |
|---|---|---|---|
| [TASK-AI-P2-001](./TASK-AI-P2-001.md) | Refactor `MembersPage.tsx` (1,993 LOC → 8 files) | 1 day | P1-003 |
| [TASK-AI-P2-002](./TASK-AI-P2-002.md) | Refactor `IDPsPage.tsx` + `IDPDetailPage.tsx` (3,729 LOC) | 1 day | P2-001 |
| [TASK-AI-P2-003](./TASK-AI-P2-003.md) | Refactor `ProjectSyncSchemaPage.tsx` + `InstanceFormBody.tsx` (4,109 LOC) | 1.5 days | P2-001 |
| [TASK-AI-P2-004](./TASK-AI-P2-004.md) | Refactor 10 remaining god components | 3 days | P2-001–003 |
| [TASK-AI-P2-005](./TASK-AI-P2-005.md) | Lint: `max-component-lines` + biome complexity | 2h | P2-004 |

### Phase 3 — State Architecture Migration (Month 3–5) `~4 days total`
> TanStack Query + Zustand thay thế 4-layer state. Data flow 8→2 levels.

| Task | What | Effort | Deps |
|---|---|---|---|
| [TASK-AI-P3-001](./TASK-AI-P3-001.md) | Install TanStack Query + QueryProvider + query-keys | 4h | — |
| [TASK-AI-P3-002](./TASK-AI-P3-002.md) | Query hooks: 5 core domains (database, project, instance, user, environment) | 1 day | P3-001 |
| [TASK-AI-P3-003](./TASK-AI-P3-003.md) | Query hooks: 19 remaining domains | 2 days | P3-002 |
| [TASK-AI-P3-004](./TASK-AI-P3-004.md) | Zustand stores: auth + ui + sqlEditor | 4h | P3-001 |
| [TASK-AI-P3-005](./TASK-AI-P3-005.md) | Migrate `useVueState` → hooks trong top-5 pages (68→<45 calls) | 2 days | P3-002, P3-004, P2-001 |

### Phase 4 — Finalization (Month 4+) `~9h total`
> Unify i18n, type-safe overlays, auto-generation scripts.

| Task | What | Effort | Deps |
|---|---|---|---|
| [TASK-AI-P4-001](./TASK-AI-P4-001.md) | `// i18n:` headers (814 files) + `I18N_GUIDE.md` | 3h | — |
| [TASK-AI-P4-002](./TASK-AI-P4-002.md) | `src/react/lib/overlay.ts` — type-safe portal API | 2h | — |
| [TASK-AI-P4-003](./TASK-AI-P4-003.md) | `scripts/generate-module-map.ts` + `generate-route-registry.ts` | 4h | P0-001, P2-004 |

---

## Coverage Verification

| Solution | Tasks |
|---|---|
| SOL-AI-001 Hybrid Framework | P0-001, P0-002, P3-004, P3-005 |
| SOL-AI-002 Proto-ES Volume | P0-006, P1-001, P1-002, P1-004 |
| SOL-AI-003 God Components | P1-003, P2-001, P2-002, P2-003, P2-004, P2-005 |
| SOL-AI-004 State Topology | P3-001, P3-002, P3-003, P3-004, P3-005 |
| SOL-AI-005 Codebase Scale | P0-001, P0-006, P1-006, P4-003 |
| SOL-AI-006 Non-Standard Patterns | P0-007, P1-003, P1-005, P4-002 |
| SOL-AI-007 ConnectRPC | P0-003, P1-004 |
| SOL-AI-008 Dual i18n | P1-005, P4-001 |
| SOL-AI-009 Routing | P0-005, P4-003 |
| SOL-AI-010 Domain Knowledge | P0-004 |

**Coverage: 10/10 Solutions → 20 Tasks ✅**

---

## Execution Order (Minimal Token Cost)

```
Week 1:  P0-006 → P0-001 → P0-002 → P0-003 → P0-004 → P0-005 → P0-007
Week 2:  P1-001 → P1-002 (parallel) | P1-003 (parallel) | P1-004 → P1-005
Week 3:  P1-006 | P4-001 | P4-002 (all parallelizable)
Month 2: P2-001 → P2-002 → P2-003 → P2-004 → P2-005
Month 3: P3-001 → P3-004 (parallel with P3-002 → P3-003) → P3-005
Month 4: P4-003 (after P2 complete)
```

**Each task is scoped to ≤ 150 tokens context needed** — AI reads task file + 2–3 source files only.
