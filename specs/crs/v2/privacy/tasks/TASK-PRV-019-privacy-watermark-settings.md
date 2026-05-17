# TASK-PRV-019 — Privacy Watermark + User Privacy Settings

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-019                               |
| **Source**       | SOL-PRV-008 Phase 3–4 (CR-PRV-008)        |
| **Status**       | Pending                                    |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2–3                                 |

---

## Mô tả

Pseudonymized watermark và user privacy settings UI + ExportMyData (GDPR portability).

## Scope

1. **L5 — `component/watermark/privacy.go`**: PrivacyWatermark — 3 content modes: Token (pseudonymized), Initials, Department (thay thế email trực tiếp)
2. **Watermark trace-back**: Admin can resolve token → user (requires `bb.privacy.reidentify` + audit)
3. **L4 — `api/v1/user_privacy_service.go`**: User privacy settings CRUD — stored in existing `user_setting` table
4. **Privacy settings**: Query history retention (1/7/30/90 days), Telemetry opt-out, Activity visibility (private/team/project), Login notification
5. **ExportMyData API**: GDPR Article 20 — export user's own data in JSON (profile, query history, activities, settings)
6. **L1 — `UserPrivacy.tsx`**: Privacy settings UI page

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/watermark/privacy.go` | NEW — Privacy watermark |
| `backend/api/v1/user_privacy_service.go` | NEW — Privacy settings |
| `frontend/src/react/pages/settings/UserPrivacy.tsx` | NEW — Settings UI |
| `backend/enterprise/feature.go` | ADD — `FeatureUserPrivacy` |

## Acceptance Criteria

- [ ] Default watermark: pseudonymized token (not email/full name)
- [ ] Configurable watermark content: token, initials, department
- [ ] Watermark trace-back requires admin approval + audit
- [ ] Privacy settings accessible from user profile
- [ ] Settings take effect within 24 hours
- [ ] Default settings: most privacy-preserving option
- [ ] ExportMyData: JSON export of own data
- [ ] ExportMyData rate limited: 1 per day

## Dependencies

- TASK-PRV-006 (Pseudonymization engine — for watermark tokens)
- TASK-ENT-024 (Watermark overlay — existing watermark component)

## Definition of Done

- Watermark modes tested visually
- Privacy settings CRUD functional
- ExportMyData tested with real user data
