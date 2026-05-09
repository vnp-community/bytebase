# LIM-006 — Enterprise Feature Gating & Pricing Restrictions

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | LIM-006                                    |
| Category       | Licensing / Business                       |
| Severity       | HIGH (for self-hosted users)               |
| Affected Layer | L9 (Enterprise)                            |
| Source Files   | `backend/enterprise/license.go`, `backend/enterprise/plan.yaml` |

---

## Mô tả

Bytebase sử dụng BSL license kết hợp feature gating qua license JWT. Nhiều tính năng critical bị khóa sau Enterprise plan.

## Chi tiết hạn chế

### 1. Instance & Seat Limits

| Plan       | Instances | Seats     |
|------------|-----------|-----------|
| FREE       | 10        | 20        |
| TEAM       | 10        | Unlimited |
| ENTERPRISE | Unlimited | Unlimited |

FREE plan giới hạn 10 instances — không đủ cho production multi-env setup.

### 2. Security Features Locked Behind Enterprise

Approval Workflow, Data Masking, Full Audit Log, 2FA, SSO (OIDC/SAML/LDAP), Custom Roles, SCIM, External Secret Manager — tất cả yêu cầu Enterprise.

### 3. Audit Log Retention

- FREE: Không có access
- TEAM: 7 ngày
- ENTERPRISE: Unlimited

TEAM 7 ngày không đủ cho compliance (thường 90 ngày - 1 năm).

## Khuyến nghị

1. Tăng FREE instance limit lên 20-30.
2. Đưa 2FA vào TEAM plan (security baseline).
3. Tăng TEAM audit log retention lên 30 ngày.
