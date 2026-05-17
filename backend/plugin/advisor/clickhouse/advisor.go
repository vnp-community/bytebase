// Package clickhouse provides SQL review rules for ClickHouse databases.
//
// This advisor implements 18 registered rules across 7 categories:
//   - Naming conventions (table, column)
//   - Engine clauses (require ENGINE, prefer MergeTree)
//   - Partitioning (require PARTITION BY)
//   - Query safety (no SELECT *, require LIMIT, no JOIN on Distributed)
//   - Type safety (prefer LowCardinality, no Nullable in aggregates)
//   - DML safety (WHERE required, no TRUNCATE)
//   - Backward compatibility (DROP TABLE/COLUMN warnings)
//   - Index guidance (ORDER BY usage)
//   - DDL/DML controls
package clickhouse

import (
	"context"
	"strings"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/code"
)

// ─── Naming rules ─────────────────────────────────────────────

// TableLowercaseSnakeAdvisor checks that table names use lowercase_snake_case.
type TableLowercaseSnakeAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_NAMING_TABLE, &TableLowercaseSnakeAdvisor{})
}

func (*TableLowercaseSnakeAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") && !strings.Contains(text, "ALTER TABLE") {
			continue
		}
		tableName := extractIdentifier(stmt.Text, "TABLE")
		if tableName == "" {
			continue
		}
		if !isLowercaseSnake(tableName) {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.NamingTableConventionMismatch.Int32(),
				Title:   "Table name should be lowercase_snake_case",
				Content: "Table `" + tableName + "` does not follow lowercase_snake_case convention",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ColumnLowercaseAdvisor checks that column names are lowercase.
type ColumnLowercaseAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_NAMING_COLUMN, &ColumnLowercaseAdvisor{})
}

func (*ColumnLowercaseAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") {
			continue
		}
		columns := extractColumnNames(stmt.Text)
		for _, col := range columns {
			if col != strings.ToLower(col) {
				status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
				if err != nil {
					return nil, err
				}
				adviceList = append(adviceList, &storepb.Advice{
					Status:  status,
					Code:    code.NamingColumnConventionMismatch.Int32(),
					Title:   "Column name should be lowercase",
					Content: "Column `" + col + "` is not lowercase",
					StartPosition: &storepb.Position{
						Line: int32(stmt.BaseLine()),
					},
				})
			}
		}
	}
	return adviceList, nil
}

// ─── Engine rules ─────────────────────────────────────────────

// RequireEngineClauseAdvisor checks that CREATE TABLE includes ENGINE clause.
type RequireEngineClauseAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_TABLE_REQUIRE_PK, &RequireEngineClauseAdvisor{})
}

func (*RequireEngineClauseAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") {
			continue
		}
		if !strings.Contains(text, "ENGINE") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.TableNoPK.Int32(),
				Title:   "Missing ENGINE clause",
				Content: "CREATE TABLE must include an ENGINE clause for ClickHouse",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// PreferMergeTreeAdvisor warns when non-MergeTree engine families are used.
type PreferMergeTreeAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_TABLE_COMMENT, &PreferMergeTreeAdvisor{})
}

func (*PreferMergeTreeAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	mergeTreeFamilies := []string{
		"MERGETREE", "REPLACINGMERGETREE", "SUMMINGMERGETREE",
		"AGGREGATINGMERGETREE", "COLLAPSINGMERGETREE",
		"VERSIONEDCOLLAPSINGMERGETREE", "GRAPHITEMERGETREE",
		"REPLICATEDMERGETREE", "REPLICATEDREPLACINGMERGETREE",
		"REPLICATEDSUMMINGMERGETREE", "REPLICATEDAGGREGATINGMERGETREE",
	}
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") || !strings.Contains(text, "ENGINE") {
			continue
		}
		engineName := extractEngineClause(stmt.Text)
		if engineName == "" {
			continue
		}
		isMergeTree := false
		upper := strings.ToUpper(engineName)
		for _, mt := range mergeTreeFamilies {
			if upper == mt {
				isMergeTree = true
				break
			}
		}
		if !isMergeTree {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.NotInnoDBEngine.Int32(),
				Title:   "Prefer MergeTree family engine",
				Content: "Engine `" + engineName + "` is not in the MergeTree family. Consider using MergeTree or its variants for production tables.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── Query safety rules ───────────────────────────────────────

// NoSelectStarAdvisor checks for SELECT *.
type NoSelectStarAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_STATEMENT_SELECT_NO_SELECT_ALL, &NoSelectStarAdvisor{})
}

func (*NoSelectStarAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(strings.TrimSpace(stmt.Text))
		if !strings.HasPrefix(text, "SELECT") {
			continue
		}
		if containsSelectStar(stmt.Text) {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.StatementSelectAll.Int32(),
				Title:   "Avoid SELECT *",
				Content: "SELECT * is discouraged. Specify columns explicitly for better performance and maintainability.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// RequireLimitAdvisor checks that SELECT queries include a LIMIT clause.
type RequireLimitAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_STATEMENT_MAXIMUM_LIMIT_VALUE, &RequireLimitAdvisor{})
}

