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

- [x] Types defined: `DumpLevel`, `MaskingLevel`, `DriverCapabilities` → **DONE**: 3 iota enums + 14-field struct
- [x] Registry functions: Register, Get, ListAll → **DONE**: `RegisterCapabilities()`, `GetCapabilities()`, `ListAllCapabilities()`
- [x] Thread-safe registry (sync.RWMutex or sync.Map) → **DONE**: `sync.RWMutex` guards `capsMap`
- [x] Get returns zero-value for unregistered engine (no panic) → **DONE**: map lookup returns zero-value `DriverCapabilities{}`

## Implementation Notes

- Created `backend/plugin/db/capability.go`:
  - `DumpLevel` enum: `DumpNone`, `DumpPartial`, `DumpFull`
  - `MaskingLevel` enum: `MaskingNone`, `MaskingDocument`, `MaskingColumn`
  - `DriverCapabilities` struct: 14 fields covering advisor, dump, backup, masking, sync, parser
  - Thread-safe global registry with `sync.RWMutex`
  - `RegisterCapabilities()` — panics on duplicate (like driver registration)
  - `GetCapabilities()` — zero-value for unknown engines
  - `ListAllCapabilities()` — returns a copy map
  - `RegisteredCapabilityCount()` — helper for assertions

**Status: ✅ DONE**
