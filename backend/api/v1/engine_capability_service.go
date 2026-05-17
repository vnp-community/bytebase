// Package v1 provides the engine capability query API.
//
// This serves the driver capability registry data as JSON for the
// frontend feature matrix and engine comparison views.
package v1

import (
	"github.com/bytebase/bytebase/backend/plugin/db"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// EngineCapabilityResponse is the JSON response for engine capability queries.
type EngineCapabilityResponse struct {
	Engine             string   `json:"engine"`
	SQLAdvisor         bool     `json:"sqlAdvisor"`
	AdvisorRuleCount   int      `json:"advisorRuleCount"`
	SchemaDump         string   `json:"schemaDump"`
	PriorBackup        bool     `json:"priorBackup"`
	OnlineSchemaChange bool     `json:"onlineSchemaChange"`
	DataMasking        string   `json:"dataMasking"`
	SchemaSync         bool     `json:"schemaSync"`
	ChangeHistory      bool     `json:"changeHistory"`
	BatchQuery         bool     `json:"batchQuery"`
	ReadOnlyConnection bool     `json:"readOnlyConnection"`
	StreamingExport    bool     `json:"streamingExport"`
	ParserEngine       string   `json:"parserEngine"`
	KnownParserGaps    []string `json:"knownParserGaps,omitempty"`
}

func dumpLevelString(l db.DumpLevel) string {
	switch l {
	case db.DumpNone:
		return "none"
	case db.DumpPartial:
		return "partial"
	case db.DumpFull:
		return "full"
	default:
		return "unknown"
	}
}

func maskingLevelString(l db.MaskingLevel) string {
	switch l {
	case db.MaskingNone:
		return "none"
	case db.MaskingDocument:
		return "document"
	case db.MaskingColumn:
		return "column"
	default:
		return "unknown"
	}
}

func capsToResponse(engine storepb.Engine, caps db.DriverCapabilities) EngineCapabilityResponse {
	return EngineCapabilityResponse{
		Engine:             engine.String(),
		SQLAdvisor:         caps.SQLAdvisor,
		AdvisorRuleCount:   caps.AdvisorRuleCount,
		SchemaDump:         dumpLevelString(caps.SchemaDump),
		PriorBackup:        caps.PriorBackup,
		OnlineSchemaChange: caps.OnlineSchemaChange,
		DataMasking:        maskingLevelString(caps.DataMasking),
		SchemaSync:         caps.SchemaSync,
		ChangeHistory:      caps.ChangeHistory,
		BatchQuery:         caps.BatchQuery,
		ReadOnlyConnection: caps.ReadOnlyConnection,
		StreamingExport:    caps.StreamingExport,
		ParserEngine:       caps.ParserEngine,
		KnownParserGaps:    caps.KnownParserGaps,
	}
}

// GetSingleEngineCapabilities returns capabilities for a single engine.
func GetSingleEngineCapabilities(engine storepb.Engine) EngineCapabilityResponse {
	caps := db.GetCapabilities(engine)
	return capsToResponse(engine, caps)
}

// GetAllEngineCapabilities returns capabilities for all registered engines.
func GetAllEngineCapabilities() []EngineCapabilityResponse {
	allCaps := db.ListAllCapabilities()
	result := make([]EngineCapabilityResponse, 0, len(allCaps))
	for engine, caps := range allCaps {
		result = append(result, capsToResponse(engine, caps))
	}
	return result
}
