# Bytebase Frontend — AI Development Solutions Index

> **Version**: 1.0.0  
> **Date**: 2026-05-13  
> **Scope**: Giải pháp khắc phục 100% vấn đề AI development được xác định trong `specs/v1/ai/issues/`

---

## Tổng Quan

Tài liệu này định nghĩa **10 solutions** giải quyết toàn bộ issues đã phân tích. Các solutions được phân loại theo impact và effort, với lộ trình triển khai phân giai đoạn.

---

## Solution Map

| Solution | Resolves | Type | Priority | Effort |
|---|---|---|---|---|
| [SOL-AI-001](./SOL-AI-001-unified-react-first-architecture.md) | ISS-AI-001 Hybrid Framework | Architectural Change | 🔴 Critical | Large |
| [SOL-AI-002](./SOL-AI-002-proto-es-ai-reference-layer.md) | ISS-AI-002 Proto-ES Volume | Tooling + Docs | 🟠 High | Medium |
| [SOL-AI-003](./SOL-AI-003-component-decomposition-standard.md) | ISS-AI-003 God Components | Code Architecture | 🟠 High | Large |
| [SOL-AI-004](./SOL-AI-004-unified-state-management.md) | ISS-AI-004 State Topology | Architectural Change | 🟠 High | Large |
| [SOL-AI-005](./SOL-AI-005-ai-context-system.md) | ISS-AI-005 Codebase Scale | Documentation Infra | 🔴 Critical | Medium |
| [SOL-AI-006](./SOL-AI-006-pattern-codification-templates.md) | ISS-AI-006 Non-Standard Patterns | Tooling + Docs | 🟠 High | Medium |
| [SOL-AI-007](./SOL-AI-007-connectrpc-ai-guide.md) | ISS-AI-007 ConnectRPC | Docs + Tooling | 🟡 Medium | Small |
| [SOL-AI-008](./SOL-AI-008-i18n-unification.md) | ISS-AI-008 Dual i18n | Technical Change | 🟡 Medium | Medium |
| [SOL-AI-009](./SOL-AI-009-route-registry-playbook.md) | ISS-AI-009 Routing | Docs + Tooling | 🟡 Medium | Small |
| [SOL-AI-010](./SOL-AI-010-domain-knowledge-base.md) | ISS-AI-010 Domain Knowledge | Documentation | 🟡 Medium | Medium |

---

## Implementation Roadmap

### Phase 0 — Immediate Quick Wins (Week 1–2)

> Không thay đổi code, chỉ tạo tài liệu và tooling. Áp dụng ngay lập tức.

| Action | Solution | Effort |
|---|---|---|
| Tạo `.ai-context/` directory với 8 root context files | SOL-AI-005 | 2 days |
| Tạo `.ai-context/CONNECTRPC_GUIDE.md` + `ERROR_POLICY.md` | SOL-AI-007 | 1 day |
| Tạo `.ai-context/GLOSSARY.md` + `WORKFLOWS.md` | SOL-AI-010 | 2 days |
| Tạo `.ai-context/NEW_PAGE_PLAYBOOK.md` | SOL-AI-009 | 1 day |
| Thêm `@ai-exclude` markers vào proto-es + openapi files | SOL-AI-005 | 0.5 day |
| Tạo `.aiignore` file | SOL-AI-005 | 0.5 day |
| Update AGENTS.md với checklists | SOL-AI-006 | 1 day |

**Expected Impact**: AI code generation accuracy tăng ~30% ngay lập tức.

---

### Phase 1 — Tooling & Lint (Week 2–4)

> Thêm guardrails tự động để enforce conventions.

| Action | Solution | Effort |
|---|---|---|
| Tạo `src/types/ai-ref/` với 10 domain type stubs | SOL-AI-002 | 3 days |
| Tạo `src/types/ai-ref/service-map.ts` | SOL-AI-002 | 1 day |
| Tạo `src/react/templates/` với 7 scaffold templates | SOL-AI-006 | 2 days |
| Thêm ESLint rules: `no-proto-constructor`, `no-fetch-for-grpc`, `require-update-mask` | SOL-AI-002 + 007 | 2 days |
| Thêm ESLint rule: `react-page-named-export` | SOL-AI-006 | 1 day |
| Script `generate-ai-ref.ts` + `generate-route-registry.ts` + `generate-module-map.ts` | SOL-AI-002 + 009 + 005 | 3 days |
| Thêm `// i18n:` header comments vào all framework files | SOL-AI-008 | 1 day |

**Expected Impact**: AI convention violations giảm ~60%.

---

### Phase 2 — Component Decomposition (Month 2–3)

> Refactor god components thành modular structure.

