# Change Request: Custom Logo / Branding

| Field | Value |
|---|---|
| **CR ID** | CR-ENT-020 |
| **Feature ID** | ADM-06 |
| **Title** | Custom Logo & Branding |
| **Plan** | ENTERPRISE |
| **Priority** | P2 — Medium |
| **Status** | Draft |
| **Created** | 2026-05-08 |

---

## 1. Tổng quan

Cho phép ENTERPRISE customers tùy chỉnh **logo và branding** trên giao diện Bytebase, tạo trải nghiệm white-label cho internal teams.

## 2. Yêu cầu chức năng

### FR-001: Custom Logo
- Upload custom logo (replace Bytebase logo)
- Supported formats: PNG, SVG, JPEG
- Size constraints: max 2MB, min 32x32px, max 512x512px
- Display locations: sidebar header, login page, browser favicon

### FR-002: Custom Branding Colors
- Primary color customization (sidebar, buttons, links)
- Optional: secondary color, accent color
- Preview trước khi apply
- Reset to default option

### FR-003: Custom Login Page
- Custom logo on login page
- Optional: custom welcome message
- Optional: custom background image/color
- Terms of Service / Privacy Policy links

### FR-004: Branding Persistence
- Branding settings stored in workspace settings
- Applied globally cho tất cả users trong workspace
- Branding visible cho unauthenticated users (login page)

## 3. Backend Changes

| Component | Thay đổi |
|---|---|
| `backend/api/v1/setting_service.go` | Branding settings CRUD |
| `backend/store/setting.go` | Store logo file + branding config |
| `enterprise/feature.go` | `FeatureCustomBranding` |

## 4. Test Cases

| TC | Mô tả | Expected |
|---|---|---|
| TC-001 | Upload custom logo (PNG) | Logo displayed in sidebar |
| TC-002 | Upload oversized logo (>2MB) | Rejected with error |
| TC-003 | Set custom primary color | UI theme updated |
| TC-004 | Reset to default branding | Bytebase default restored |
| TC-005 | Non-ENTERPRISE | Branding settings hidden |
