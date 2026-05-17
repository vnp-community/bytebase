# Solution: Multi-Format Policy Compiler

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-005 |
| **CR Reference** | CR-POL-005 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-001 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| `PolicyCompiler` interface | **L5 — Component** | Shared compilation pipeline |
| Format-specific compilers | **L7 — Plugin** | Each compiler is a plugin, registered via factory |
| `AbstractPolicy` model | **L5 — Component** | Intermediate representation for cross-format conversion |
| `PolicyConverter` | **L5 — Component** | Cross-format translation engine |
| `PolicyImporter` | **L5 — Component** | External policy import (bundle, git, URL) |

---

## 2. Package Structure

```
backend/component/policy/compiler/
├── compiler.go        ← PolicyCompiler interface + CompilerRegistry
├── abstract.go        ← AbstractPolicy, AbstractRule, ConditionExpr (IR)
├── converter.go       ← PolicyConverter: cross-format translation via APM
├── importer.go        ← PolicyImporter: file, URL, git, OPA bundle import
├── rego.go            ← RegoCompiler: OPA ast + Regal integration
├── cedar.go           ← CedarCompiler: cedar-go parse + validate
├── cel.go             ← CELCompiler: google/cel-go parse + type-check
└── yaml.go            ← YAMLCompiler: go-yaml + JSON Schema validation
```

---

## 3. Key Design Decisions

### 3.1 PolicyCompiler Interface

```go
type PolicyCompiler interface {
    // Language returns the policy language this compiler handles.
    Language() PolicyLanguage

    // Parse converts source code to Abstract Syntax Tree.
    Parse(source string) (*PolicyAST, error)

    // Validate checks AST for semantic correctness.
    Validate(ast *PolicyAST) (*ValidationResult, error)

    // Compile converts AST to engine-specific binary form.
    Compile(ast *PolicyAST, targetEngine string) ([]byte, error)

    // Lint checks for style/best-practice issues.
    Lint(source string) ([]*LintIssue, error)

    // Format auto-formats source code.
    Format(source string) (string, error)
}
```

### 3.2 Abstract Policy Model (APM) — Cross-Format IR

The APM serves as an intermediate representation for policy translation:

```go
type AbstractPolicy struct {
    Name        string
    Description string
    Rules       []*AbstractRule
}

type AbstractRule struct {
    Effect      RuleEffect        // PERMIT, FORBID, ALLOW, DENY
    Subject     *SubjectMatcher   // Principal matching conditions
    Action      *ActionMatcher    // Action matching (query, migrate, export)
    Resource    *ResourceMatcher  // Resource matching conditions
    Conditions  []*ConditionExpr  // When/Unless conditions
}

type ConditionExpr struct {
    Op          ConditionOp       // AND, OR, NOT, EQ, NEQ, GT, LT, IN, CONTAINS
    Left        *ConditionValue
    Right       *ConditionValue
    Children    []*ConditionExpr  // For AND/OR/NOT
}
```

**Design rationale**: APM maps cleanly to:
- **Rego**: `AbstractRule` → Rego rule with conditions as Rego expressions
- **Cedar**: `AbstractRule` → Cedar `permit`/`forbid` statement with `when`/`unless`
- **CEL**: `AbstractRule` → CEL expression tree

### 3.3 Conversion Matrix

| From → To | Rego | Cedar | CEL | YAML |
|---|---|---|---|---|
| **Rego** | — | Partial ⚠️ | Partial ⚠️ | Limited ❌ |
| **Cedar** | Full ✅ | — | Partial ⚠️ | Partial ⚠️ |
| **CEL** | Full ✅ | Full ✅ | — | Partial ⚠️ |
| **YAML** | Full ✅ | Full ✅ | Full ✅ | — |

**Conversion strategy**:
1. Parse source → AST (format-specific)
2. AST → AbstractPolicy (normalize)
3. AbstractPolicy → target AST (generate)
4. Target AST → formatted source

**Limitations**: Complex Rego (data aggregation, comprehensions) cannot be losslessly converted to Cedar/CEL. The converter reports fidelity score (0.0-1.0) and unsupported constructs.

### 3.4 Format-Specific Compilers

| Compiler | Library | Capabilities |
|---|---|---|
| `RegoCompiler` | OPA `ast` package + Regal linter | Parse, compile, lint (200+ rules), format |
| `CedarCompiler` | `cedar-go` | Parse, validate against Bytebase schema, format |
| `CELCompiler` | `google/cel-go` | Parse, type-check, compile to program |
| `YAMLCompiler` | `go-yaml/v3` + JSON Schema | Parse, validate structure, convert to JSON |

### 3.5 Policy Importer

```go
type PolicyImporter struct {
    compilers map[PolicyLanguage]PolicyCompiler
}

// Import detects language and parses policies from various sources.
func (i *PolicyImporter) Import(ctx context.Context, source ImportSource) ([]*PolicyDefinition, error)

type ImportSource struct {
    Type     string  // "file", "url", "git", "opa-bundle", "conftest"
    Location string  // Path, URL, or git repo URL
    Branch   string  // For git sources
    Path     string  // Path within git repo
}
```

Auto-detection rules:
- `.rego` → Rego
- `.cedar` → Cedar
- `.cel` → CEL
- `.yaml`/`.yml` → YAML
- OPA bundle (`.tar.gz` with `/.manifest`) → Rego

---

## 4. Integration Points

### With PolicyManager (SOL-POL-001)

```go
// PolicyManager uses compiler for:
// 1. Validate policy before LoadPolicy()
decision := compiler.Validate(ast)
if !decision.Valid {
    return errors.New("invalid policy")
}

// 2. Compile policy for target engine
compiled, err := compiler.Compile(ast, engine.Name())
engine.LoadPolicy(ctx, &PolicyDefinition{Source: source, Compiled: compiled})
```

### With GitOps Pipeline (SOL-POL-007)

```go
// CI/CD webhook handler uses compiler for:
// 1. Lint new policies (PR check)
issues := compiler.Lint(source)
// 2. Validate against schema
result := compiler.Validate(ast)
// 3. Run impact analysis via conversion to test fixtures
```

---

## 5. Go Dependencies

```go
require (
    github.com/open-policy-agent/opa v1.x.x       // Rego parser/compiler
    github.com/cedar-policy/cedar-go v0.x.x        // Cedar parser
    github.com/google/cel-go v0.x.x                // CEL compiler
    github.com/styrainc/regal v0.x.x               // Rego linter (optional)
    gopkg.in/yaml.v3                                // YAML parser
)
```

---

## 6. Performance Targets

| Operation | Target |
|---|---|
| Parse (any format) | < 10ms |
| Compile (any format) | < 50ms |
| Convert (simple policy) | < 20ms |
| Convert (complex policy) | < 200ms |
| Lint (Rego, 100 rules) | < 500ms |
| Import OPA bundle (50 policies) | < 2s |
