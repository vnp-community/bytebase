# Solution: CR-ENT-005 — Restrict Copying Data

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-005                |
| **Solution**   | SOL-ENT-005               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Triển khai copy prevention trên SQL Editor result grid qua **backend policy** (`COPY_DATA` policy type) và **frontend enforcement** (event handlers + CSS). Policy scoped theo workspace/environment/project, kết hợp với Data Masking (CR-ENT-012) và Watermark (CR-ENT-021).

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `org_policy_service.go` | Add `COPY_DATA` policy type |
| **L4 — Service** | `sql_service.go` | Include copy policy in query response |
| **L9 — Enterprise** | `feature.go` | `FeatureRestrictCopyData` gate |
| **L1 — Presentation** | `SQLResultTable.vue` | Copy prevention handlers |
| **L1 — Presentation** | `PolicySettings.vue` | Copy policy configuration UI |

---

## 3. Chi tiết Implementation

### 3.1 Backend — Policy Definition

**Proto**:
```protobuf
message CopyDataPolicy {
  CopyDataRestriction restriction = 1;
}

enum CopyDataRestriction {
  COPY_DATA_RESTRICTION_UNSPECIFIED = 0;
  ALLOW = 1;
  RESTRICT = 2;
  RESTRICT_WITH_MASKING = 3;  // Block raw, allow masked data copy
}
```

**Policy resolution order**: Project > Environment > Workspace (most specific wins).

### 3.2 L4 — SQL Service

Trong response metadata của `SQLService.Query()`:
```go
// Include copy policy in query response metadata
metadata := &v1pb.QueryResponseMetadata{
    CopyRestriction: resolvedPolicy.Restriction,
}
```

### 3.3 L1 — Frontend Copy Prevention

```javascript
// SQLResultTable.vue
function setupCopyPrevention(policy) {
    if (policy === 'RESTRICT') {
        // 1. Disable Ctrl+C / Cmd+C
        resultGrid.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && (e.key === 'c' || e.key === 'a')) {
                e.preventDefault();
                showToast('Copy is restricted by workspace policy');
            }
        });

        // 2. Disable context menu
        resultGrid.addEventListener('contextmenu', (e) => e.preventDefault());

        // 3. Disable text selection via CSS
        resultGrid.style.userSelect = 'none';

        // 4. Disable drag selection
        resultGrid.addEventListener('selectstart', (e) => e.preventDefault());
    }
}
```

**Important**: Query text trong Monaco Editor vẫn copyable — chỉ restrict result data.

### 3.4 Audit Integration

Mỗi blocked copy attempt → audit log entry (nếu Full Audit Log enabled):
```json
{
  "action": "COPY_DATA_BLOCKED",
  "actor": "user@example.com",
  "resource": "instances/prod-pg/databases/myapp",
  "metadata": { "estimated_rows": 150 }
}
```

---

## 4. Database Changes

Không cần migration — `COPY_DATA` policy type lưu trong bảng `policy` hiện có dưới dạng JSONB payload.

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-012 (Data Masking) | `RESTRICT_WITH_MASKING` cho phép copy masked data |
| CR-ENT-021 (Watermark) | Defense-in-depth: cả 3 hoạt động đồng thời |
| CR-ENT-003 (Audit Log) | Copy attempts logged |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Policy backend + proto | Sprint 1 |
| 2 | Frontend copy prevention | Sprint 1 |
| 3 | Audit integration | Sprint 2 |
| 4 | Export restriction integration | Sprint 2 |