| Action | Solution | Effort |
|---|---|---|
| Refactor P0 components: 3 files (ProjectSyncSchema, IDPs, Members) | SOL-AI-003 | 3 weeks |
| Refactor P1 components: 5 files (DataSourceForm, InstanceForm, Env, IDPDetail, PlanDetail) | SOL-AI-003 | 3 weeks |
| Refactor P2+ components: 10 files | SOL-AI-003 | 4 weeks |
| Thêm biome `maxCognitiveComplexity` rule | SOL-AI-003 | 0.5 day |
| Thêm ESLint `max-component-lines` rule | SOL-AI-003 | 1 day |

**Expected Impact**: AI edit precision tăng từ ~50% lên >85%.

---

### Phase 3 — State Architecture Migration (Month 3–5)

> Replace 4-layer state topology với TanStack Query + Zustand.

| Action | Solution | Effort |
|---|---|---|
| Install TanStack Query v5 + setup QueryProvider | SOL-AI-004 | 2 days |
| Tạo query-keys.ts + 5 core domain hooks | SOL-AI-004 | 1 week |
| Migrate remaining 25+ domain queries | SOL-AI-004 | 3 weeks |
| Create Zustand stores (auth, ui, sqlEditor) | SOL-AI-004 | 1 week |
| Remove Pinia after migration complete | SOL-AI-004 | 1 week |

**Expected Impact**: State reasoning depth giảm từ 8 → 2 levels.

---

### Phase 4 — Framework Unification (Month 4–8)

> React-only architecture + i18n unification.

| Action | Solution | Effort |
|---|---|---|
| Migrate Pinia auth/permission → Zustand (unblock useVueState removal) | SOL-AI-001 | 2 weeks |
| Migrate top 20 high-`useVueState` pages to pure React hooks | SOL-AI-001 | 4 weeks |
| Merge i18n locale files + switch React to unified i18next | SOL-AI-008 | 2 weeks |
| React Router v7 integration (parallel with Vue Router) | SOL-AI-001 | 2 weeks |
| Migrate remaining Vue pages → React | SOL-AI-001 | 4–8 weeks |
| Remove Vue Router when Vue pages < 10% | SOL-AI-001 | 1 week |

**Expected Impact**: Hybrid complexity eliminated. AI works in single framework.

---

## Architecture Changes Summary

### `specs/architecture.md` Updates Required

| Section | Change | Solution |
|---|---|---|
| 1. Tech Stack | Add React Router v7, TanStack Query to target stack | SOL-AI-001 + 004 |
| 4. Hybrid Vue + React | Add 4-phase migration roadmap | SOL-AI-001 |
| 6. State Management | Replace Pinia architecture with TanStack Query + Zustand | SOL-AI-004 |
| 8. API Communication | Add AI Reference Layer (src/types/ai-ref) | SOL-AI-002 |
| NEW Section 14 | AI Context System architecture | SOL-AI-005 |

### `specs/technical-design-document.md` Updates Required

| Section | Change | Solution |
|---|---|---|
| 2. Bootstrap Sequence | Add QueryProvider to bootstrap order | SOL-AI-004 |
| 3.2 ConnectRPC Transport | Add AI Reference Layer subsection | SOL-AI-002 |
| 3.3 State Management | Replace with TanStack Query + Zustand design | SOL-AI-004 |
| 3.8 Internationalization | Add i18next unification roadmap | SOL-AI-008 |
| NEW Section 5 | Component Decomposition Standard | SOL-AI-003 |

---

## Expected Outcomes (After All Phases)

```
AI Code Generation Accuracy:  ██████████  ~95% (từ ~60%)
Refactoring Safety:           █████████░  ~90% (từ ~40%)
Bug Fix Precision:            █████████░  ~90% (từ ~50%)
Feature Development Speed:    ████████░░  ~80% (từ ~60%)
State Debugging Time:         ↓ 75% reduction
Context Files Needed per Task: 1–3 (từ 10–20)
```

---

## Coverage Verification

| Issue | Solution | Status |
|---|---|---|
| ISS-AI-001 Hybrid Vue+React | SOL-AI-001 | ✅ Covered |
| ISS-AI-002 Proto-ES Volume | SOL-AI-002 | ✅ Covered |
| ISS-AI-003 God Components | SOL-AI-003 | ✅ Covered |
| ISS-AI-004 State Topology | SOL-AI-004 | ✅ Covered |
| ISS-AI-005 Codebase Scale | SOL-AI-005 | ✅ Covered |
| ISS-AI-006 Non-Standard Patterns | SOL-AI-006 | ✅ Covered |
| ISS-AI-007 ConnectRPC | SOL-AI-007 | ✅ Covered |
| ISS-AI-008 Dual i18n | SOL-AI-008 | ✅ Covered |
| ISS-AI-009 Routing Complexity | SOL-AI-009 | ✅ Covered |
| ISS-AI-010 Domain Knowledge | SOL-AI-010 | ✅ Covered |

**Coverage: 10/10 (100%)**
