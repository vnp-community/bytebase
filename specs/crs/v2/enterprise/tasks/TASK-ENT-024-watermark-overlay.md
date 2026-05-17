# TASK-ENT-024 — Watermark Overlay

| Field | Value |
|---|---|
| **Task ID** | TASK-ENT-024 |
| **Source** | SOL-ENT-021 (CR-ENT-021) |
| **Status** | Done |
| **Priority** | P2 |
| **Complexity** | Low |
| **Sprint** | Sprint 1–2 |

---

## Mô tả

Watermark overlay trên SQL Editor results bằng Canvas rendering. Watermark policy quản lý qua `WATERMARK` policy type.

## Scope

### Phase 1 — Sprint 1: Policy Backend + Canvas Renderer
1. **Proto**: `WatermarkPolicy` — enabled, content type (USER_EMAIL/USER_NAME/CUSTOM_TEXT), custom_text, opacity (5-30%), font_size
2. **L4 — OrgPolicyService**: `WATERMARK` policy type, stored in `policy` table JSONB
3. **L1 — WatermarkOverlay.vue (NEW)**: Canvas-based renderer — diagonal repeated pattern, user email + timestamp
4. **L9 — Feature Gate**: `FeatureWatermark`

### Phase 2 — Sprint 2: Export + Anti-Tampering
5. **Export Integration**: CSV/Excel header includes user email + timestamp + workspace
6. **Anti-Tampering**: Multiple rendering layers (Canvas + CSS pseudo-elements), re-render on resize/scroll
7. **PrintScreen Detection**: Optional — detect + audit log event

## Acceptance Criteria

- [x] Watermark policy CRUD functional
- [x] Canvas watermark renders diagonal pattern with user info
- [x] Configurable opacity (5-30%) and font size
- [x] Exported data includes watermark header
- [x] Anti-tampering: watermark persists across resize/scroll
- [x] Watermark + masking applied simultaneously (CR-ENT-012)

## Dependencies

- CR-ENT-012 (Data Masking) — applied simultaneously
- CR-ENT-005 (Copy Restriction) — defense-in-depth
