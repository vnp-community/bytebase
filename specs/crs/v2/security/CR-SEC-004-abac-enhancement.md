# Change Request: ABAC Enhancement — Context-Aware Access Control

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-004                                               |
| **Feature ID**     | SEC-14, SEC-20                                           |
| **Title**          | Attribute-Based Access Control (ABAC) Enhancement        |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Mở rộng hệ thống IAM hiện tại (CEL-based RBAC) thành ABAC — cho phép quyết định access dựa trên context attributes: thời gian, IP, environment tier, risk score, data classification, và user attributes.

### 1.2 Bối cảnh
Bytebase sử dụng CEL-based IAM (PRD: Authorization layer) với Custom Roles (SEC-14) và JIT Access (SEC-20). ABAC mở rộng khả năng bằng context-aware policies cho enterprise environments.

---

## 2. Yêu cầu chức năng

### FR-001: Context-Aware Policy Engine
- Extend CEL policy engine với context attributes:
  ```cel
  // Chỉ cho phép deploy lên production trong business hours
  request.environment.tier == "PRODUCTION" 
    && time.now().hour >= 9 && time.now().hour <= 17
    && time.now().dayOfWeek >= 1 && time.now().dayOfWeek <= 5
  
  // Chỉ cho phép access từ corporate network
  request.source_ip in ["10.0.0.0/8", "172.16.0.0/12"]
  
  // Block high-risk changes ngoài giờ hành chính
  change.risk_level != "HIGH" || user.on_call == true
  ```
- **Acceptance Criteria**:
  - AC-1: Time-based access rules (business hours, maintenance windows)
  - AC-2: Network-based rules (IP ranges, VPN detection)
  - AC-3: Environment-tier-based rules (prod vs non-prod)
  - AC-4: Risk-level-based rules (link với SEC-08 Risk Assessment)
  - AC-5: Data classification-based rules (link với SEC-16)
  - AC-6: Policy evaluation latency < 5ms per request

### FR-002: Policy Composition & Inheritance
- Hierarchical policies: Workspace → Project → Environment
- **Acceptance Criteria**:
  - AC-1: Child policies inherit and can restrict parent policies
  - AC-2: Explicit deny overrides allow
  - AC-3: Policy conflict resolution: most restrictive wins
  - AC-4: Policy dry-run mode (evaluate without enforcing)

### FR-003: Emergency Access Override
- Break-glass procedure cho emergency situations
- **Acceptance Criteria**:
  - AC-1: Emergency override requires MFA re-authentication
  - AC-2: Override action logged với mandatory justification
  - AC-3: Auto-notification tới all workspace admins
  - AC-4: Override expires after configurable duration (max 4h)
  - AC-5: Post-incident review workflow triggered

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| IAM Component                | `backend/component/iam/`                    | ABAC policy engine extension                |
| ACL Interceptor              | `backend/api/interceptor/acl.go`            | Context attribute injection                 |
| Policy Service               | `backend/api/v1/org_policy_service.go`      | ABAC policy CRUD                            |
| Emergency Access (new)       | `backend/api/v1/emergency_access.go`        | Break-glass procedure                       |
| Policy Editor UI (new)       | `frontend/src/views/PolicyEditor.vue`       | Visual policy builder                       |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Deploy to prod outside business hours                | Access denied                    |
| TC-002  | Access from non-corporate IP                         | Access denied per policy         |
| TC-003  | High-risk change by non-on-call user                 | Requires additional approval     |
| TC-004  | Emergency override with MFA                          | Access granted, logged           |
| TC-005  | Policy dry-run mode                                  | Evaluation result without enforce|
| TC-006  | Child policy restricts parent                        | More restrictive applied         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Context attribute injection          | Sprint 1       |
| Phase 2 | CEL engine ABAC extension            | Sprint 2       |
| Phase 3 | Policy composition + inheritance     | Sprint 3       |
| Phase 4 | Emergency access + UI                | Sprint 4       |
