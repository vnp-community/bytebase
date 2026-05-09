# Change Request: Watermark

| Field | Value |
|---|---|
| **CR ID** | CR-ENT-021 |
| **Feature ID** | ADM-07 |
| **Title** | Watermark for SQL Editor |
| **Plan** | ENTERPRISE |
| **Priority** | P2 — Medium |
| **Status** | Draft |
| **Created** | 2026-05-08 |

---

## 1. Tổng quan

Hiển thị **watermark** trên SQL Editor results để deter unauthorized data sharing. Watermark chứa user identity, đảm bảo traceability nếu screenshots/exports bị leak.

## 2. Yêu cầu chức năng

### FR-001: Watermark Display
- Semi-transparent watermark overlay trên SQL Editor result grid
- Watermark content: user email + timestamp
- Pattern: diagonal repeated text across result area
- Opacity configurable (default: 10%)

### FR-002: Watermark Policy
- Admin enable/disable watermark per workspace
- Scope: SQL Editor results, Data Export previews
- Policy fields:
  - `enabled`: boolean
  - `content`: `USER_EMAIL` | `USER_NAME` | `CUSTOM_TEXT`
  - `opacity`: 5-30%
  - `font_size`: configurable

### FR-003: Anti-Tampering
- Watermark rendered via CSS/Canvas (not easily removable via DevTools)
- Multiple rendering layers for resilience
- Screenshot detection (optional): detect PrintScreen key + log audit

### FR-004: Integration
- Works with Data Masking (CR-ENT-012): both applied simultaneously
- Works with Copy Restriction (CR-ENT-005): defense-in-depth
- Export includes watermark in header/footer

## 3. Backend Changes

| Component | Thay đổi |
|---|---|
| `backend/api/v1/org_policy_service.go` | Watermark policy CRUD |
| `enterprise/feature.go` | `FeatureWatermark` |

## 4. Frontend Changes

| Component | Thay đổi |
|---|---|
| SQL Result Table | Canvas-based watermark overlay |
| Watermark Component | Reusable watermark renderer |
| Policy Settings | Watermark configuration UI |

## 5. Test Cases

| TC | Mô tả | Expected |
|---|---|---|
| TC-001 | Enable watermark | Watermark visible on results |
| TC-002 | Watermark shows user email | Correct email displayed |
| TC-003 | Disable watermark | No watermark visible |
| TC-004 | Screenshot of watermarked results | User traceable from image |
| TC-005 | Non-ENTERPRISE | Watermark feature hidden |
