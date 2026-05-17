# Solution: CR-ENT-020 — Custom Logo & Branding

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-020                |
| **Solution**   | SOL-ENT-020               |
| **Status**     | Proposed                  |
| **Complexity** | Low                       |

---

## 1. Tóm tắt giải pháp

Lưu branding config (logo, colors, welcome message) trong workspace settings (L8 `setting` table, JSONB payload). Frontend đọc branding settings tại bootstrap và apply CSS custom properties. Logo served via dedicated endpoint.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `setting_service.go` | Branding settings CRUD |
| **L8 — Store** | `setting` table | Store logo binary + config JSONB |
| **L9 — Enterprise** | `feature.go` | `FeatureCustomBranding` gate |
| **L2 — API Gateway** | `echo_routes.go` | Logo serving endpoint |
| **L1 — Presentation** | Sidebar, Login page, Favicon | Apply branding |

---

## 3. Chi tiết Implementation

### 3.1 Branding Settings Schema

```json
{
  "setting_name": "bb.workspace.branding",
  "value": {
    "logo_url": "/api/v1/branding/logo",
    "primary_color": "#6366f1",
    "secondary_color": "#8b5cf6",
    "welcome_message": "Welcome to MyCompany Database Portal",
    "login_background_color": "#1e1b4b",
    "favicon_url": "/api/v1/branding/favicon"
  }
}
```

### 3.2 Logo Upload

- Supported: PNG, SVG, JPEG
- Size: max 2MB, 32×32 to 512×512px
- Storage: `setting` table JSONB + binary in separate column or file storage
- Serving: `GET /api/v1/branding/logo` — public endpoint (unauthenticated for login page)

### 3.3 Frontend Theming

```css
:root {
  --bb-primary: var(--branding-primary, #6366f1);
  --bb-secondary: var(--branding-secondary, #8b5cf6);
}
```

Frontend reads branding settings at bootstrap and overrides CSS custom properties.

---

## 4. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Branding settings CRUD + logo upload | Sprint 1 |
| 2 | Frontend theming | Sprint 1 |
| 3 | Login page customization | Sprint 2 |
