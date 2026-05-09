# Solution: CR-LIM-004 — Frontend Framework Unification

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-LIM-004                               |
| **Solution ID**| SOL-LIM-004                              |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-09                               |
| **Arch Refs**  | L1 (Presentation Layer), L2 (API Gateway) |
| **TDD Refs**   | §10 Frontend Architecture, §14 Trade-offs |

---

## 1. Solution Overview

### 1.1 Approach Summary

**Proposed Architecture Change**: Chuyển từ monolithic SPA (Vue+React embedded in Go binary) sang **standalone CSR (Client-Side Rendering)** frontend, tách biệt hoàn toàn khỏi backend Go binary. Đây là thay đổi kiến trúc quan trọng ở L1 và L2.

4-phase migration:

1. **Phase A — Architecture Decoupling**: Tách frontend thành standalone Vite SPA, served qua Nginx/CDN thay vì embedded trong Go binary
2. **Phase B — React Shell + Router Migration**: React Router v7 replaces Vue Router, React layout shell wraps legacy Vue pages
3. **Phase C — Route-by-Route Component Migration**: Migrate Vue components sang React theo priority tiers
4. **Phase D — Vue Removal + Bundle Cleanup**: Xóa Vue runtime, Pinia, Naive UI

### 1.2 Architectural Change Proposal

> **⚠️ PROPOSED ARCHITECTURE CHANGE — L1/L2 Boundary**

**Current Architecture** (TDD §10, Architecture L1):
```
Go Binary
  ├── Backend (API Server)
  └── Embedded SPA (server_frontend_embed.go)
       ├── Vue 3 + React 19 (dual framework)
       └── Served via Echo fallback route (/* → index.html)
```

**Proposed Architecture**:
```
┌─────────────────────┐     ┌──────────────────────────────┐
│  Nginx / CDN        │     │  Go Binary (Backend Only)    │
│  ├── Static assets  │────▶│  ├── ConnectRPC API          │
│  ├── index.html     │     │  ├── gRPC-Gateway REST       │
│  └── SPA routing    │     │  ├── LSP WebSocket           │
│      (try_files)    │     │  ├── MCP SSE                 │
└─────────────────────┘     │  └── OAuth2/SCIM/Stripe      │
       React 19 SPA          └──────────────────────────────┘
       (standalone)                Backend API Server
```

**Rationale**:
1. **Independent deployment** — Frontend deployable mà không cần rebuild Go binary (hiện ~500MB+ build time)
2. **CDN caching** — Static assets served from edge, giảm latency
3. **Phù hợp Vue→React migration** — Tách biệt cho phép thay đổi frontend framework mà không ảnh hưởng backend release cycle
4. **Consistent with VNP platform** — Dify và Flowise đã migrate sang CSR architecture (conversations fc91d67c, e593ef38)
5. **Loại bỏ `server_frontend_embed.go`** — Giảm Go binary size ~40MB

### 1.3 Impact Assessment

| Aspect | Current (Embedded) | Proposed (Standalone CSR) |
|--------|--------------------|-----------------------------|
| Deployment | Single binary | Backend binary + Nginx/CDN |
| Build time | ~8-12min (Go+Frontend) | ~3min backend, ~2min frontend |
| Frontend deploy | Requires full binary rebuild | Independent static deploy |
| CORS | Not needed (same origin) | Required (cross-origin API) |
| Auth | Cookie (HttpOnly, SameSite) | JWT Bearer token |
| CSP | Hash-based (vite plugin) | Standard CSP |

---

## 2. Detailed Technical Design

### 2.1 Phase A — Architecture Decoupling (L1/L2 Boundary Change)

#### 2.1.1 Frontend Build Output

**File**: `frontend/vite.config.ts` (modify)

