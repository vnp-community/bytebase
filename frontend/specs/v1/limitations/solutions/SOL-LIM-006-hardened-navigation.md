# SOL-LIM-006 — Hardened Navigation & Redirect Validation

> **Resolves**: BUG-LIM-006 (Query Preservation Loop & Open Redirect)  
> **Type**: Security Fix + Code-Level Fix  
> **Priority**: Medium  
> **Effort**: Small (~2 ngày)  
> **Status**: Proposed

---

## 1. Mục Tiêu

1. Loại bỏ infinite query preservation loop qua **route-scoped whitelist**.
2. Close open redirect vulnerability qua **unified URL validator**.
3. Fix async store reset race condition.

---

## 2. Giải Pháp Kỹ Thuật

### 2.1 Route-Scoped Query Preservation (Thay thế global watch)

```typescript
// App.vue — REDESIGNED query preservation

// Per-route query whitelist via route meta
declare module "vue-router" {
  interface RouteMeta {
    /** Query params to preserve from previous route */
    preserveQuery?: string[];
  }
}

// Replace global "fields" with route-specific config
watch(route, (current, prev) => {
  const preservable = current.meta.preserveQuery;
  if (!preservable || preservable.length === 0) return;

  const preservedQuery = { ...current.query };
  let changed = false;

  for (const key of preservable) {
    if (preservedQuery[key] === undefined && prev.query[key] !== undefined) {
      preservedQuery[key] = prev.query[key];
      changed = true;
    }
  }

  if (!changed) return;

  // Use nextTick to avoid triggering during same navigation cycle
  nextTick(() => {
    router.replace({ ...current, query: preservedQuery });
  });
}, { flush: "post" });  // "post" ensures DOM is settled
```

**Route definitions:**
```typescript
// router/dashboard/workspace.ts — Example
{
  name: "workspace.databases",
  meta: {
    preserveQuery: ["project", "filter"],  // Only these params preserved
  }
}
```

### 2.2 Unified Redirect Validator

```typescript
// src/utils/redirect-validator.ts — NEW FILE

/**
 * Validate redirect URL to prevent open redirect attacks.
 * Only allows relative paths (no protocol, no authority).
 */
export function isValidRedirectUrl(url: string): boolean {
  if (!url || typeof url !== "string") return false;
  
  // Must start with /
  if (!url.startsWith("/")) return false;
  
  // Block protocol-relative URLs
  if (url.startsWith("//")) return false;
  
  // Block URL-encoded variants of //
  const decoded = decodeURIComponent(url);
  if (decoded.startsWith("//")) return false;
  
  // Block any protocol scheme (javascript:, data:, etc.)
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) return false;
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(decoded)) return false;
  
  // Block null bytes
  if (url.includes("\0") || url.includes("%00")) return false;
  
  return true;
}

/**
 * Sanitize redirect URL — returns "/" if invalid.
 */
export function sanitizeRedirectUrl(url: string | undefined | null): string {
  if (!url || !isValidRedirectUrl(url)) return "/";
  return url;
}
```

**Apply to router:**
```typescript
// router/index.ts — UPDATED redirect handling

import { sanitizeRedirectUrl } from "@/utils/redirect-validator";

// For relay_state
const redirect = sanitizeRedirectUrl(relayState);

// For redirect param — CRITICAL FIX: was unvalidated
const redirect = sanitizeRedirectUrl(redirectParam);
```

### 2.3 Synchronous Store Reset

```typescript
// router/index.ts — UPDATED auth route handler

if (isAuthRelatedRoute(to.name as string)) {
  // Synchronous resets
  useDatabaseV1Store().reset();
  useProjectV1Store().reset();
  useInstanceV1Store().reset();
  
  // Async reset — await BEFORE navigation
  try {
    const { useConversationStore } = await import("@/plugins/ai/store");
    useConversationStore().reset();
  } catch {
    // AI plugin may not be loaded — safe to ignore
  }
  
  next();
  return;
}
```

---

## 3. Thay Đổi Architecture Document

### 3.1 Cập nhật `specs/architecture.md` — Section 5.3 Navigation Guards

Thêm bullet point:

> 10. **Redirect validation** — All redirect URLs (`relay_state`, `redirect`) validated via `sanitizeRedirectUrl()` to prevent open redirect attacks.

### 3.2 Cập nhật `specs/architecture.md` — Section 9.3 Security Features

Thêm:

> - **Open Redirect Prevention**: Unified `isValidRedirectUrl()` validates ALL redirect parameters — blocks protocol-relative URLs, encoded schemes, and null bytes.
> - **Query Preservation Scoping**: Query params only preserved when explicitly declared in route meta (`preserveQuery`), preventing stale filter carry-over.

### 3.3 Cập nhật `specs/technical-design-document.md` — Section 3.6.3 Query Preservation

Thay thế nội dung hiện tại:

> **Query Preservation**: Route meta `preserveQuery` array declares which query params should carry over from the previous route. Only params in this whitelist are preserved. The watch uses `flush: "post"` and `nextTick` to prevent triggering during same navigation cycle.

---

## 4. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Query preservation infinite loop | Possible | Impossible (flush: post + nextTick) |
| Stale filter carry-over | Global 5 params | Route-scoped whitelist |
| Open redirect via `redirect` param | Unvalidated | Validated (sanitizeRedirectUrl) |
| AI store stale on auth page | Possible (async) | Impossible (awaited) |
| URL-encoded redirect bypass | Possible | Blocked (double-decode check) |
