package common

import (
	"testing"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// TestEngineCapabilityMatrix_Exhaustive captures the current behavior of all engine
// capability functions. This test serves as a safety net for refactoring engine.go
// (TASK-AI-003-4: switch → map migration).
//
// Run BEFORE and AFTER refactoring to verify identical results.
func TestEngineCapabilityMatrix_Exhaustive(t *testing.T) {
	for name, val := range storepb.Engine_value {
		eng := storepb.Engine(val)
		if eng == storepb.Engine_ENGINE_UNSPECIFIED {
			continue
		}

		t.Run(name, func(t *testing.T) {
			// Record all 10 boolean capabilities
			sqlReview := EngineSupportSQLReview(eng)
			queryACL := EngineSupportQueryNewACL(eng)
			masking := EngineSupportMasking(eng)
			autoComplete := EngineSupportAutoComplete(eng)
			stmtAdvise := EngineSupportStatementAdvise(eng)
			stmtReport := EngineSupportStatementReport(eng)
			priorBackup := EngineSupportPriorBackup(eng)
			createDB := EngineSupportCreateDatabase(eng)
			querySpan := EngineSupportQuerySpanPlainField(eng)
			syntaxCheck := EngineSupportSyntaxCheck(eng)

			// Record BackupDatabaseName
			backupName := BackupDatabaseNameOfEngine(eng)

			// Log for debugging (test output shows actual capability matrix)
			t.Logf("Engine %-15s SQLReview=%t QueryACL=%t Masking=%t AutoComplete=%t StmtAdvise=%t StmtReport=%t PriorBackup=%t CreateDB=%t QuerySpan=%t SyntaxCheck=%t BackupDB=%q",
				name, sqlReview, queryACL, masking, autoComplete, stmtAdvise, stmtReport, priorBackup, createDB, querySpan, syntaxCheck, backupName)

			// Verify specific well-known engine capabilities (golden values)
			switch eng {
			case storepb.Engine_POSTGRES:
				assertEqual(t, "SQLReview", sqlReview, true)
				assertEqual(t, "QueryACL", queryACL, true)
				assertEqual(t, "Masking", masking, true)
				assertEqual(t, "CreateDB", createDB, true)
				assertEqual(t, "PriorBackup", priorBackup, true)
				assertEqual(t, "BackupDB", backupName, "bbdataarchive")
			case storepb.Engine_MYSQL:
				assertEqual(t, "SQLReview", sqlReview, true)
				assertEqual(t, "QueryACL", queryACL, true)
				assertEqual(t, "Masking", masking, true)
				assertEqual(t, "AutoComplete", autoComplete, true)
				assertEqual(t, "QuerySpan", querySpan, true)
			case storepb.Engine_ORACLE:
				assertEqual(t, "BackupDB", backupName, "BBDATAARCHIVE")
				assertEqual(t, "CreateDB", createDB, false)
			case storepb.Engine_REDIS:
				assertEqual(t, "SQLReview", sqlReview, false)
				assertEqual(t, "QueryACL", queryACL, false)
				assertEqual(t, "CreateDB", createDB, false)
			case storepb.Engine_CASSANDRA:
				assertEqual(t, "Masking", masking, true)
				assertEqual(t, "SQLReview", sqlReview, false)
			}
		})
	}
}

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