```typescript
export default defineConfig({
  build: {
    outDir: 'dist',
    // Assets output structure for CDN/Nginx
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        // Hashed filenames for cache busting
        entryFileNames: 'assets/[name].[hash].js',
        chunkFileNames: 'assets/[name].[hash].js',
        assetFileNames: 'assets/[name].[hash].[ext]',
      },
    },
  },
  // Runtime config injection (deploy-time, not build-time)
  define: {
    // Will be overridden by window.__ENV__ at runtime
  },
})
```

#### 2.1.2 Runtime Environment Configuration

**File**: `frontend/public/env-config.js` (new — injected at deploy time)

```javascript
// This file is generated/overridden at deploy time by Nginx/Docker
// Enables deploy-time API URL configuration without rebuild
window.__ENV__ = {
  API_URL: '',  // Empty = same origin, or 'https://api.bytebase.example.com'
  AUTH_MODE: 'token',  // 'token' (Bearer JWT) or 'cookie' (legacy)
};
```

**File**: `frontend/src/config/env.ts` (new)

```typescript
// 3-tier config: Runtime > Build-time > Default
interface EnvConfig {
  apiUrl: string;
  authMode: 'token' | 'cookie';
}

export function getEnvConfig(): EnvConfig {
  const runtime = (window as any).__ENV__ || {};
  return {
    apiUrl: runtime.API_URL || import.meta.env.VITE_API_URL || '',
    authMode: runtime.AUTH_MODE || import.meta.env.VITE_AUTH_MODE || 'cookie',
  };
}
```

#### 2.1.3 Auth Mode Migration (Cookie → Bearer Token)

**File**: `frontend/src/auth/token-manager.ts` (new)

```typescript
// Token-based auth for standalone CSR mode.
// Stores JWT in memory (access) + localStorage (refresh).
// Compatible with existing backend JWT (TDD §11.1).
class TokenManager {
  private accessToken: string | null = null;
  private refreshTimer: number | null = null;

  setTokens(access: string, refresh: string) {
    this.accessToken = access;
    localStorage.setItem('bb_refresh_token', refresh);
    this.scheduleRefresh(access);
  }

  getAccessToken(): string | null {
    return this.accessToken;
  }

  // Attach token to ConnectRPC transport
  createTransport(baseUrl: string): Transport {
    return createConnectTransport({
      baseUrl,
      interceptors: [
        (next) => async (req) => {
          if (this.accessToken) {
            req.header.set('Authorization', `Bearer ${this.accessToken}`);
          }
          return next(req);
        },
      ],
    });
  }

  async refreshAccessToken(): Promise<void> {
    const refreshToken = localStorage.getItem('bb_refresh_token');
    if (!refreshToken) throw new Error('No refresh token');

    const response = await fetch(`${getEnvConfig().apiUrl}/v1/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken }),
    });

    if (!response.ok) {
      this.clearTokens();
      window.location.href = '/auth/login';
      return;
    }

    const data = await response.json();
    this.setTokens(data.accessToken, data.refreshToken);
  }

  private scheduleRefresh(token: string) {
    const payload = JSON.parse(atob(token.split('.')[1]));
    const expiresIn = payload.exp * 1000 - Date.now();
    const refreshIn = expiresIn - 60000; // Refresh 1 min before expiry
    if (this.refreshTimer) clearTimeout(this.refreshTimer);
    this.refreshTimer = window.setTimeout(() => this.refreshAccessToken(), refreshIn);
  }
}

