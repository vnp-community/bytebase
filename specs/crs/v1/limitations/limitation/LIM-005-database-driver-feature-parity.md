# LIM-005 — Database Driver Feature Parity Gap

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | LIM-005                                    |
| Category       | Feature Coverage                           |
| Severity       | MEDIUM                                     |
| Affected Layer | L7 (Plugin — DB Drivers)                   |
| Source Files   | `backend/plugin/db/*/`                     |

---

## Mô tả

Bytebase hỗ trợ **22 database engines** qua plugin driver architecture. Tuy nhiên, không phải tất cả engines đều có feature coverage ngang nhau.

## Chi tiết hạn chế

### 1. SQL Advisor Coverage Gap

```
SQL Advisor hỗ trợ đầy đủ: PostgreSQL, MySQL, TiDB, Oracle, MSSQL, Snowflake, OceanBase, Redshift
Không có SQL Advisor:       MongoDB, Redis, Cassandra, DynamoDB, Elasticsearch, CosmosDB,
                            ClickHouse, BigQuery, Spanner, Trino, Hive, Databricks, StarRocks, SQLite
```

- **14/22 engines** không có SQL review/lint rules.
- NoSQL databases (MongoDB, Redis, DynamoDB, etc.) không có SQL concept → advisor không áp dụng.
- Một số SQL databases (ClickHouse, BigQuery) thiếu advisor rules.

### 2. Schema Dump Coverage

- Dump (schema export) không đầy đủ cho tất cả engines.
- NoSQL engines không có structured schema dump.
- DynamoDB, Elasticsearch chỉ có partial metadata sync.

### 3. Prior Backup Support

```go
// backend/common/engine.go
func EngineSupportPriorBackup(engine storepb.Engine) bool {
    // Chỉ một số engines hỗ trợ
}
```

- Prior backup (automatic backup before data changes) chỉ hỗ trợ một số engines.
- One-click rollback phụ thuộc vào prior backup → cũng bị giới hạn.

### 4. Online Schema Change

- **gh-ost** chỉ hỗ trợ **MySQL** (và fork `bytebase/gh-ost2`).
- PostgreSQL, TiDB và các engines khác không có online schema change tương đương.

### 5. Data Masking Coverage

```
Full masking:     SQL databases (column-level)
Document masking: MongoDB, CosmosDB, Elasticsearch (JSON path)
No masking:       Redis, DynamoDB, Cassandra
```

### 6. Parser Limitations (TODO/FIXME markers)

Từ source code analysis, nhiều parsers có acknowledged gaps:

| Engine       | Known Gaps                                                    |
|--------------|--------------------------------------------------------------|
| Oracle/PLSQL | Bind variables, xmltable, USING clause chưa xử lý           |
| Spanner      | Dashed path expressions, recursive CTE detection             |
| BigQuery     | UNION alias handling                                         |
| Redshift     | Cross-database queries chưa hỗ trợ                          |
| CockroachDB  | Expression extraction chưa hoàn thiện                        |
| StarRocks    | Parser compatibility issues, index information missing       |

## Impact Matrix

| Feature              | PG | MySQL | TiDB | Oracle | MSSQL | MongoDB | Redis | ClickHouse | Snowflake |
|---------------------|----|-------|------|--------|-------|---------|-------|------------|-----------|
| SQL Review (200+)    | ✅ | ✅    | ✅   | ✅     | ✅    | ❌      | ❌    | ❌         | ✅        |
| Schema Dump          | ✅ | ✅    | ✅   | ✅     | ✅    | ⚠️      | ❌    | ✅         | ✅        |
| Prior Backup         | ✅ | ✅    | ⚠️   | ⚠️     | ⚠️    | ❌      | ❌    | ❌         | ❌        |
| Online Schema Change | ❌ | ✅    | ❌   | ❌     | ❌    | ❌      | ❌    | ❌         | ❌        |
| Data Masking         | ✅ | ✅    | ✅   | ✅     | ✅    | ✅      | ❌    | ✅         | ✅        |

## Khuyến nghị

1. Document rõ feature matrix per engine trên UI/docs.
2. Ưu tiên advisor rules cho ClickHouse, BigQuery (SQL-capable engines).
3. Cân nhắc online schema change cho PostgreSQL (pg-osc hoặc pgroll).
4. Giải quyết parser FIXME/TODO items theo priority.
