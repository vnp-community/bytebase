# SOL-AI-006 — Pattern Codification + Scaffold Templates

> **Resolves**: ISS-AI-006 (Implicit Convention và Non-Standard Patterns)  
> **Type**: Documentation + Tooling  
> **Priority**: High  
> **Effort**: Medium (~1–2 weeks)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Biến tất cả implicit conventions thành **explicit, machine-verifiable rules** và **code scaffolds** — loại bỏ hoàn toàn "AI không biết pattern này".

---

## 2. Giải Pháp

### 2.1 Pattern Codification — Scaffold Templates

Tạo `src/react/templates/` (AI sử dụng trực tiếp như copy-paste base):

```
src/react/templates/
├── new-page.tsx              # Template cho new React page
├── new-sheet-create.tsx      # Template cho Create sheet
├── new-sheet-edit.tsx        # Template cho Edit sheet (outer+inner+key pattern)
├── new-dialog.tsx            # Template cho confirmation dialog
├── new-data-hook.ts          # Template cho TanStack Query hook
├── new-action-hook.ts        # Template cho CRUD actions hook
└── new-filter-hook.ts        # Template cho filter/sort/pagination hook
```

**`new-page.tsx` template:**

```tsx
/**
 * TEMPLATE: New React Page
 * Usage: Copy this file, rename to YourPageName.tsx, replace all TODO markers
 *
 * RULES:
 * 1. Named export ONLY — file name MUST match function name
 * 2. Register in src/router/dashboard/ and src/react/mount.ts glob
 * 3. Use semantic color tokens, not raw colors
 * 4. Overlays → use getLayerRoot("overlay"), NOT document.body
 */

// TODO: Rename to YourPageName
export function TemplatePage() {
  // TODO: Import data hook
  // const { data, isLoading, error } = useTemplateData();
  
  // TODO: Import actions hook  
  // const { create, update, delete: remove } = useTemplateActions();
  
  return (
    // TODO: Replace with actual content
    <div className="flex flex-col gap-4 p-4">
      {/* Use semantic tokens: bg-main, text-main-text, border-control-border */}
      {/* Use gap-*, not space-x-* or space-y-* */}
      {/* Use size-* for equal dimensions */}
      <h1 className="text-xl font-semibold">TODO: Page Title</h1>
    </div>
  );
}
```

**`new-sheet-edit.tsx` template (outer+inner+key pattern):**

```tsx
/**
 * TEMPLATE: Edit Sheet — Outer wrapper + Inner form + Key pattern
 *
 * CRITICAL: Do NOT use useState(entity.field) in outer component.
 * This stale pattern causes data not to refresh when editing different entities.
 * Always use the outer/inner split with useRef freeze + key reset.
 */

interface TemplateEditSheetProps {
  open: boolean;
  entity: TemplateEntity | undefined;
  onClose: () => void;
  onUpdated?: (entity: TemplateEntity) => void;
}

// OUTER: Freezes entity during close animation (~200ms)
export function TemplateEditSheet(props: TemplateEditSheetProps) {
  const { open, entity, onClose } = props;
  
  // Freeze last-open entity so inner form stays stable during close animation
  const openEntityRef = useRef(entity);
  if (open) {
    openEntityRef.current = entity;
  }
  const stableEntity = openEntityRef.current;

  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent width="standard">
        {/* key forces fresh mount when different entity is opened */}
        <TemplateForm
          key={stableEntity?.name ?? "new"}
          entity={stableEntity}
          onClose={onClose}
          onUpdated={props.onUpdated}
        />
      </SheetContent>
    </Sheet>
  );
}

// INNER: useState initializers always read fresh entity via key reset
function TemplateForm({
  entity,
  onClose,
  onUpdated,
}: {
  entity: TemplateEntity | undefined;
  onClose: () => void;
  onUpdated?: (entity: TemplateEntity) => void;
}) {
  // Safe: initializers run fresh on each mount (key forces remount)
  const [title, setTitle] = useState(entity?.title ?? "");
  
  // isDirty: disable Save until changed
  const initialTitle = useRef(entity?.title ?? "");
  const isDirty = title !== initialTitle.current;
  const isValid = title.trim().length > 0;

  const { mutate: updateEntity } = useUpdateTemplate();

  const handleSave = () => {
    if (!entity || !isDirty || !isValid) return;
    updateEntity(
      { entity: { ...entity, title }, updateMask: ["title"] },
      { onSuccess: (updated) => { onUpdated?.(updated); onClose(); } }
    );
  };

  return (
    <SheetBody>
      <SheetTitle>Edit Template</SheetTitle>
      {/* form fields */}
      <Button onClick={handleSave} disabled={!isDirty || !isValid}>
        Save
      </Button>
    </SheetBody>
  );
}
```