export const tokenManager = new TokenManager();
```

#### 2.1.4 Backend CORS Support

**File**: `backend/server/echo_routes.go` (modify)

```go
// Add CORS middleware for standalone CSR mode
func (s *Server) configureCORS(e *echo.Echo) {
    allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
    if allowedOrigins == "" {
        return // No CORS needed (embedded mode)
    }

    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins:     strings.Split(allowedOrigins, ","),
        AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.PATCH, echo.OPTIONS},
        AllowHeaders:     []string{"Authorization", "Content-Type", "Connect-Protocol-Version", "X-Auth-Mode"},
        AllowCredentials: true,
        MaxAge:           86400,
    }))
}
```

#### 2.1.5 Backend Auth Mode Extension

**File**: `backend/api/auth/auth.go` (modify)

```go
// Support both Cookie and Bearer token auth modes.
// Auth mode determined by X-Auth-Mode header or CORS origin.
func (i *AuthInterceptor) extractToken(ctx context.Context, req connect.AnyRequest) (string, error) {
    // Priority 1: Authorization Bearer header (token mode)
    if auth := req.Header().Get("Authorization"); auth != "" {
        if strings.HasPrefix(auth, "Bearer ") {
            return strings.TrimPrefix(auth, "Bearer "), nil
        }
    }

    // Priority 2: Cookie (legacy embedded mode)
    if cookie := req.Header().Get("Cookie"); cookie != "" {
        // Parse access-token cookie (existing logic)
        return extractCookieToken(cookie, "access-token"), nil
    }

    return "", errors.New("no authentication token found")
}
```

#### 2.1.6 Auth Service Token Response

**File**: `backend/api/v1/auth_service.go` (modify Login response)

```go
func (s *AuthService) Login(ctx context.Context, req *connect.Request[v1pb.LoginRequest]) (*connect.Response[v1pb.LoginResponse], error) {
    // ... existing credential verification ...

    // Generate JWT (existing logic)
    accessToken, err := s.generateAccessToken(user)
    refreshToken, err := s.generateRefreshToken(user)

    resp := connect.NewResponse(&v1pb.LoginResponse{
        User: convertUser(user),
    })

    authMode := req.Header().Get("X-Auth-Mode")
    if authMode == "token" {
        // Token mode: return tokens in response body
        resp.Msg.AccessToken = accessToken
        resp.Msg.RefreshToken = refreshToken
    } else {
        // Cookie mode: set HttpOnly cookies (existing behavior)
        resp.Header().Set("Set-Cookie", fmt.Sprintf("access-token=%s; HttpOnly; SameSite=Lax; Path=/", accessToken))
    }

    return resp, nil
}
```

#### 2.1.7 Nginx Configuration

**File**: `deploy/nginx/bytebase-frontend.conf` (new)

```nginx
server {
    listen 80;
    server_name bytebase.example.com;

    root /usr/share/nginx/html;
    index index.html;

    # SPA routing — all paths serve index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Static assets — long cache
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API proxy to backend
    location /bytebase.v1. {
        proxy_pass http://bytebase-backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location /v1/ {
        proxy_pass http://bytebase-backend:8080;
    }

    location /lsp {
        proxy_pass http://bytebase-backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location /mcp/ {
        proxy_pass http://bytebase-backend:8080;
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_buffering off;
    }
}
```

### 2.2 Phase B — React Shell + Router Migration

#### 2.2.1 React App Entry Point

**File**: `frontend/src/main.tsx` (new — replaces `main.ts`)

```typescript
import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router';
import { router } from './router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import './index.css';

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('app')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </React.StrictMode>
);
```

#### 2.2.2 React Router Configuration

**File**: `frontend/src/router.tsx` (new)

```typescript
import { createBrowserRouter } from 'react-router';
import { RootLayout } from './layouts/RootLayout';
import { AuthGuard } from './auth/AuthGuard';

// Lazy-loaded route modules
const SQLEditor = React.lazy(() => import('./pages/sql-editor/SQLEditorPage'));
const SchemaEditor = React.lazy(() => import('./pages/schema-editor/SchemaEditorPage'));
const IssuePage = React.lazy(() => import('./pages/issue/IssuePage'));
const InstanceList = React.lazy(() => import('./pages/instance/InstanceListPage'));
const DatabaseList = React.lazy(() => import('./pages/database/DatabaseListPage'));
const ProjectPage = React.lazy(() => import('./pages/project/ProjectPage'));
const SettingsPage = React.lazy(() => import('./pages/settings/SettingsPage'));
const LoginPage = React.lazy(() => import('./pages/auth/LoginPage'));

