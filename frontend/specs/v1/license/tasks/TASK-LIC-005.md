# TASK-LIC-005: E2E Verification — Enterprise Features

> **Source**: SOL-LIC-001 §7 | **Priority**: P1 | **Effort**: 2h  
> **Status**: DONE | **Deps**: LIC-001, LIC-002, LIC-003, LIC-004

## Scope
- **VERIFY** All Enterprise-gated pages and features across the application

## What
Manual E2E verification đảm bảo tất cả Enterprise features được mở khóa và hoạt động đúng sau khi apply LIC-001~004. Kiểm tra cả Vue và React components, router guards, và banners.

## Test Matrix

### Security & Access Control
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 1 | Custom Roles management | `/setting/custom-roles` | Page loads, role CRUD works |
| 2 | Risk Assessment config | `/setting/risk-assessment` | Page loads, no paywall |
| 3 | Custom Approval workflow | `/setting/custom-approval` | Page loads, rule CRUD works |
| 4 | Enterprise SSO (OIDC) | `/setting/idps` | Add IdP button works, no sparkles |
| 5 | 2FA enforcement | `/setting/general` → MFA toggle | Toggle works, guard enforces correctly |
| 6 | SCIM config | `/setting/scim` | Page loads |
| 7 | Password Restrictions | `/setting/security` | Config available |
| 8 | Directory Sync | `/setting/directory-sync` | Page loads |

### Data Security
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 9 | Data Masking | Project → Masking Exemption | Page loads, no sparkles badge |
| 10 | Data Classification | Project → Settings | Classification UI available |
| 11 | External Secret Manager | Instance → Data Source config | Option available, no lock icon |
| 12 | Token Duration Control | `/setting/security` | Slider available |

### SQL Editor
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 13 | Batch Query | SQL Editor multi-tab | No feature gate |
| 14 | Read-Only Connection | Instance → Data Source | Option available |
| 15 | Restrict Copying Data | SQL Editor result panel | Copy restriction toggle works |
| 16 | Query Policy | Project settings | Config available |

### Administration
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 17 | Environment Tiers | `/setting/environments` | Tier labels visible |
| 18 | Dashboard Announcement | `/setting/general` | Announcement section visible |
| 19 | Custom Logo | `/setting/branding` | Upload available |
| 20 | Watermark | `/setting/general` | Toggle available |
| 21 | Database Groups | Project → Database Groups | Create group works |

### Instance & License
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 22 | Unlimited instances | `/instances` | No "X/10 instances" limit shown |
| 23 | No "Assign License" prompt | Instance list | No lock badges |
| 24 | Instance activation bypass | Instance detail | All features available regardless |

### Banners & UI
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 25 | No trial banner | Top of page | No trial/upgrade banner |
| 26 | No subscription banner | Top of page | No expiry warning |
| 27 | No upgrade scroll banner | Top of page | No scrolling upgrade prompt |
| 28 | Subscription page shows Enterprise | `/setting/subscription` | Shows Enterprise plan |

### Router Guards
| # | Test Case | Page/Component | Expected |
|---|-----------|----------------|----------|
| 29 | 2FA guard works | Navigate any page with `requireMfa=true` | Redirects to 2FA setup |
| 30 | All routes accessible | Navigate to any Enterprise-only route | No 403/redirect due to license |

## AC
- [ ] All 30 test cases pass
- [ ] No sparkles (✨) badges visible anywhere in the UI
- [ ] No lock (🔒) badges on instances
- [ ] No "Upgrade", "Trial", "Subscribe" banners
- [ ] No paywall modals triggered
- [ ] `FeatureAttention` alerts not rendered
- [ ] TypeScript build clean (`pnpm build` no errors)
- [ ] Existing Vitest tests pass (or updated expectations)
