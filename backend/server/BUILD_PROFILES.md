# Build Profiles — Bytebase Server

> Quick reference for AI agents and developers on the 3 binary profiles.

## Profile Comparison

| Attribute | `ultimate` (default) | `enterprise_core` | `minidemo` |
|-----------|---------------------|-------------------|------------|
| Build tag | _(none)_ | `-tags enterprise_core` | `-tags minidemo` |
| Source file | `ultimate.go` | `enterprise_core.go` | `minimal.go` |
| DB engines | ALL 22+ | 6 core (PG, MySQL, MSSQL, Oracle, CockroachDB, Redis) | PostgreSQL only |
| Parsers | ALL 18 | 4 (mysql, pg, plsql, tsql) | 1 (pg) |
| Advisors | ALL 8 | 4 (mssql, mysql, oracle, pg) | 1 (pg) |
| Schema designers | ALL 8 | 4 (mssql, mysql, oracle, pg) | 1 (pg) |
| Webhooks | ALL 8 | ALL 8 | 5 (dingtalk, feishu, googlechat, slack, wecom) |
| Binary size | ~100% | ~60% | ~30% |

## Engine Availability Matrix

| Engine | `ultimate` | `enterprise_core` | `minidemo` |
|--------|-----------|-------------------|------------|
| PostgreSQL | ✅ | ✅ | ✅ |
| MySQL | ✅ | ✅ | ❌ |
| MariaDB | ✅ | ✅ (via MySQL) | ❌ |
| OceanBase | ✅ | ✅ (via MySQL) | ❌ |
| MSSQL | ✅ | ✅ | ❌ |
| Oracle | ✅ | ✅ | ❌ |
| CockroachDB | ✅ | ✅ | ❌ |
| Redis | ✅ | ✅ | ❌ |
| TiDB | ✅ | ❌ | ❌ |
| Snowflake | ✅ | ❌ | ❌ |
| ClickHouse | ✅ | ❌ | ❌ |
| MongoDB | ✅ | ❌ | ❌ |
| Spanner | ✅ | ❌ | ❌ |
| BigQuery | ✅ | ❌ | ❌ |
| Redshift | ✅ | ❌ | ❌ |
| SQLite | ✅ | ❌ | ❌ |
| StarRocks | ✅ | ❌ | ❌ |
| Hive | ✅ | ❌ | ❌ |
| Cassandra | ✅ | ❌ | ❌ |
| DynamoDB | ✅ | ❌ | ❌ |
| Elasticsearch | ✅ | ❌ | ❌ |
| CosmosDB | ✅ | ❌ | ❌ |
| Databricks | ✅ | ❌ | ❌ |
| Trino | ✅ | ❌ | ❌ |

## Build Commands

```bash
# Ultimate (default) — all engines
go build ./backend/...

# Enterprise Core — core SQL engines only
go build -tags enterprise_core ./backend/...

# MiniDemo — PostgreSQL only
go build -tags minidemo ./backend/...

# Release mode (combinable with any profile)
go build -tags "release" ./backend/...
go build -tags "release,enterprise_core" ./backend/...
```

## File → Profile Mapping

| File | Build constraint | Active when |
|------|-----------------|-------------|
| `ultimate.go` | `!minidemo && !enterprise_core` | No special tags |
| `enterprise_core.go` | `enterprise_core` | `-tags enterprise_core` |
| `minimal.go` | `minidemo` | `-tags minidemo` |
| `config_dev.go` | `!release` | No release tag |
| `config_release.go` | `release` | `-tags release` |

## Mutual Exclusion Rules

- `ultimate.go`, `enterprise_core.go`, and `minimal.go` are **mutually exclusive**
- Only ONE is compiled per binary
- `config_dev.go` and `config_release.go` are also mutually exclusive
- Build tags can be combined: `-tags "enterprise_core,release"` = core engines + production mode