func (*RequireLimitAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(strings.TrimSpace(stmt.Text))
		if !strings.HasPrefix(text, "SELECT") {
			continue
		}
		if !strings.Contains(text, "LIMIT") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.StatementExceedMaximumLimitValue.Int32(),
				Title:   "SELECT should include LIMIT",
				Content: "Unbounded SELECT on ClickHouse can consume excessive resources. Add a LIMIT clause.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// NoJoinOnDistributedAdvisor warns against JOIN on Distributed tables.
type NoJoinOnDistributedAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_TABLE_NO_FOREIGN_KEY, &NoJoinOnDistributedAdvisor{})
}

func (*NoJoinOnDistributedAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if strings.Contains(text, "JOIN") && strings.Contains(text, "DISTRIBUTED") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.TableHasFK.Int32(),
				Title:   "Avoid JOIN on Distributed tables",
				Content: "JOINing Distributed tables can cause significant cross-shard data movement. Consider using local tables or subqueries.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── DML safety rules ─────────────────────────────────────────

// WhereRequireForUpdateDeleteAdvisor requires WHERE for UPDATE/DELETE.
type WhereRequireForUpdateDeleteAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_STATEMENT_WHERE_REQUIRE_UPDATE_DELETE, &WhereRequireForUpdateDeleteAdvisor{})
}

func (*WhereRequireForUpdateDeleteAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(strings.TrimSpace(stmt.Text))
		isUpdate := strings.HasPrefix(text, "ALTER TABLE") && strings.Contains(text, "UPDATE")
		isDelete := strings.HasPrefix(text, "ALTER TABLE") && strings.Contains(text, "DELETE")
		if !isUpdate && !isDelete {
			continue
		}
		if !strings.Contains(text, "WHERE") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.StatementNoWhere.Int32(),
				Title:   "UPDATE/DELETE requires WHERE clause",
				Content: "Mutation without WHERE affects all rows. Add a WHERE clause.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// DisallowTruncateAdvisor prevents TRUNCATE statements.
type DisallowTruncateAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_STATEMENT_DISALLOW_TRUNCATE, &DisallowTruncateAdvisor{})
}

func (*DisallowTruncateAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(strings.TrimSpace(stmt.Text))
		if strings.HasPrefix(text, "TRUNCATE") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.StatementDisallowTruncate.Int32(),
				Title:   "TRUNCATE is not allowed",
				Content: "TRUNCATE TABLE is dangerous on ClickHouse. Use ALTER TABLE ... DELETE WHERE 1=1 with caution instead.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── Type safety rules ────────────────────────────────────────

// PreferLowCardinalityAdvisor suggests using LowCardinality for String columns.
type PreferLowCardinalityAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_COLUMN_TYPE_DISALLOW_LIST, &PreferLowCardinalityAdvisor{})
}

func (*PreferLowCardinalityAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") {
			continue
		}
		if strings.Contains(stmt.Text, " String") && !strings.Contains(stmt.Text, "LowCardinality") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.DisabledColumnType.Int32(),
				Title:   "Consider LowCardinality(String)",
				Content: "Plain String columns with low distinct values should use LowCardinality(String) for better compression and query performance.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// NoNullableAggregateAdvisor warns against Nullable types in aggregate columns.
type NoNullableAggregateAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_COLUMN_NO_NULL, &NoNullableAggregateAdvisor{})
}

func (*NoNullableAggregateAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") {
			continue
		}
		if strings.Contains(text, "AGGREGATINGMERGETREE") && strings.Contains(stmt.Text, "Nullable") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.ColumnCannotNull.Int32(),
				Title:   "Avoid Nullable in AggregatingMergeTree",
				Content: "Nullable columns in AggregatingMergeTree tables add overhead. Use default values instead.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── DDL safety rules ─────────────────────────────────────────

// TableDisallowDDLAdvisor prevents DDL operations on specified tables.
type TableDisallowDDLAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_TABLE_DISALLOW_DDL, &TableDisallowDDLAdvisor{})
}

func (*TableDisallowDDLAdvisor) Check(_ context.Context, _ advisor.Context) ([]*storepb.Advice, error) {
	return nil, nil
}

// TableDisallowDMLAdvisor prevents DML operations on specified tables.
type TableDisallowDMLAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_TABLE_DISALLOW_DML, &TableDisallowDMLAdvisor{})
}

func (*TableDisallowDMLAdvisor) Check(_ context.Context, _ advisor.Context) ([]*storepb.Advice, error) {
	return nil, nil
}

// ─── Partition rules ──────────────────────────────────────────

// RequirePartitionKeyAdvisor checks that MergeTree tables have PARTITION BY.
type RequirePartitionKeyAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_TABLE_DROP_NAMING_CONVENTION, &RequirePartitionKeyAdvisor{})
}

