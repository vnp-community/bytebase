# ISS-AI-003 — God Components Vượt Quá 1000+ LOC Gây Lỗi Khi AI Chỉnh Sửa

> **Category**: Code Complexity  
> **Severity**: High  
> **Impact**: Code Editing, Bug Fixing, Refactoring  
> **Affected Area**: `src/react/pages/`, `src/react/components/`

---

## 1. Mô Tả Vấn Đề

Nhiều file component vượt quá 1000 dòng — mỗi file chứa rendering logic, business logic, state management, và form handling trong cùng một function component. Đây là anti-pattern nghiêm trọng khi sử dụng AI.

### 1.1 Top God Components (>1000 LOC)

| File | LOC | Chức năng |
|---|---|---|
| `ProjectSyncSchemaPage.tsx` | 2,196 | Multi-database schema synchronization |
| `IDPsPage.tsx` | 2,104 | Identity Provider CRUD + complex form |
| `DataSourceForm.tsx` | 1,997 | Data source configuration (15+ DB engines) |
| `MembersPage.tsx` | 1,993 | Member management + role assignment |
| `InstanceFormBody.tsx` | 1,913 | Instance creation form (engine-specific fields) |
| `PlanDetailChangesBranch.tsx` | 1,791 | Plan change review UI |
| `EnvironmentsPage.tsx` | 1,670 | Environment CRUD + ordering |
| `IDPDetailPage.tsx` | 1,625 | IDP edit (OIDC, OAuth2, LDAP form fields) |
| `ExprEditor.tsx` | 1,584 | CEL expression editor |
| `SemanticTypesPage.tsx` | 1,431 | Semantic type management |
| `InstancesPage.tsx` | 1,418 | Instance list + filters |
| `SheetTree.tsx` | 1,410 | Worksheet tree navigation |
| `ProjectMaskingExemptionPage.tsx` | 1,401 | Masking exemption rules |
| `ProjectSettingsPage.tsx` | 1,350 | Project settings (multiple tabs) |
| `TableDetailDialog.tsx` | 1,341 | Table schema detail view |
| `AgentWindow.tsx` | 1,312 | AI agent floating window |
| `ProjectPlanDashboardPage.tsx` | 1,251 | Plan dashboard with filters |
| `ConnectionPane.tsx` | 1,208 | SQL Editor connection panel |

### 1.2 Tại Sao Đây Là Vấn Đề Cho AI

1. **Context window saturation**: Một file 2000 LOC chiếm ~50% context window của AI, không còn chỗ cho related files (stores, types, utils).
2. **Edit precision degradation**: AI dễ target sai vị trí khi edit trong file lớn — ví dụ thay đổi wrong handler trong component có 20+ event handlers.
3. **Implicit dependencies**: God component thường import 20+ modules → AI khó trace dependency graph.
4. **State explosion**: Single component với 15+ `useState` calls → AI dễ nhầm lẫn state variable names.

## 2. Ví Dụ Cụ Thể: `MembersPage.tsx` (1,993 LOC)

```
MembersPage.tsx chứa:
- 16 useVueState() calls (cross-framework state bridges)
- ~15 useState/useCallback hooks
- CRUD operations cho members, groups, roles
- Permission checking logic
- Table rendering với sort/filter
- Multiple dialog/sheet states
- Search và pagination logic
```

AI muốn "thêm một filter mới" cần:
1. Đọc toàn bộ 1993 LOC để hiểu existing filter logic
2. Tìm đúng vị trí insert trong 1 mega-function
3. Hiểu 16 `useVueState` dependencies
4. Không break existing table rendering

→ **Xác suất lỗi cao**.

## 3. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **Add feature** | AI phải đọc toàn bộ god component để hiểu context, dễ quên state dependencies |
| **Fix bug** | AI khó isolate root cause trong 2000 LOC monolith |
| **Refactor** | AI không thể safely extract sub-components mà không hiểu toàn bộ state coupling |
| **Review** | AI code review accuracy giảm mạnh ở files >800 LOC |

## 4. Khuyến Nghị Giảm Thiểu

1. **Enforce component size limit**: Tạo lint rule cảnh báo khi component >500 LOC.
2. **Extract hooks**: Di chuyển business logic vào custom hooks (e.g. `useMembersCRUD`, `useMembersFilter`).
3. **Compositional split**: Tách god components thành `*Container.tsx` (logic) + `*View.tsx` (rendering).
4. **AI chunking strategy**: Khi AI edit, provide chỉ relevant section (function, handler) thay vì entire file.