// Migration bridge: Vue pages wrapped in React component
const VueBridge = React.lazy(() => import('./bridge/VueBridgePage'));

export const router = createBrowserRouter([
  {
    path: '/auth',
    children: [
      { path: 'login', element: <LoginPage /> },
      { path: 'signup', element: <LoginPage mode="signup" /> },
    ],
  },
  {
    path: '/',
    element: <AuthGuard><RootLayout /></AuthGuard>,
    children: [
      // Tier 1 — Migrated first (highest interaction)
      { path: 'sql-editor/*', element: <SQLEditor /> },
      { path: 'schema-editor/:database', element: <SchemaEditor /> },

      // Tier 2 — Core workflow
      { path: 'projects/:project/issues/*', element: <IssuePage /> },
      { path: 'projects/:project/plans/*', element: <IssuePage /> },

      // Tier 3 — CRUD pages
      { path: 'instances/*', element: <InstanceList /> },
      { path: 'databases/*', element: <DatabaseList /> },
      { path: 'projects/*', element: <ProjectPage /> },

      // Tier 4 — Settings/Admin
      { path: 'settings/*', element: <SettingsPage /> },

      // Bridge: unmigrated Vue pages (catch-all during migration)
      { path: '*', element: <VueBridge /> },
    ],
  },
]);
```

#### 2.2.3 Vue Bridge Component (Temporary)

**File**: `frontend/src/bridge/VueBridgePage.tsx` (new — temporary during migration)

```typescript
import { useEffect, useRef } from 'react';
import { useLocation } from 'react-router';
import { createApp } from 'vue';
import { createPinia } from 'pinia';
import LegacyApp from '../LegacyApp.vue';

// Mounts the Vue app inside a React container for unmigrated routes.
// This is TEMPORARY — removed when all routes are migrated.
export default function VueBridgePage() {
  const containerRef = useRef<HTMLDivElement>(null);
  const location = useLocation();
  const vueAppRef = useRef<ReturnType<typeof createApp> | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Mount Vue app
    const app = createApp(LegacyApp, { initialPath: location.pathname });
    app.use(createPinia());
    app.mount(containerRef.current);
    vueAppRef.current = app;

    return () => {
      app.unmount();
      vueAppRef.current = null;
    };
  }, []);

  // Sync React router changes to Vue router
  useEffect(() => {
    if (vueAppRef.current) {
      // Push route change to Vue router
      const vueRouter = vueAppRef.current.config.globalProperties.$router;
      vueRouter?.push(location.pathname + location.search);
    }
  }, [location]);

  return <div ref={containerRef} className="vue-bridge-container h-full" />;
}
```

### 2.3 Phase C — Component Migration Pattern

#### 2.3.1 State Migration: Pinia → Zustand

```typescript
// BEFORE: Pinia store (Vue)
// frontend/src/store/modules/instance.ts
export const useInstanceStore = defineStore('instance', {
  state: () => ({ instances: [] as Instance[], loading: false }),
  actions: {
    async fetchInstances() { ... }
  }
});

// AFTER: Zustand store (React)
// frontend/src/stores/instance.ts
import { create } from 'zustand';
import { instanceServiceClient } from '../grpc/clients';

interface InstanceState {
  instances: Instance[];
  loading: boolean;
  fetchInstances: () => Promise<void>;
}