### 2.2 Overlay Layering Policy — Enforced via Types

Tạo type-safe API để AI không thể bypass policy:

```typescript
// src/react/lib/overlay.ts — Typed overlay API
import { getLayerRoot } from "@/react/components/ui/layer";

/**
 * Use this function instead of createPortal(el, document.body)
 * or any manual z-index manipulation.
 *
 * @param family - "overlay" for app dialogs/sheets,
 *                 "agent" for AI agent surfaces,
 *                 "critical" for session-expired only
 */
export function createOverlayPortal(
  content: React.ReactNode,
  family: "overlay" | "agent" | "critical" = "overlay"
) {
  return createPortal(content, getLayerRoot(family));
}

// DEPRECATED: Do NOT use
// createPortal(content, document.body)  ← will fail layering check
```

### 2.3 Named Export Enforcement

Thêm ESLint rule (`eslint-rules/react-page-named-export.mjs`):

```javascript
// Enforce named export in React page files
// src/react/pages/**/*.tsx MUST have named export matching filename
export const reactPageNamedExport = {
  create(context) {
    const filename = path.basename(context.filename, ".tsx");
    if (!context.filename.includes("/pages/")) return {};
    
    return {
      Program(node) {
        const hasNamedExport = node.body.some(
          (stmt) =>
            stmt.type === "ExportNamedDeclaration" &&
            stmt.declaration?.id?.name === filename
        );
        if (!hasNamedExport) {
          context.report({
            node,
            message: `Page file "${filename}.tsx" must export a named function "${filename}". Use "export function ${filename}()" not "export default".`,
          });
        }
      },
    };
  },
};
```

### 2.4 Styling Convention Lint

Thêm vào `biome.json` hoặc ESLint config:

```json
// Detect raw color classes in JSX className
// Flag: bg-blue-*, text-red-*, bg-green-* etc.
// Allow: bg-accent, bg-main, text-control, text-error (semantic tokens)
```

Tạo `scripts/check-semantic-tokens.mjs`:

```javascript
// Scans TSX files for raw Tailwind color classes
// Reports violations with file:line:col
// Run in CI: node scripts/check-semantic-tokens.mjs
const rawColorPattern = /bg-(blue|red|green|yellow|purple|gray|slate|zinc|stone)-\d{3}/;
```

### 2.5 AGENTS.md Enhancement

Nâng cấp AGENTS.md từ "guide" thành "decision tree + checklist":

```markdown
## AI Task Checklists

### Checklist: Add New Page
- [ ] File in `src/react/pages/{section}/YourPageName.tsx`
- [ ] Copy from `src/react/templates/new-page.tsx`
- [ ] Export: `export function YourPageName()` (NOT export default)
- [ ] Register route in `src/router/dashboard/{section}.ts`
- [ ] Add glob to `src/react/mount.ts` if new page section
- [ ] Set `meta.requiredPermissionList` on route

### Checklist: Add Edit Sheet
- [ ] Copy from `src/react/templates/new-sheet-edit.tsx`
- [ ] Outer component: useRef freeze pattern
- [ ] Inner component: `key={entity?.name ?? "new"}`
- [ ] isDirty computed from useMemo vs initial values
- [ ] Update button disabled when `!isDirty || !isValid`

### Checklist: Add Dialog
- [ ] Use `<Dialog>` for confirmations, `<AlertDialog>` for destructive
- [ ] Use `<Sheet>` for multi-field forms
- [ ] Portal via `getLayerRoot("overlay")` — NOT document.body
- [ ] NO raw z-index values
- [ ] Include `<DialogTitle>` (can be sr-only)
```

---

## 3. Implementation Checklist

- [ ] Tạo `src/react/templates/` với 7 template files
- [ ] Tạo `src/react/lib/overlay.ts` typed overlay API
- [ ] Thêm ESLint rule `react-page-named-export`
- [ ] Tạo `scripts/check-semantic-tokens.mjs` + CI integration
- [ ] Update AGENTS.md với checklists + decision trees
- [ ] Update `src/react/components/ui/layer.ts` docs

---

## 4. Acceptance Criteria

| Pattern | Current (AI compliance rate) | Target |
|---|---|---|
| Named export convention | ~60% (AI guesses) | 100% (lint-enforced) |
| Sheet outer+inner+key pattern | ~30% | 100% (template) |
| Semantic color tokens | ~50% | 100% (lint-enforced) |
| Overlay layering policy | ~40% | 100% (typed API) |
| New page registration flow | ~40% | 100% (checklist) |
