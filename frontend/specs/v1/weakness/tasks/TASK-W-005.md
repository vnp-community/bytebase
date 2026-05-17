# TASK-W-005: React Error Boundary

> **Source**: SOL-WEAK-001 §3.3 | **Priority**: P1 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/react/components/ReactErrorBoundary.tsx`

## What
Create React class component that catches render errors, shows fallback UI with retry, and logs stack in dev.

## Implementation
Create `src/react/components/ReactErrorBoundary.tsx` — full code in SOL-WEAK-001 §3.3:
- Props: `{ pageName: string; children: ReactNode }`
- State: `{ hasError: boolean; error: Error | null }`
- `getDerivedStateFromError` → set error state
- `componentDidCatch` → `console.error` with pageName and componentStack
- Fallback UI: centered error message, retry button, dev-only stack trace

## AC
- [ ] File created at `src/react/components/ReactErrorBoundary.tsx`
- [ ] Catches render errors from children
- [ ] Shows retry button that resets error state
- [ ] Shows stack trace only in dev mode (`import.meta.env.DEV`)
- [ ] Exports named class `ReactErrorBoundary`
