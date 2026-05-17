# Solution: CR-ENT-021 — Watermark

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-021                |
| **Solution**   | SOL-ENT-021               |
| **Status**     | Proposed                  |
| **Complexity** | Low                       |

---

## 1. Tóm tắt giải pháp

Triển khai watermark overlay trên SQL Editor results bằng Canvas rendering (L1). Watermark policy quản lý qua `WATERMARK` policy type trong `org_policy_service` (L4). Nội dung watermark: user email + timestamp, hiển thị diagonal repeated pattern.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `org_policy_service.go` | Watermark policy CRUD |
| **L9 — Enterprise** | `feature.go` | `FeatureWatermark` gate |
| **L1 — Presentation** | `WatermarkOverlay.vue` (NEW) | Canvas-based watermark renderer |
| **L1 — Presentation** | SQL Result Table | Apply watermark overlay |

---

## 3. Chi tiết Implementation

### 3.1 Watermark Policy

```protobuf
message WatermarkPolicy {
  bool enabled = 1;
  WatermarkContent content = 2;  // USER_EMAIL | USER_NAME | CUSTOM_TEXT
  string custom_text = 3;
  int32 opacity_percent = 4;     // 5-30, default 10
  int32 font_size = 5;           // default 14
}
```

Stored in `policy` table (existing) as JSONB payload.

### 3.2 Frontend Canvas Renderer

```javascript
// WatermarkOverlay.vue
function renderWatermark(canvas, config) {
    const ctx = canvas.getContext('2d');
    ctx.globalAlpha = config.opacity / 100;
    ctx.font = `${config.fontSize}px monospace`;
    ctx.fillStyle = '#000000';

    const text = `${userEmail} | ${timestamp}`;
    const angle = -30 * Math.PI / 180;
    ctx.rotate(angle);

    // Repeat pattern across canvas
    for (let y = -canvas.height; y < canvas.height * 2; y += 80) {
        for (let x = -canvas.width; x < canvas.width * 2; x += 300) {
            ctx.fillText(text, x, y);
        }
    }
}
```

### 3.3 Anti-Tampering

- Multiple rendering layers (Canvas + CSS pseudo-elements)
- Watermark re-renders on window resize/scroll
- Optional: detect PrintScreen key press → audit log event

### 3.4 Export Integration

Exported data (CSV/Excel) includes watermark in header:
```
# Exported by: user@example.com
# Export time: 2026-05-13T10:00:00Z
# Workspace: my-workspace
```

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-012 | Watermark + masking applied simultaneously |
| CR-ENT-005 | Watermark + copy restriction = defense-in-depth |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Watermark policy backend | Sprint 1 |
| 2 | Canvas watermark renderer | Sprint 1 |
| 3 | Export integration | Sprint 2 |
| 4 | Anti-tampering hardening | Sprint 2 |
