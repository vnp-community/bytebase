# TASK-AI-P4-002: Tạo `src/react/lib/overlay.ts` — Type-safe Overlay API

> **Source**: SOL-AI-006 §2.2 | **Priority**: P2 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —  
> **Phase**: 4 — Framework Unification

## Scope
- **NEW** `src/react/lib/overlay.ts`
- **EDIT** `src/react/components/ui/layer.ts` — thêm JSDoc warnings

## What
Tạo type-safe overlay API để AI không thể bypass layering policy qua `document.body` portals.

## Implementation

### `src/react/lib/overlay.ts`
```typescript
import { createPortal } from "react-dom";
import { getLayerRoot } from "@/react/components/ui/layer";

/**
 * Type-safe overlay family selector.
 *
 * - "overlay": Standard app dialogs and sheets (default)
 * - "agent": AI agent surfaces — above overlay
 * - "critical": Session expired only — above agent
 *
 * @throws If called outside a mounted DOM (SSR)
 */
export type OverlayFamily = "overlay" | "agent" | "critical";

/**
 * Create a React portal into the correct semantic overlay layer.
 * DO NOT use createPortal(el, document.body) directly.
 *
 * @example
 * return createOverlayPortal(<MyDialog />, "overlay");
 */
export function createOverlayPortal(
  content: React.ReactNode,
  family: OverlayFamily = "overlay"
): React.ReactPortal {
  return createPortal(content, getLayerRoot(family));
}

/**
 * Hook: get the DOM root element for an overlay family.
 * Use when you need the container element directly.
 */
export function useLayerRoot(family: OverlayFamily = "overlay"): HTMLElement {
  return getLayerRoot(family);
}
```

### Update `src/react/components/ui/layer.ts` JSDoc
Add to `getLayerRoot`:
```typescript
/**
 * @deprecated Prefer createOverlayPortal() from @/react/lib/overlay.ts
 * unless you specifically need the container element.
 */
```

### Update AGENTS.md
Add to overlay section:
```markdown
## Overlay Portals — ALWAYS use createOverlayPortal()

```typescript
import { createOverlayPortal } from "@/react/lib/overlay";
// ✅ Correct
return createOverlayPortal(<MyModal />, "overlay");
// ❌ NEVER
return createPortal(<MyModal />, document.body);
```
```

## AC
- [x] `overlay.ts` tạo xong với 2 exports
- [x] TypeScript compiles — `OverlayFamily` type exported
- [x] AGENTS.md updated với overlay portal example
- [x] Existing overlay usage NOT broken (layer.ts unchanged functionally)