export const useInstanceStore = create<InstanceState>((set) => ({
  instances: [],
  loading: false,
  fetchInstances: async () => {
    set({ loading: true });
    const resp = await instanceServiceClient.listInstances({});
    set({ instances: resp.instances, loading: false });
  },
}));
```

#### 2.3.2 Migration Checklist Per Route

Each route migration follows:

```
□ Create React page component
□ Create Zustand store (if Pinia equivalent exists)
□ Map Naive UI components → Base UI equivalents
□ Migrate vue-i18n $t() → useTranslation() t()
□ Write visual regression test (screenshot comparison)
□ Write E2E test (Playwright)
□ Update router.tsx (remove VueBridge catch-all for this path)
□ Remove Vue component files
```

### 2.4 Phase D — Vue Removal

#### 2.4.1 Dependencies to Remove

```json
// package.json changes
{
  "remove": [
    "vue", "@vue/runtime-dom", "@vue/compiler-sfc",
    "vue-router", "pinia",
    "naive-ui", "@css-render/vue3-ssr",
    "vue-i18n",
    "@vitejs/plugin-vue",
    "vue-tsc",
    "@vueuse/core"
  ]
}
```

#### 2.4.2 Build Config Cleanup

```typescript
// vite.config.ts — AFTER Vue removal
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],  // No more vue() plugin
  build: {
    target: 'es2022',
    outDir: 'dist',
  },
});
```

#### 2.4.3 Backend Cleanup

```go
// REMOVE these files:
// backend/server/server_frontend_embed.go  — No longer embedding SPA
// backend/server/server_frontend_routes.go — No longer serving SPA routes

// MODIFY backend/server/echo_routes.go:
// Remove /* fallback to embedded SPA
// Add /healthz, /metrics as top-level routes only
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| **L1 (Presentation)** | **CRITICAL** | Full framework swap, deployment model change |
| **L2 (API Gateway)** | **HIGH** | CORS middleware, auth mode extension, remove SPA embedding |
| **L3 (Security)** | **MEDIUM** | Bearer token auth path, CSP simplification |
| L4-L10 | **NONE** | Backend logic unchanged |

### 3.1 Architecture Dependency Change

```
BEFORE:
  L1 ──(embedded in)──► L2 (Go binary serves SPA)
  L1 → L2 (same-origin API calls, cookie auth)

AFTER:
  L1 ──(deployed separately)──► Nginx/CDN
  L1 → L2 (cross-origin API calls, Bearer token auth)
  L2 has CORS middleware for L1 origins
```

---

## 4. Migration Safety Plan

### 4.1 Rollout Steps

```
Phase A (Sprint 1-2):
  1. Add CORS middleware to backend (feature-flagged)
  2. Add Bearer token auth support alongside cookie
  3. Create frontend env-config.js + TokenManager
  4. Create Nginx config + Docker setup
  5. Deploy standalone frontend pointing to existing backend
  6. Validate: both embedded and standalone modes work

Phase B (Sprint 3-4):
  7. Create React Router shell + AuthGuard
  8. Create VueBridge for unmigrated routes
  9. Migrate Tier 4 (Settings — lowest risk)
  10. Validate: Settings page fully React

Phase C (Sprint 5-10):
  11. Migrate Tier 3 (Instance/DB/Project CRUD)
  12. Migrate Tier 2 (Issue/Plan/Rollout workflow)
  13. Migrate Tier 1 (SQL Editor, Schema — highest risk)
  14. Each tier: visual regression + E2E tests before merge

Phase D (Sprint 11-12):
  15. Remove VueBridge component
  16. Remove Vue dependencies from package.json
  17. Remove server_frontend_embed.go from Go binary
  18. Remove vue plugin from vite.config.ts
  19. Final bundle size validation (target: ≥ 30% reduction)
```

### 4.2 Rollback Plan

```
Phase A: Remove CORS middleware, revert to embedded-only
Phase B: VueBridge catches all routes → effectively Vue-only
Phase C: Per-route rollback — re-add VueBridge catch for specific path
Phase D: Cannot rollback (Vue removed) — only proceed after full E2E pass
```

---

## 5. Performance Validation

| Metric | Current (Vue+React embedded) | Target (React standalone) |
|--------|------|--------|
| Bundle size (gzipped) | ~3.2MB | ≤ 2.0MB (≥ 37% reduction) |
| Initial load (LCP) | ~2.5s | ≤ 1.5s (CDN + code split) |
| Go binary size | ~180MB | ~140MB (no embedded SPA) |
| Full build time | ~12min | ~3min backend + 2min frontend |
| Frontend deploy time | ~12min (full rebuild) | ~30s (static file sync) |
