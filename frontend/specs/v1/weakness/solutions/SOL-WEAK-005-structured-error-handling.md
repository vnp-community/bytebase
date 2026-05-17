# SOL-WEAK-005: Structured Error Handling — Explicit Boundaries & Diagnostic Transparency

> **Source**: [BUG-WEAK-005](../bugs/BUG-WEAK-005-error-handling-antipatterns.md)  
> **Severity**: MEDIUM → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Thay thế blanket error suppression bằng explicit error policies: scoped ConnectError handling, React ErrorBoundary, và structured logging thay vì empty catches.

---

## 2. Thiết kế Chi tiết

### 2.1 ConnectError Scoped Suppression (Fix BUG 2.1)

```diff
// src/App.vue — onErrorCaptured
 onErrorCaptured((error: unknown) => {
   if (error instanceof ConnectError) {
-    if (Object.values(Code).includes(error.code)) {
-      return; // ALL ConnectErrors suppressed
-    }
+    // Only suppress codes that are definitively handled by interceptors
+    const INTERCEPTOR_HANDLED_CODES = [
+      Code.Unauthenticated,  // Handled by authInterceptor → SessionExpiredSurface
+      Code.PermissionDenied, // Handled by RoutePermissionGuardShell → 403
+      Code.Canceled,         // User-initiated cancellation (AbortController)
+    ];
+    if (INTERCEPTOR_HANDLED_CODES.includes(error.code)) {
+      return;
+    }
+    // Log unexpected ConnectErrors for diagnostics
+    console.error("[App] Unhandled ConnectError:", error.code, error.message);
   }
-  // ... show CRITICAL notification
-  return true; // prevent propagation
+  // ... show CRITICAL notification
+  return false; // ← Allow propagation to Vue error handler for tracking
 });
```

### 2.2 NotFound Default Behavior (Fix BUG 2.2)

```diff
// src/connect/middlewares/errorNotificationMiddleware.ts
 if ((ignoredCodes.length === 0
-  ? [Code.NotFound, Code.Unauthenticated]
+  ? [Code.Unauthenticated]  // Only suppress auth — NotFound should show notification
   : ignoredCodes
 ).includes(error.code)) {
   // ignored
 }
```

**Impact**: `Code.NotFound` will now show "Resource not found" notification by default. Callers that expect NotFound (e.g., `getOrCreate` patterns) should explicitly pass `ignoredCodes: [Code.NotFound]`.

### 2.3 React Error Boundary (Fix BUG 2.4)

Already defined in [SOL-WEAK-001](./SOL-WEAK-001-bridge-lifecycle-manager.md) Section 3.3.
- `ReactErrorBoundary` wraps all React pages in `BridgeLifecycleManager.mount()`
- Provides fallback UI with retry button
- Logs crash stack to console in dev

### 2.4 Empty Catch → Structured Logging

Replace all identified empty catches with appropriate handling:

```typescript
// Pattern 1: Expected failures (e.g., initial auth check)
try { ... }
catch (error) {
  // Expected when not authenticated — no action needed
  if (error instanceof ConnectError && error.code === Code.Unauthenticated) return;
  console.warn("[Module] Operation failed:", error);
}

// Pattern 2: Non-critical persistence (e.g., localStorage)
try { localStorage.setItem(key, value); }
catch (error) {
  console.warn("[Storage] Failed to persist:", key, error);
  // Degrade gracefully — feature works without persistence
}

// Pattern 3: Best-effort operations (e.g., AI suggestions)
try { await fetchAISuggestion(); }
catch (error) {
  console.warn("[AI] Suggestion fetch failed:", error);
  // Feature degrades — no suggestion shown, not a critical path
}
```

**Files to update:**
| File | Line | Current | Fix |
|---|---|---|---|
| `auth.ts` | L202 | `// nothing` | Pattern 1 (expected on logout) |
| `auth.ts` | L225 | `// do nothing` | Pattern 1 (expected on not-logged-in) |
| `worksheet.ts` | L382 | `// Nothing` | Pattern 2 (persistence) |
| `web-storage.ts` | L39, L58 | `// nothing` | Pattern 2 (localStorage) |
| `conversation.ts` | L276 | `// nothing todo` | Pattern 3 (AI) |
| `SingleResultViewV1.vue` | L742 | empty catch | Pattern 2 (clipboard) |

---

## 3. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | Scope ConnectError suppression in `App.vue` | MEDIUM — may surface new errors | 2h |
| 2 | Remove NotFound from default ignored codes | LOW — callers may need explicit opt-in | 1h |
| 3 | Replace empty catches (6 files) | LOW | 2h |
| 4 | React ErrorBoundary (cross-ref SOL-001) | — | (included in SOL-001) |

**Total**: ~5h

---

## 4. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Silently suppressed error codes | All 17 gRPC codes | 3 codes (Unauth, Permission, Canceled) |
| Empty catch blocks | 6+ files | 0 |
| React crash visibility | Silent blank page | Error boundary with retry |
| NotFound feedback | Silent | Notification displayed |
