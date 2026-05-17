# Change Request: Multi-Format Policy Compiler

| Field | Value |
|---|---|
| **CR ID** | CR-POL-005 |
| **Title** | Multi-Format Policy Compiler |
| **Plan** | ENTERPRISE |
| **Priority** | P1 — High |
| **Status** | Draft |
| **Created** | 2026-05-17 |
| **Author** | VNP AI Ops Team |
| **Dependencies** | CR-POL-001 |

---

## 1. Tổng quan

### 1.1 Mô tả
Hệ thống dịch, biên dịch, và validate chính sách giữa nhiều định dạng: Rego, Cedar DSL, CEL, YAML, JSON. Cho phép viết policy bằng bất kỳ ngôn ngữ nào, compile sang engine-specific format, và convert giữa formats khi có thể.

### 1.2 Mục tiêu
- Unified compilation pipeline cho tất cả policy languages
- Format-specific parsers, validators, linters
- Cross-format conversion (best-effort) qua Abstract Policy Model
- Policy import từ external systems (OPA bundles, Git, Conftest)

---

## 2. Yêu cầu chức năng

### FR-001: PolicyCompiler Interface

```go
type PolicyCompiler interface {
    Parse(source string, lang PolicyLanguage) (*PolicyAST, error)
    Validate(ast *PolicyAST) (*ValidationResult, error)
    Compile(ast *PolicyAST, targetEngine string) ([]byte, error)
    Lint(source string, lang PolicyLanguage) ([]*LintIssue, error)
    Format(source string, lang PolicyLanguage) (string, error)
}
```

### FR-002: Abstract Policy Model (APM)

Intermediate representation cho cross-format conversion:
- `AbstractPolicy` → `AbstractRule[]` → `ConditionExpr` tree
- Maps cleanly to Rego rules, Cedar permit/forbid, CEL expressions

### FR-003: Format-Specific Compilers

| Compiler | Library | Features |
|---|---|---|
| RegoCompiler | OPA `ast` package + Regal | Parse, compile, lint, format |
| CedarCompiler | `cedar-go` | Parse, validate against schema |
| CELCompiler | `google/cel-go` | Parse, type-check, compile |
| YAMLCompiler | `go-yaml` + JSON Schema | Parse, validate structure |

### FR-004: Conversion Matrix

| From / To | Rego | Cedar | CEL | YAML |
|---|---|---|---|---|
| Rego | — | Partial ⚠️ | Partial ⚠️ | Limited ❌ |
| Cedar | Full ✅ | — | Partial ⚠️ | Partial ⚠️ |
| CEL | Full ✅ | Full ✅ | — | Partial ⚠️ |
| YAML | Full ✅ | Full ✅ | Full ✅ | — |

### FR-005: Policy Importer

- Import from file, URL, Git repository, OPA bundle, Conftest
- Auto-detect policy language from file extension/content

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| PolicyCompiler interface | `backend/component/policy/compiler/compiler.go` | New |
| RegoCompiler | `backend/component/policy/compiler/rego.go` | New |
| CedarCompiler | `backend/component/policy/compiler/cedar.go` | New |
| CELCompiler | `backend/component/policy/compiler/cel.go` | New |
| YAMLCompiler | `backend/component/policy/compiler/yaml.go` | New |
| AbstractPolicy model | `backend/component/policy/compiler/abstract.go` | New |
| PolicyConverter | `backend/component/policy/compiler/converter.go` | New |
| PolicyImporter | `backend/component/policy/compiler/importer.go` | New |

---

## 4. Test Cases

| Test ID | Mô tả | Expected Result |
|---|---|---|
| TC-001 | Parse valid Rego | PolicyAST returned |
| TC-002 | Parse invalid Rego | Error with line/column |
| TC-003 | Convert Cedar → Rego | Valid Rego, fidelity=1.0 |
| TC-004 | Convert Rego → Cedar (complex) | Partial with limitations |
| TC-005 | Import OPA bundle | All policies loaded |
| TC-006 | Lint Rego with Regal | Issues identified |
| TC-007 | Round-trip: parse → abstract → generate | Semantically equivalent |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Abstract model + interfaces | Sprint 1 |
| Phase 2 | Rego compiler + Regal | Sprint 1-2 |
| Phase 3 | Cedar + CEL compilers | Sprint 2-3 |
| Phase 4 | Cross-format converter | Sprint 3-4 |
| Phase 5 | Policy importer | Sprint 4 |
| Phase 6 | Integration testing | Sprint 5 |
