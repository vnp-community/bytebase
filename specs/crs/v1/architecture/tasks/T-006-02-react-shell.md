# T-006-02: React Shell (Layout/Nav)

| Field | Value |
|---|---|
| **Task ID** | T-006-02 |
| **Solution** | SOL-ARCH-006 |
| **Priority** | P3 |
| **Depends On** | T-006-01 |
| **Target Files** | `frontend/src/react/layout/AppLayout.tsx`, `frontend/src/react/layout/Sidebar.tsx`, `frontend/src/react/layout/Header.tsx` |
| **Type** | New files |

---

## Objective

Create React layout shell (sidebar, header, content area) that replaces Vue layout. All shared chrome becomes React. Vue pages rendered inside React layout wrapper.

## Implementation

```typescript
// AppLayout.tsx
export function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="app-layout">
      <Sidebar />
      <div className="main-content">
        <Header />
        <div className="page-content">{children}</div>
      </div>
    </div>
  );
}
```

## Acceptance Criteria

- [ ] `AppLayout` with Sidebar + Header + content area
- [ ] Responsive design matching existing Vue layout
- [ ] Navigation items match current sidebar
- [ ] Uses Zustand auth store for user info
- [ ] `npm run build` passes
