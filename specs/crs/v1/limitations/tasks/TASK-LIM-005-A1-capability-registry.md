# TASK-LIM-005-A1: Capability Types + Registry

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-005 |
| Phase | A — Capability Registry (Arch Change) |
| Priority | P0 |
| Depends On | — |
| Est. | M (~150 LoC) |

## Objective

Define `DriverCapabilities` struct and global registry. Each driver will declare its capabilities at `init()` time via `RegisterCapabilities()`.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/plugin/db/capability.go` |

## Specification

```go
type DumpLevel int
const (DumpNone DumpLevel = iota; DumpPartial; DumpFull)

type MaskingLevel int
const (MaskingNone MaskingLevel = iota; MaskingDocument; MaskingColumn)

type DriverCapabilities struct {
    SQLAdvisor         bool
    AdvisorRuleCount   int
    SchemaDump         DumpLevel
    PriorBackup        bool
    OnlineSchemaChange bool
    DataMasking        MaskingLevel
    SchemaSync         bool
    ChangeHistory      bool
    BatchQuery         bool
    ReadOnlyConnection bool
    StreamingExport    bool
    ParserEngine       string    // "antlr4", "custom", "none"
    KnownParserGaps    []string
}

// Global registry
func RegisterCapabilities(engine storepb.Engine, caps DriverCapabilities)
func GetCapabilities(engine storepb.Engine) DriverCapabilities
func ListAllCapabilities() map[storepb.Engine]DriverCapabilities
```

## Acceptance Criteria

- [ ] Types defined: `DumpLevel`, `MaskingLevel`, `DriverCapabilities`
- [ ] Registry functions: Register, Get, ListAll
- [ ] Thread-safe registry (sync.RWMutex or sync.Map)
- [ ] Get returns zero-value for unregistered engine (no panic)
