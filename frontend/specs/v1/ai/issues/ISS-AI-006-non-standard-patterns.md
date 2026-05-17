# ISS-AI-006 — Implicit Convention và Non-Standard Patterns Không Có Trong Training Data

> **Category**: Non-Standard Patterns  
> **Severity**: High  
> **Impact**: Code Generation, Pattern Compliance  
> **Affected Area**: Bridge, Layout, Plugin, Overlay systems

---

## 1. Mô Tả Vấn Đề

Codebase chứa nhiều pattern tự định nghĩa không có trong training data phổ biến của AI, khiến AI generate code vi phạm conventions.

### 1.1 Teleport-based Layout System

```
DashboardLayout.vue
  ├── teleport body → BodyLayout.vue
  │   ├── mount React → DashboardBodyShell (React)
  │   │   ├── exposes DOM refs: sidebar, content, quickstart
  │   ├── teleport sidebar → Router leftSidebar
  │   ├── teleport content → RoutePermissionGuardShell
```

AI không hiểu Vue `<teleport>` + React portal + DOM ref forwarding kết hợp cùng lúc.

### 1.2 Overlay Layering Policy (3 Families)

| Family | Mục đích | Mount point |
|---|---|---|
| `overlay` | App dialogs, sheets | `getLayerRoot("overlay")` |
| `agent` | AI agent window | `getLayerRoot("agent")` |
| `critical` | Session expired | `getLayerRoot("critical")` |

**Rules AI thường vi phạm:**
- Raw `z-index` values bị cấm
- Không portal trực tiếp lên `document.body`
- Children inherit parent family
- `LAYER_SURFACE_CLASS` / `LAYER_BACKDROP_CLASS` bắt buộc

### 1.3 Sheet Pattern (Outer wrapper + Inner form + Key)

```tsx
// AI thường generate sai:
function EditSheet({ open, entity }) {
  const [field, setField] = useState(entity.field); // ❌ Stale trên re-open
  return <Sheet open={open}><Form /></Sheet>;
}

// Correct pattern yêu cầu:
function EditSheet({ open, entity, onClose }) {
  const openEntityRef = useRef(entity);      // ❌ AI không biết pattern này
  if (open) openEntityRef.current = entity;
  const stableEntity = openEntityRef.current;
  return (
    <Sheet open={open}>
      <InnerForm key={stableEntity?.name ?? "new"} entity={stableEntity} />
    </Sheet>
  );
}
```

### 1.4 React Page Name Convention

- File name PHẢI match exported function name (e.g. `MembersPage.tsx` exports `MembersPage`).
- `mount.ts` loads by `mod[name]` — nếu tên không khớp, page sẽ crash silently.
- AI thường dùng `export default` → sai, phải dùng named export.

### 1.5 Component Guidelines Implicit

- `gap-*` not `space-x-*` / `space-y-*`
- `size-*` not `w-* h-*`
- `truncate` not `overflow-hidden text-ellipsis whitespace-nowrap`
- Semantic tokens (`bg-accent`) not raw colors (`bg-blue-500`)
- `cn()` from `@/react/lib/utils` for conditional classes

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **New page** | AI generate `export default` → crash; không set up Vue Router entry |
| **New dialog** | AI set raw `z-index` → violates layering policy |
| **Edit sheet** | AI generate simple `useState(entity.field)` → stale data on re-open |
| **Styling** | AI uses `bg-blue-500` → violates semantic token rule |
| **Layout change** | AI doesn't understand teleport-based composition |

## 3. Khuyến Nghị

1. **AGENTS.md đã tốt nhưng cần structured format**: Convert conventions thành checkable rules AI có thể verify.
2. **Template scaffolds**: Cung cấp "new page" / "new sheet" / "new dialog" templates cho AI.
3. **Lint enforcement**: Biome/ESLint rules cho naming convention, z-index ban, semantic token enforcement.
4. **Code generation tests**: Test rằng AI-generated code passes `pnpm check` + layering scanner.
