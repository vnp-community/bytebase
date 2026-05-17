# Error Handling Policy

> **TL;DR**: ConnectRPC interceptors handle most errors automatically. DO NOT add try/catch unless you need custom behavior.

---

## What Interceptors Already Handle

The ConnectRPC transport layer (`src/connect/`) includes interceptors that run on EVERY request:

| Error | Interceptor | Automatic Action |
|---|---|---|
| `401 Unauthenticated` | `authInterceptor` | Attempts token refresh → on failure, shows `SessionExpiredSurface` |
| All gRPC error codes | `errorNotificationInterceptor` | Shows error notification toast |
| Network timeout | `errorNotificationInterceptor` | Shows "Network error" notification |
| `403 PermissionDenied` | `errorNotificationInterceptor` | Shows "Permission denied" toast |

---

## CORRECT: Let Interceptors Handle Errors

### In TanStack Query mutations

```typescript
// ✅ No onError needed — interceptor shows notification automatically
const { mutate: deleteDB } = useMutation({
  mutationFn: ({ name }: { name: string }) =>
    databaseServiceClientConnect.deleteDatabase({ name }),
  onSuccess: () => {
    router.push("/databases");
    toast.success("Database deleted");
  },
  // onError: NOT NEEDED — interceptor handles notification
});
```

### In direct API calls

```typescript
// ✅ No try/catch — interceptor handles errors
async function handleCreate() {
  const project = await projectServiceClientConnect.createProject({
    project: create(ProjectSchema, { title }),
    projectId: slug,
  });
  router.push(`/projects/${project.name}`);
}
```

---

## ONLY Catch When You Need Custom Behavior

```typescript
// ✅ OK: Custom error behavior (show inline error, not just toast)
try {
  await projectServiceClientConnect.createProject({ ... });
} catch (error) {
  if (error instanceof ConnectError && error.code === Code.AlreadyExists) {
    setSlugError("Project ID already taken");  // Inline error on form field
    return;  // Don't re-throw — interceptor already showed toast
  }
  throw error;  // Re-throw unknown errors so interceptor handles them
}
```

```typescript
// ✅ OK: Optimistic rollback
const previousValue = queryClient.getQueryData(["database", name]);
queryClient.setQueryData(["database", name], optimisticUpdate);

try {
  await databaseServiceClientConnect.updateDatabase({ database, updateMask });
} catch {
  queryClient.setQueryData(["database", name], previousValue);  // Rollback
  // Don't re-throw — interceptor already showed error toast
}
```

---

## Token Refresh Flow

```
Request gets 401
  → authInterceptor catches it
  → Acquires Web Lock (navigator.locks.request) — only ONE tab refreshes
  → Calls authServiceClientConnect.refresh({})
    → Success: BroadcastChannel posts "complete" to other tabs
              → Retry original request with new token
    → Failure: BroadcastChannel posts "failed"
              → Sets authStore.unauthenticatedOccurred = true
              → App.vue renders SessionExpiredSurface
              → User re-authenticates
```

**DO NOT implement custom token refresh logic.** It's centralized in `src/connect/refreshToken.ts`.

---

## Error Type Reference

```typescript
import { ConnectError, Code } from "@connectrpc/connect";

// Check specific error codes:
if (error instanceof ConnectError) {
  switch (error.code) {
    case Code.NotFound:        // 5 — resource doesn't exist
    case Code.AlreadyExists:   // 6 — duplicate creation
    case Code.PermissionDenied: // 7 — RBAC denied
    case Code.Unauthenticated: // 16 — not logged in
    case Code.InvalidArgument: // 3 — bad request data
    case Code.FailedPrecondition: // 9 — business rule violation
  }
}
```

---

## Anti-Patterns

```typescript
// ❌ NEVER: Double-handle errors (interceptor + component both show toast)
try {
  await api.doSomething();
} catch (error) {
  toast.error(error.message);  // Interceptor ALREADY shows toast → user sees 2 toasts
}

// ❌ NEVER: Swallow errors silently
try {
  await api.doSomething();
} catch {
  // nothing — user gets no feedback at all
}

// ❌ NEVER: Implement custom token refresh
if (error.code === Code.Unauthenticated) {
  await refreshToken();  // Already handled by interceptor
}
```
