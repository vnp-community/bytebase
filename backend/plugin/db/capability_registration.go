// capability_registration.go registers DriverCapabilities for every database engine.
//
// This file runs at init() time and centralizes the capability matrix in one place
// rather than spreading RegisterCapabilities() calls across 22+ driver files.
// The data here is sourced from backend/common/engine.go EngineCapabilities and
// verified against actual driver implementation.
//
// To add a new engine: add an entry here AND in backend/common/engine.go.

package db

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

func init() {
	// ─── Tier 1: Full-featured engines ────────────────────────

	RegisterCapabilities(storepb.Engine_POSTGRES, DriverCapabilities{
		SQLAdvisor:         true,
		AdvisorRuleCount:   200,
		SchemaDump:         DumpFull,
		PriorBackup:        true,
		OnlineSchemaChange: false, // pgroll integration pending (TASK-LIM-005-C1)
		DataMasking:        MaskingColumn,
		SchemaSync:         true,
		ChangeHistory:      true,
		BatchQuery:         true,
		ReadOnlyConnection: true,
		StreamingExport:    true,
		ParserEngine:       "antlr4",
	})

	RegisterCapabilities(storepb.Engine_MYSQL, DriverCapabilities{
		SQLAdvisor:         true,
		AdvisorRuleCount:   200,
		SchemaDump:         DumpFull,
		PriorBackup:        true,
		OnlineSchemaChange: true, // gh-ost
		DataMasking:        MaskingColumn,
		SchemaSync:         true,
		ChangeHistory:      true,
		BatchQuery:         true,
		ReadOnlyConnection: true,
		StreamingExport:    true,
		ParserEngine:       "antlr4",
	})

	RegisterCapabilities(storepb.Engine_TIDB, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 180,
		SchemaDump:       DumpFull,
		PriorBackup:      true,
		DataMasking:      MaskingColumn,
		SchemaSync:       true,
		ChangeHistory:    true,
		BatchQuery:       true,
		ReadOnlyConnection: true,
		StreamingExport:  true,
		ParserEngine:     "antlr4",
	})

	// ─── Tier 2: Good SQL review, partial features ────────────

	RegisterCapabilities(storepb.Engine_MARIADB, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 120,
		SchemaDump:       DumpFull,
		DataMasking:      MaskingColumn,
		SchemaSync:       true,
		ChangeHistory:    true,
		BatchQuery:       true,
		StreamingExport:  true,
		ParserEngine:     "antlr4",
	})

	RegisterCapabilities(storepb.Engine_OCEANBASE, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 120,
		SchemaDump:       DumpPartial,
		DataMasking:      MaskingColumn,
		SchemaSync:       true,
		ChangeHistory:    true,
		BatchQuery:       true,
		ParserEngine:     "antlr4",
	})

	RegisterCapabilities(storepb.Engine_ORACLE, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 80,
		SchemaDump:       DumpFull,
		PriorBackup:      true,
		DataMasking:      MaskingColumn,
		SchemaSync:       true,
		ChangeHistory:    true,
		BatchQuery:       true,
		ParserEngine:     "antlr4",
	})

	RegisterCapabilities(storepb.Engine_MSSQL, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 25,
		SchemaDump:       DumpFull,
		PriorBackup:      true,
		DataMasking:      MaskingColumn,
		SchemaSync:       true,
		ChangeHistory:    true,
		BatchQuery:       true,
		ParserEngine:     "antlr4",
	})

	RegisterCapabilities(storepb.Engine_SNOWFLAKE, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 15,
		SchemaDump:       DumpPartial,
		SchemaSync:       true,
		ChangeHistory:    true,
		ParserEngine:     "antlr4",
	})

	RegisterCapabilities(storepb.Engine_REDSHIFT, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 15,
		SchemaDump:       DumpPartial,
		DataMasking:      MaskingColumn,
		SchemaSync:       true,
		ChangeHistory:    true,
		ParserEngine:     "antlr4",
	})

	RegisterCapabilities(storepb.Engine_COCKROACHDB, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 10,
		SchemaDump:       DumpPartial,
		SchemaSync:       true,
		ChangeHistory:    true,
		ParserEngine:     "antlr4",
		KnownParserGaps:  []string{"CRDB-specific syntax not covered by PG grammar"},
	})

	// ─── Tier 3: Limited / no SQL review ──────────────────────

	RegisterCapabilities(storepb.Engine_CLICKHOUSE, DriverCapabilities{
		SQLAdvisor:       false, // TASK-LIM-005-B1 will set true
		AdvisorRuleCount: 0,
		SchemaDump:       DumpPartial,
		SchemaSync:       true,
		ChangeHistory:    true,
		BatchQuery:       true,
		ParserEngine:     "custom",
		KnownParserGaps:  []string{"MergeTree ENGINE clause", "MATERIALIZED VIEW", "AggregatingMergeTree"},
	})

	RegisterCapabilities(storepb.Engine_MONGODB, DriverCapabilities{
		DataMasking:   MaskingDocument,
		SchemaSync:    true,
		ChangeHistory: true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_SPANNER, DriverCapabilities{
		DataMasking:   MaskingColumn,
		SchemaSync:    true,
		ChangeHistory: true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_BIGQUERY, DriverCapabilities{
		DataMasking:   MaskingColumn,
		SchemaSync:    true,
		ChangeHistory: true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_SQLITE, DriverCapabilities{
		SchemaDump:    DumpFull,
		SchemaSync:    true,
		ChangeHistory: true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_REDIS, DriverCapabilities{
		SchemaSync: true,
		ParserEngine: "none",
	})

	RegisterCapabilities(storepb.Engine_CASSANDRA, DriverCapabilities{
		DataMasking:  MaskingColumn,
		SchemaSync:   true,
		ParserEngine: "none",
	})

	RegisterCapabilities(storepb.Engine_STARROCKS, DriverCapabilities{
		SchemaDump:    DumpPartial,
		SchemaSync:    true,
		ChangeHistory: true,
		BatchQuery:    true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_DORIS, DriverCapabilities{
		SchemaDump:    DumpPartial,
		SchemaSync:    true,
		ChangeHistory: true,
		BatchQuery:    true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_HIVE, DriverCapabilities{
		SchemaSync:    true,
		ChangeHistory: true,
		ParserEngine:  "none",
	})

	RegisterCapabilities(storepb.Engine_DYNAMODB, DriverCapabilities{
		SQLAdvisor:       true,
		AdvisorRuleCount: 5,
		SchemaSync:       true,
		ParserEngine:     "custom",
	})

	RegisterCapabilities(storepb.Engine_ELASTICSEARCH, DriverCapabilities{
		SchemaSync:   true,
		ParserEngine: "none",
	})

	RegisterCapabilities(storepb.Engine_DATABRICKS, DriverCapabilities{
		SchemaSync:   true,
		ParserEngine: "none",
	})

	RegisterCapabilities(storepb.Engine_COSMOSDB, DriverCapabilities{
		SchemaSync:   true,
		ParserEngine: "none",
	})

	RegisterCapabilities(storepb.Engine_TRINO, DriverCapabilities{
		DataMasking:  MaskingColumn,
		SchemaSync:   true,
		ParserEngine: "none",
	})
}
