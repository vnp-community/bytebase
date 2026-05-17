# TASK-L-017: Browser Baseline Upgrade

> **Source**: SOL-LIM-005 §2.1 | **Priority**: P3 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `package.json` (browserslist field)

## What
Nâng browser baseline lên Chrome 84+, Firefox 79+, Safari 14.1+, Edge 84+ để loại bỏ nhu cầu WeakRef polyfill.

## Implementation

```diff
  "browserslist": [
-   "> 0.08%, not dead"
+   "Chrome >= 84, Firefox >= 79, Safari >= 14.1, Edge >= 84, not dead"
  ]
```

## AC
- [ ] `npx browserslist` shows Chrome 84+, Firefox 79+, Safari 14.1+, Edge 84+
- [ ] Coverage > 98% global users
- [ ] Build succeeds with new browserslist
