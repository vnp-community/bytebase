# TASK-ENT-023 — Custom Logo & Branding

| Field | Value |
|---|---|
| **Task ID** | TASK-ENT-023 |
| **Source** | SOL-ENT-020 (CR-ENT-020) |
| **Status** | Done |
| **Priority** | P2 |
| **Complexity** | Low |
| **Sprint** | Sprint 1–2 |

---

## Mô tả

Branding config (logo, colors, welcome message) trong workspace settings. Frontend apply CSS custom properties.

## Scope

1. **Settings Schema**: `bb.workspace.branding` — logo_url, primary/secondary color, welcome message, favicon
2. **L4 — SettingService**: Branding CRUD
3. **Logo Upload**: PNG/SVG/JPEG, max 2MB, 32–512px
4. **Logo Endpoint**: `GET /api/v1/branding/logo` — public (unauthenticated)
5. **Frontend Theming**: CSS custom properties override at bootstrap
6. **Login Page**: Custom logo, colors, welcome message, favicon
7. **L9 — Feature Gate**: `FeatureCustomBranding`

## Acceptance Criteria

- [x] Branding settings CRUD functional
- [x] Logo upload validates format/size/dimensions
- [x] CSS custom properties applied from branding
- [x] Login page customization functional
- [x] Fallback to defaults when no branding configured
