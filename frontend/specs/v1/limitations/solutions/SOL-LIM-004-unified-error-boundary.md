# SOL-LIM-004 — Unified Error Boundary Layer

> **Resolves**: BUG-LIM-004 (Error Boundary Gaps — React Exceptions Không Được Catch)  
> **Type**: Architectural Change (Error Handling Layer)  
> **Priority**: High  
> **Effort**: Small (~3 ngày)  
> **Status**: Proposed

---

## 1. Mục Tiêu

1. Thêm React `ErrorBoundary` component wrapping mọi React page/component mount.
2. Thu hẹp ConnectError swallow filter — chỉ ignore codes đã có explicit handler.
3. Fix OAuth event listener leak và thêm global `unhandledrejection` handler.

---

## 2. Giải Pháp Kỹ Thuật

### 2.1 React Error Boundary Component (NEW)

```typescript
// src/react/components/ErrorBoundary.tsx — NEW FILE

import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  pageName?: string;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error(
      `[ErrorBoundary] React error in ${this.props.pageName || "unknown"}:`,
      error, errorInfo
    );
    this.props.onError?.(error, errorInfo);
    
    // Dispatch to Vue shell notification system
    window.dispatchEvent(
      new CustomEvent("bb.react-notification", {
        detail: {
          module: "bytebase",
          style: "CRITICAL",
          title: `Page Error: ${this.props.pageName || "Unknown"}`,
          description: error.message,
        },
      })
    );
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center h-full gap-4 p-8">
          <div className="text-red-500 text-lg font-medium">
            Something went wrong
          </div>
          <div className="text-gray-500 text-sm max-w-md text-center">
            {this.state.error?.message}
          </div>
          <button
            className="px-4 py-2 bg-accent text-white rounded hover:bg-accent-hover"
            onClick={() => this.setState({ hasError: false, error: null })}
          >
            Try Again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
```

### 2.2 Integrate ErrorBoundary vào mount.ts

```typescript
// src/react/mount.ts — UPDATED buildTree()

function buildTree(
  deps: ReactDeps,
  Component: ReactComponent,
  props?: any,
  pageName?: string
) {
  return deps.createElement(
    ErrorBoundary,            // ← NEW: Outermost wrapper
    { pageName },
    deps.createElement(
      deps.StrictMode,
      null,
      deps.createElement(
        deps.I18nextProvider,
        { i18n: deps.i18n },
        deps.createElement(Component, props)
      )
    )
  );
}

// Update callers to pass pageName
export async function mountReactPage(container, page, props) {
  const [deps, Component] = await Promise.all([loadCoreDeps(), loadPage(page)]);
  const root = deps.createRoot(container);
  root.render(buildTree(deps, Component, props, page)); // ← pass page name
  return root;
}
```

### 2.3 Refined ConnectError Filter (App.vue)

```typescript
// App.vue — UPDATED onErrorCaptured

// Only these codes are explicitly handled by interceptors
const INTERCEPTOR_HANDLED_CODES = [
  Code.Unauthenticated,  // authInterceptor → SessionExpiredSurface
  Code.PermissionDenied,  // authInterceptor → 403 page
  Code.NotFound,          // errorNotificationInterceptor → ignored
];

onErrorCaptured((error: unknown) => {
  if (
    error instanceof ConnectError &&
    INTERCEPTOR_HANDLED_CODES.includes(error.code)
  ) {
    return; // Already handled by specific interceptor logic
  }

  // All other errors (including INTERNAL, DATA_LOSS, UNAVAILABLE)
  // → show CRITICAL notification
  const err = error as { response?: unknown; stack?: string };
  if (!err.response) {
    notificationStore.pushNotification({
      module: "bytebase",
      style: "CRITICAL",
      title: "Internal error captured",
      description: isDev() ? err.stack : undefined,
    });
  }
  return true;
});
```

### 2.4 Fix OAuth Listener Leak

```typescript
// App.vue — UPDATED: Move OAuth handler to lifecycle

const handleOAuthUnknown = () => {
  notificationStore.pushNotification({
    module: "bytebase",
    style: "CRITICAL",
    title: t("oauth.unknown-event"),
  });
};

onMounted(() => {
  window.addEventListener("bb.oauth.unknown", handleOAuthUnknown);
  // Global async error handler for React
  window.addEventListener("unhandledrejection", handleUnhandledRejection);
});

onUnmounted(() => {
  window.removeEventListener("bb.oauth.unknown", handleOAuthUnknown);
  window.removeEventListener("unhandledrejection", handleUnhandledRejection);
});

function handleUnhandledRejection(event: PromiseRejectionEvent) {
  // Skip ConnectErrors (already handled by interceptors)
  if (event.reason instanceof ConnectError) return;
  
  console.error("[Unhandled Rejection]", event.reason);
  notificationStore.pushNotification({
    module: "bytebase",
    style: "CRITICAL",
    title: "Unexpected error",
    description: isDev() ? String(event.reason) : undefined,
  });
}
```

---

## 3. Thay Đổi Architecture/TDD

### 3.1 Cập nhật `specs/architecture.md` — Section 8.1 Component Architecture

Thêm `ErrorBoundary` vào Level 1 Primitives:

> | **React UI Kit** | `Button, Dialog, Combobox, Table, **ErrorBoundary**` |

### 3.2 Cập nhật `specs/technical-design-document.md` — Section 7 Error Handling

**Thay thế** Error Boundaries table:

> | Layer | Mechanism | Behavior |
> |---|---|---|
> | **React ErrorBoundary** | `ErrorBoundary` class component wrapping every React mount | Catch render errors → show fallback UI + dispatch notification to Vue shell |
> | **Vue Global** | `App.vue > onErrorCaptured` | Show CRITICAL notification for non-interceptor-handled errors |
> | **ConnectRPC** | `errorNotificationInterceptor` | Show user-friendly error notification (skip `NotFound`, `Unauthenticated`) |
> | **Auth** | `authInterceptor` | 401 → token refresh → retry reads only → SessionExpiredSurface |
> | **Route** | Navigation guard fallback | Unknown routes → 404 page |
> | **Async** | `unhandledrejection` handler | Catch React async errors → CRITICAL notification |

**Thay thế** ConnectError Filtering section:

> ### 7.2 ConnectError Filtering
> ```typescript
> // App.vue - Only codes with explicit interceptor handlers are silently passed
> const INTERCEPTOR_HANDLED_CODES = [Code.Unauthenticated, Code.PermissionDenied, Code.NotFound];
> if (error instanceof ConnectError && INTERCEPTOR_HANDLED_CODES.includes(error.code)) {
>   return; // Handled by interceptor chain
> }
> // All other gRPC errors (INTERNAL, DATA_LOSS, UNAVAILABLE, etc.) → notification
> ```

---

## 4. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| React runtime error → blank page | Always | Never (ErrorBoundary fallback) |
| gRPC INTERNAL error silent swallow | Yes | No (notification shown) |
| OAuth listener leak on HMR | Yes | No (proper cleanup) |
| Unhandled React async errors | Silent | Caught + notified |
| Error recovery without refresh | Impossible | "Try Again" button in ErrorBoundary |
