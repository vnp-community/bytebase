# LIM-004 — Frontend Dual Framework Complexity (Vue + React)

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | LIM-004                                    |
| Category       | Frontend / Developer Experience            |
| Severity       | MEDIUM                                     |
| Affected Layer | L1 (Presentation)                          |
| Source Files   | `frontend/src/`, `frontend/src/react/`     |

---

## Mô tả

Frontend đang trong quá trình migration từ **Vue 3** sang **React 19**, tạo ra kiến trúc hybrid với hai framework chạy đồng thời.

## Chi tiết hạn chế

### 1. Dual State Management

```
Vue 3 Layer → Pinia 3.x (store/modules/)
React 19 Layer → Zustand 5.x (react/hooks/)
Bridge Layer → useVueState() hook (useSyncExternalStore)
```

- **Hai hệ thống state** phải đồng bộ qua bridge layer.
- `useVueState()` subscribes to Vue reactive state từ React — phức tạp và dễ gây bugs.
- State changes phải propagate qua framework boundary → potential race conditions.

### 2. Dual UI Library

| Framework | UI Library      | Styling               |
|-----------|-----------------|------------------------|
| Vue 3     | Naive UI 2.44   | Tailwind CSS v4        |
| React 19  | Base UI         | Tailwind CSS v4        |

- Hai bộ UI components → inconsistency trong look & feel.
- Duplicate component logic (buttons, modals, forms, etc.).
- Tăng bundle size đáng kể.

### 3. Dual i18n System

```
vue-i18n → Vue components
react-i18next → React components
```

- Translation keys phải maintain ở hai nơi.
- Risk: translation mismatches giữa Vue và React components.

### 4. Build Complexity

```json
// Multiple TypeScript configs
tsconfig.app.json       // Vue app
tsconfig.react.json     // React components
tsconfig.node.json      // Node scripts
tsconfig.vitest.json    // Testing
```

- Vite build phải handle cả Vue SFC và React JSX.
- Type system phải bridge giữa Vue refs/reactive và React hooks/state.

### 5. Router Complexity

- Vue Router 4 handles routing.
- React components phải mount bên trong Vue routes.
- Deep linking vào React pages phải đi qua Vue router.

## Impact

- **Developer onboarding**: Cần kiến thức cả Vue và React.
- **Bundle size**: Estimated ~30-40% larger do dual runtime.
- **Testing**: Test infrastructure phải hỗ trợ cả hai framework.
- **Bug surface**: Bridge layer là nguồn bugs khó debug.

## Migration Status

| Aspect               | Status                        |
|-----------------------|-------------------------------|
| New components        | React 19 + Base UI            |
| Legacy components     | Vue 3 + Naive UI (maintenance)|
| Router                | Vue Router (unchanged)        |
| State migration       | Partial (bridge active)       |
| Estimated completion  | Unknown                       |

## Khuyến nghị

1. Xác định timeline migration rõ ràng.
2. Tránh viết component mới bằng Vue — chỉ React.
3. Gradual migration route-by-route.
4. Bundle splitting để giảm impact size.
