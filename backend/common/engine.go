//nolint:revive
package common

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// EngineCapabilities defines the complete capability matrix for a database engine.
// Each field maps to a previously separate switch-statement function.
type EngineCapabilities struct {
	SQLReview        bool
	QueryNewACL      bool
	Masking          bool
	AutoComplete     bool
	StatementAdvise  bool
	StatementReport  bool
	PriorBackup      bool
	CreateDatabase   bool
	QuerySpanPlain   bool
	SyntaxCheck      bool
	BackupDBName     string // default: "bbdataarchive"
}

// engineCapabilities is the single source of truth for all engine capability flags.
// Adding a new engine requires adding an entry here — the init() check enforces exhaustiveness.
var engineCapabilities = map[storepb.Engine]EngineCapabilities{
	storepb.Engine_POSTGRES: {
		SQLReview: true, QueryNewACL: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true, PriorBackup: true,
		CreateDatabase: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_MYSQL: {
		SQLReview: true, QueryNewACL: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true, PriorBackup: true,
		CreateDatabase: true, QuerySpanPlain: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_TIDB: {
		SQLReview: true, QueryNewACL: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true, PriorBackup: true,
		CreateDatabase: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_MARIADB: {
		SQLReview: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true,
		CreateDatabase: true, QuerySpanPlain: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_OCEANBASE: {
		SQLReview: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true,
		CreateDatabase: true, QuerySpanPlain: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_ORACLE: {
		SQLReview: true, QueryNewACL: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true, PriorBackup: true,
		SyntaxCheck: true,
		BackupDBName: "BBDATAARCHIVE",
	},
	storepb.Engine_MSSQL: {
		SQLReview: true, QueryNewACL: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true, PriorBackup: true,
		CreateDatabase: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_SNOWFLAKE: {
		SQLReview: true, QueryNewACL: true, AutoComplete: true,
		StatementAdvise: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_REDSHIFT: {
		SQLReview: true, Masking: true, AutoComplete: true,
		StatementAdvise: true, StatementReport: true,
		CreateDatabase: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_CLICKHOUSE: {
		AutoComplete: true, CreateDatabase: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_MONGODB: {
		QueryNewACL: true, AutoComplete: true, CreateDatabase: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_SPANNER: {
		QueryNewACL: true, Masking: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_BIGQUERY: {
		QueryNewACL: true, Masking: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_SQLITE: {
		CreateDatabase: true,
		BackupDBName:   "bbdataarchive",
	},
	storepb.Engine_REDIS: {
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_CASSANDRA: {
		Masking:      true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_STARROCKS: {
		AutoComplete: true, CreateDatabase: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_HIVE: {
		CreateDatabase: true,
		BackupDBName:   "bbdataarchive",
	},
	storepb.Engine_COCKROACHDB: {
		AutoComplete: true, StatementAdvise: true,
		CreateDatabase: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_DORIS: {
		AutoComplete: true, CreateDatabase: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_DYNAMODB: {
		AutoComplete: true, StatementAdvise: true, SyntaxCheck: true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_ELASTICSEARCH: {
		QueryNewACL:  true,
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_DATABRICKS: {
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_COSMOSDB: {
		BackupDBName: "bbdataarchive",
	},
	storepb.Engine_TRINO: {
		Masking: true, AutoComplete: true,
		BackupDBName: "bbdataarchive",
	},
}

func init() {
	// Exhaustiveness check: panic at startup if any engine is missing from the map.
	// This replaces the previous //exhaustive:enforce linter directive.
	for name, val := range storepb.Engine_value {
		eng := storepb.Engine(val)
		if eng == storepb.Engine_ENGINE_UNSPECIFIED {
			continue
		}
		if _, ok := engineCapabilities[eng]; !ok {
			panic(fmt.Sprintf("engine %s (value=%d) missing from engineCapabilities map — add it to backend/common/engine.go", name, val))
		}
	}
}

// getCapabilities returns the capability set for an engine.
// Returns zero-value EngineCapabilities for ENGINE_UNSPECIFIED or unknown engines.
func getCapabilities(engine storepb.Engine) EngineCapabilities {
	return engineCapabilities[engine]
}

// --- Public API (thin wrappers, backward-compatible signatures) ---

func EngineSupportSQLReview(engine storepb.Engine) bool {
	return getCapabilities(engine).SQLReview
}

func EngineSupportQueryNewACL(engine storepb.Engine) bool {
	return getCapabilities(engine).QueryNewACL
}

func EngineSupportMasking(e storepb.Engine) bool {
	return getCapabilities(e).Masking
}

func EngineSupportAutoComplete(e storepb.Engine) bool {
	return getCapabilities(e).AutoComplete
}

func EngineSupportStatementAdvise(e storepb.Engine) bool {
	return getCapabilities(e).StatementAdvise
}

func EngineSupportStatementReport(e storepb.Engine) bool {
	return getCapabilities(e).StatementReport
}

func EngineSupportPriorBackup(e storepb.Engine) bool {
	return getCapabilities(e).PriorBackup
}

func EngineSupportCreateDatabase(e storepb.Engine) bool {
	return getCapabilities(e).CreateDatabase
}

func EngineSupportQuerySpanPlainField(e storepb.Engine) bool {
	return getCapabilities(e).QuerySpanPlain
}

func EngineSupportSyntaxCheck(e storepb.Engine) bool {
	return getCapabilities(e).SyntaxCheck
}

func BackupDatabaseNameOfEngine(e storepb.Engine) string {
	caps := getCapabilities(e)
	if caps.BackupDBName != "" {
		return caps.BackupDBName
	}
	return "bbdataarchive"
}

// --- Non-capability functions (kept as-is) ---

// TransactionMode represents the transaction execution mode for a migration script.
type TransactionMode string

const (
	// TransactionModeOn wraps the script in a single transaction.
	TransactionModeOn TransactionMode = "on"
	// TransactionModeOff executes the script's statements sequentially in auto-commit mode.
	TransactionModeOff TransactionMode = "off"
	// TransactionModeUnspecified means no explicit mode was specified.
	TransactionModeUnspecified TransactionMode = ""
)

// IsolationLevel represents the transaction isolation level.
type IsolationLevel string

const (
	// IsolationLevelDefault uses the database's default isolation level.
	IsolationLevelDefault IsolationLevel = ""
	// IsolationLevelReadUncommitted allows dirty reads.
	IsolationLevelReadUncommitted IsolationLevel = "READ UNCOMMITTED"
	// IsolationLevelReadCommitted prevents dirty reads.
	IsolationLevelReadCommitted IsolationLevel = "READ COMMITTED"
	// IsolationLevelRepeatableRead prevents dirty reads and non-repeatable reads.
	IsolationLevelRepeatableRead IsolationLevel = "REPEATABLE READ"
	// IsolationLevelSerializable provides the highest isolation level.
	IsolationLevelSerializable IsolationLevel = "SERIALIZABLE"
)

// TransactionConfig represents the complete transaction configuration.
type TransactionConfig struct {
	Mode      TransactionMode
	Isolation IsolationLevel
}

// GetDefaultTransactionMode returns the default transaction mode.
// All engines default to "on" (transactional) for safety and backward compatibility.
// Users can explicitly set "-- txn-mode = off" when needed for engines with limited transactional DDL support.
func GetDefaultTransactionMode() TransactionMode {
	// All engines default to "on" for safety and backward compatibility
	return TransactionModeOn
}

func ConvertToParserEngine(e storepb.Engine) (storepb.Engine, error) {
	switch e {
	case storepb.Engine_POSTGRES:
		return storepb.Engine_POSTGRES, nil
	case storepb.Engine_MYSQL, storepb.Engine_MARIADB, storepb.Engine_OCEANBASE:
		return storepb.Engine_MYSQL, nil
	case storepb.Engine_TIDB:
		return storepb.Engine_TIDB, nil
	case storepb.Engine_ORACLE:
		return storepb.Engine_ORACLE, nil
	case storepb.Engine_MSSQL:
		return storepb.Engine_MSSQL, nil
	case storepb.Engine_COCKROACHDB:
		return storepb.Engine_COCKROACHDB, nil
	default:
		return storepb.Engine_ENGINE_UNSPECIFIED, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid engine type %v", e))
	}
}
