# TASK-PRV-005 — Anonymization Engine + Basic Techniques

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-005                               |
| **Source**       | SOL-PRV-002 Phase 1 (CR-PRV-002)          |
| **Status**       | Pending                                    |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Xây dựng Anonymization Engine core tại L5 với basic techniques (Suppression, Generalization, Noise Addition, Data Swapping).

## Scope

1. **L5 — `component/privacy/anonymizer.go`**: AnonymizationEngine struct + `Anonymize()` method + strategy resolver
2. **Technique implementations**: Suppression (→ NULL), Generalization (→ range), Noise Addition (numeric ±%), Data Swapping (shuffle rows)
3. **L8 — Migration**: Tạo bảng `anonymization_policy` + `pseudonym_lookup`
4. **L8 — `store/anonymization_policy.go`**: Policy CRUD
5. **L9 — `feature.go`**: `FeatureDataAnonymization` gate

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/anonymizer.go` | NEW — Core engine |
| `backend/store/anonymization_policy.go` | NEW — Policy store |
| `backend/store/migration/` | NEW — DDL migration |
| `backend/enterprise/feature.go` | ADD — `FeatureDataAnonymization` |

## Acceptance Criteria

- [ ] 4 techniques implemented: Suppression, Generalization, Noise Addition, Data Swapping
- [ ] Anonymized data irreversible (Suppression/Generalization)
- [ ] Referential integrity maintained across related tables (Data Swapping)
- [ ] Per-column technique selection via policy
- [ ] Policy CRUD functional
- [ ] Unit tests cho mỗi technique

## Dependencies

- None (foundation for anonymization stack)

## Definition of Done

- All 4 techniques tested with sample data
- Irreversibility verified