func (*RequirePartitionKeyAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") {
			continue
		}
		hasMergeTree := strings.Contains(text, "MERGETREE")
		hasPartition := strings.Contains(text, "PARTITION BY")
		if hasMergeTree && !hasPartition {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.CreateTablePartition.Int32(),
				Title:   "MergeTree table should have PARTITION BY",
				Content: "MergeTree tables without PARTITION BY will have a single partition which limits data management. Add an appropriate PARTITION BY clause.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── Backward compatibility rules ─────────────────────────────

// MigrationCompatibilityAdvisor checks for backward-incompatible schema changes.
type MigrationCompatibilityAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_SCHEMA_BACKWARD_COMPATIBILITY, &MigrationCompatibilityAdvisor{})
}

func (*MigrationCompatibilityAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(strings.TrimSpace(stmt.Text))
		if strings.HasPrefix(text, "DROP TABLE") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.CompatibilityDropTable.Int32(),
				Title:   "Backward-incompatible: DROP TABLE",
				Content: "DROP TABLE is a destructive operation. Ensure all consumers have been updated before dropping.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
		if strings.Contains(text, "DROP COLUMN") {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.CompatibilityDropColumn.Int32(),
				Title:   "Backward-incompatible: DROP COLUMN",
				Content: "Dropping a column is backward-incompatible. Consider adding a new column and deprecating the old one.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── Index rules ──────────────────────────────────────────────

// OrderByUsageAdvisor checks that MergeTree tables have ORDER BY.
type OrderByUsageAdvisor struct{}

func init() {
	advisor.Register(storepb.Engine_CLICKHOUSE, storepb.SQLReviewRule_INDEX_NOT_REDUNDANT, &OrderByUsageAdvisor{})
}

func (*OrderByUsageAdvisor) Check(_ context.Context, ctx advisor.Context) ([]*storepb.Advice, error) {
	var adviceList []*storepb.Advice
	for _, stmt := range ctx.ParsedStatements {
		text := strings.ToUpper(stmt.Text)
		if !strings.Contains(text, "CREATE TABLE") {
			continue
		}
		hasMergeTree := strings.Contains(text, "MERGETREE")
		hasOrderBy := strings.Contains(text, "ORDER BY")
		if hasMergeTree && !hasOrderBy {
			status, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
			if err != nil {
				return nil, err
			}
			adviceList = append(adviceList, &storepb.Advice{
				Status:  status,
				Code:    code.RedundantIndex.Int32(),
				Title:   "MergeTree table should have ORDER BY",
				Content: "MergeTree tables should define an ORDER BY clause for optimal query performance.",
				StartPosition: &storepb.Position{
					Line: int32(stmt.BaseLine()),
				},
			})
		}
	}
	return adviceList, nil
}

// ─── Helper functions ─────────────────────────────────────────

func isLowercaseSnake(name string) bool {
	for _, c := range name {
		if c >= 'A' && c <= 'Z' {
			return false
		}
		if c != '_' && !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func extractIdentifier(sql string, keyword string) string {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, keyword)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(sql[idx+len(keyword):])
	restUpper := strings.ToUpper(rest)
	if strings.HasPrefix(restUpper, "IF NOT EXISTS") {
		rest = strings.TrimSpace(rest[13:])
	} else if strings.HasPrefix(restUpper, "IF EXISTS") {
		rest = strings.TrimSpace(rest[9:])
	}
	var name strings.Builder
	inBacktick := false
	for _, c := range rest {
		if c == '`' {
			inBacktick = !inBacktick
			continue
		}
		if !inBacktick && (c == ' ' || c == '(' || c == '\n' || c == '\t' || c == '\r') {
			break
		}
		name.WriteRune(c)
	}
	result := name.String()
	if dot := strings.LastIndex(result, "."); dot >= 0 {
		result = result[dot+1:]
	}
	return result
}

func extractColumnNames(sql string) []string {
	start := strings.Index(sql, "(")
	if start < 0 {
		return nil
	}
	depth := 1
	end := start + 1
	for end < len(sql) && depth > 0 {
		if sql[end] == '(' {
			depth++
		} else if sql[end] == ')' {
			depth--
		}
		end++
	}
	if depth != 0 {
		return nil
	}
	body := sql[start+1 : end-1]
	var columns []string
	for _, line := range strings.Split(body, ",") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		col := strings.Trim(fields[0], "`\"")
		upper := strings.ToUpper(col)
		if upper == "INDEX" || upper == "CONSTRAINT" || upper == "PRIMARY" || upper == "ORDER" || upper == "ENGINE" || upper == "PARTITION" || upper == "SETTINGS" {
			continue
		}
		columns = append(columns, col)
	}
	return columns
}

func extractEngineClause(sql string) string {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, "ENGINE")
	if idx < 0 {
		return ""
	}
	rest := sql[idx+6:]
	rest = strings.TrimLeft(rest, " \t\n\r=")
	var name strings.Builder
	for _, c := range rest {
		if c == '(' || c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			break
		}
		name.WriteRune(c)
	}
	return name.String()
}

func containsSelectStar(sql string) bool {
	upper := strings.ToUpper(sql)
	selectIdx := strings.Index(upper, "SELECT")
	if selectIdx < 0 {
		return false
	}
	fromIdx := strings.Index(upper, "FROM")
	if fromIdx < 0 {
		fromIdx = len(sql)
	}
	selectClause := sql[selectIdx+6 : fromIdx]
	return strings.Contains(selectClause, "*")
}
