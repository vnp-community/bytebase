// Package db provides the interfaces and libraries for database driver plugins.
//
// capability.go defines the DriverCapabilities struct and a thread-safe global
// registry. Each database driver declares its capabilities at init() time via
// RegisterCapabilities(), enabling runtime feature-flag queries.

package db

import (
	"sync"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// DumpLevel indicates the schema-dump fidelity of a database driver.
type DumpLevel int

const (
	// DumpNone means the driver cannot produce a schema dump.
	DumpNone DumpLevel = iota
	// DumpPartial means the driver can dump some (but not all) object types.
	DumpPartial
	// DumpFull means the driver produces a complete, restorable schema dump.
	DumpFull
)

// MaskingLevel indicates the data-masking capability of a database driver.
type MaskingLevel int

const (
	// MaskingNone means no masking support.
	MaskingNone MaskingLevel = iota
	// MaskingDocument means document-level masking (NoSQL).
	MaskingDocument
	// MaskingColumn means column-level masking (SQL engines).
	MaskingColumn
)

// DriverCapabilities declares what features a database driver supports.
// Populated at init() time by each driver via RegisterCapabilities().
type DriverCapabilities struct {
	// SQLAdvisor indicates SQL review / lint rules are available.
	SQLAdvisor bool
	// AdvisorRuleCount is the number of registered advisor rules.
	AdvisorRuleCount int
	// SchemaDump indicates schema export fidelity.
	SchemaDump DumpLevel
	// PriorBackup indicates the driver supports pre-migration backup.
	PriorBackup bool
	// OnlineSchemaChange indicates zero-downtime DDL (gh-ost, pgroll, etc.).
	OnlineSchemaChange bool
	// DataMasking indicates column/document-level data masking support.
	DataMasking MaskingLevel
	// SchemaSync indicates the driver can sync database schema metadata.
	SchemaSync bool
	// ChangeHistory indicates the driver supports migration change history.
	ChangeHistory bool
	// BatchQuery indicates the driver supports batch query execution.
	BatchQuery bool
	// ReadOnlyConnection indicates the driver supports read-only connections.
	ReadOnlyConnection bool
	// StreamingExport indicates the driver supports streaming data export.
	StreamingExport bool
	// ParserEngine identifies the SQL parser used ("antlr4", "custom", "none").
	ParserEngine string
	// KnownParserGaps lists known SQL constructs not fully supported by the parser.
	KnownParserGaps []string
}

// --- Thread-safe capability registry ---

var (
	capsMu   sync.RWMutex
	capsMap  = make(map[storepb.Engine]DriverCapabilities)
)

// RegisterCapabilities registers the capabilities for a database engine.
// Typically called from each driver's init() function.
// Panics if called twice for the same engine.
func RegisterCapabilities(engine storepb.Engine, caps DriverCapabilities) {
	capsMu.Lock()
	defer capsMu.Unlock()
	if _, dup := capsMap[engine]; dup {
		panic("db: RegisterCapabilities called twice for engine " + engine.String())
	}
	capsMap[engine] = caps
}

// GetCapabilities returns the capabilities for a database engine.
// Returns zero-value DriverCapabilities for unregistered engines (no panic).
func GetCapabilities(engine storepb.Engine) DriverCapabilities {
	capsMu.RLock()
	defer capsMu.RUnlock()
	return capsMap[engine]
}

// ListAllCapabilities returns a snapshot of all registered engine capabilities.
// The returned map is a copy; callers may modify it freely.
func ListAllCapabilities() map[storepb.Engine]DriverCapabilities {
	capsMu.RLock()
	defer capsMu.RUnlock()
	result := make(map[storepb.Engine]DriverCapabilities, len(capsMap))
	for k, v := range capsMap {
		result[k] = v
	}
	return result
}

// RegisteredCapabilityCount returns the number of engines with registered capabilities.
func RegisteredCapabilityCount() int {
	capsMu.RLock()
	defer capsMu.RUnlock()
	return len(capsMap)
}
