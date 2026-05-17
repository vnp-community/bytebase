# ISS-AI-010 — Domain Knowledge Gap: Database Admin Workflows Đặc Thù

> **Category**: Domain Specificity  
> **Severity**: Medium  
> **Impact**: Feature Development, Business Logic Understanding  
> **Affected Area**: SQL Editor, Schema Management, Rollout, Review

---

## 1. Mô Tả Vấn Đề

Bytebase là **Database DevOps platform** với domain knowledge đặc thù:

### 1.1 Specialized Domains AI Thiếu Context

| Domain | Complexity | AI Blind Spot |
|---|---|---|
| **SQL Review Rules** | 100+ rules × 15+ DB engines | AI không biết rule nào áp dụng cho engine nào |
| **Schema Change Workflow** | Plan → Issue → Rollout → Stage → Task → TaskRun | AI confuse multi-step lifecycle |
| **Data Masking** | Column-level masking + CEL expressions | AI không hiểu CEL syntax và masking levels |
| **Multi-engine Support** | MySQL, PostgreSQL, TiDB, Oracle, SQL Server, MongoDB, Spanner, etc. | AI generate engine-specific SQL sai |
| **IAM Model** | Resource-based permissions (workspace/project scope) | AI dùng sai permission granularity |
| **Database Group** | CEL expression matching databases | AI không hiểu dynamic group membership |

### 1.2 Business Logic Encoded In Code

- `src/types/sql-review-schema.yaml` (46KB) — SQL review rule definitions.
- `src/types/plan.yaml` (6KB) — Subscription plan feature matrix.
- `src/utils/issue/` — Issue lifecycle state machine.
- `src/utils/schemaEditor/` — Schema diff algorithms.
- `src/plugins/cel/` — Common Expression Language parser.

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **SQL Review UI** | AI không biết rule categories, severity levels, engine compatibility |
| **Plan/Issue flow** | AI confuse: Plan vs Issue vs Rollout vs Release lifecycle |
| **Permission check** | AI dùng wrong scope (workspace vs project vs database level) |
| **Schema editor** | AI không hiểu diff/merge algorithms cho DDL statements |
| **CEL expressions** | AI generate invalid CEL syntax cho database group matching |

## 3. Khuyến Nghị

1. **Domain glossary**: Tạo `GLOSSARY.md` giải thích Plan, Issue, Rollout, Stage, Task, TaskRun relationships.
2. **Workflow diagrams**: Mermaid diagrams cho top-5 business workflows.
3. **Engine compatibility matrix**: Which features/rules apply to which DB engines.
4. **AI domain primers**: Short docs giải thích mỗi domain area cho AI consumption.
