# TASK-AI-P1-003: Tạo 7 Scaffold Templates trong `src/react/templates/`

> **Source**: SOL-AI-006 §2.1 | **Priority**: P1 | **Effort**: 3h  
> **Status**: ✅ DONE | **Deps**: —  
> **Phase**: 1 — Tooling & Lint

## Scope
- **NEW** `src/react/templates/new-page.tsx`
- **NEW** `src/react/templates/new-sheet-create.tsx`
- **NEW** `src/react/templates/new-sheet-edit.tsx`
- **NEW** `src/react/templates/new-dialog.tsx`
- **NEW** `src/react/templates/new-data-hook.ts`
- **NEW** `src/react/templates/new-action-hook.ts`
- **NEW** `src/react/templates/new-filter-hook.ts`

## What
Templates copy-paste ready để AI không cần "tự đoán" pattern — đặc biệt outer+inner+key sheet pattern.

## Implementation

### `new-page.tsx` (~40 LOC)
```tsx
// AI: Copy + rename to YourPageName.tsx
// RULES: named export, match filename, register in Vue Router
export function TemplatePage() {
  // Uncomment when hooks ready:
  // const { data, isLoading } = useTemplateData();
  // const { create, update } = useTemplateActions();
  return (
    <div className="flex flex-col gap-4 p-4">
      <h1 className="text-xl font-semibold text-main-text">TODO: Title</h1>
    </div>
  );
}
```

### `new-sheet-edit.tsx` (~70 LOC)
Full outer+inner+key pattern với useRef freeze, isDirty, isValid, disabled Update button.
Include TODO markers cho: entity type, updateMask fields, form fields.

### `new-sheet-create.tsx` (~50 LOC)
Simpler: no useRef, `key="new"` always, isDirty = hasRequiredFields.

### `new-dialog.tsx` (~35 LOC)
Confirmation dialog pattern: title, description, Cancel + Confirm buttons, AlertDialog for destructive.

### `new-data-hook.ts` (~40 LOC)
```typescript
// Pattern: useQuery + useQueryClient
export function useTemplateData(name: string) {
  return useQuery({
    queryKey: ["template", name],
    queryFn: () => templateServiceClientConnect.getTemplate({ name }),
    enabled: !!name,
  });
}
```

### `new-action-hook.ts` (~50 LOC)
useMutation patterns: create, update (with updateMask), delete — với onSuccess cache invalidation.

### `new-filter-hook.ts` (~40 LOC)
useState cho filters, sort, pagination — with URL sync pattern.

## AC
- [ ] 7 template files tạo xong
- [ ] Mỗi file có comment TODO markers rõ ràng
- [ ] `new-sheet-edit.tsx` implement đúng outer+inner+key pattern
- [ ] Templates không import placeholder modules (compile cleanly khi uncomment)
- [ ] Files thêm vào `.gitignore` exemption (templates không phải generated code)
