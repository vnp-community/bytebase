//go:build enterprise_core

// AI-CONTEXT: Build Profile = "enterprise_core"
// AI-CONTEXT: This file is compiled ONLY when: -tags enterprise_core
// AI-CONTEXT: Available engines: PostgreSQL, MySQL/MariaDB/OceanBase, MSSQL, Oracle, CockroachDB, Redis (6 core)
// AI-CONTEXT: Available plugins: core parsers (mysql, pg, plsql, tsql), core advisors, core schema, all webhooks
// AI-CONTEXT: Excluded: BigQuery, Cassandra, ClickHouse, CosmosDB, Databricks, DynamoDB,
//             Elasticsearch, Hive, MongoDB, Redshift, Snowflake, Spanner, SQLite, StarRocks, TiDB, Trino
// AI-CONTEXT: See BUILD_PROFILES.md for full profile comparison.

package server

// enterprise_core.go includes only the core SQL database engines commonly used
// in enterprise/financial environments. This reduces binary size by ~40% compared
// to the ultimate build profile.
//
// Build with: go build -tags enterprise_core ...
//
// Included engines: PostgreSQL, MySQL/MariaDB/OceanBase, MSSQL, Oracle, CockroachDB, Redis
// Excluded engines: BigQuery, Cassandra, ClickHouse, CosmosDB, Databricks, DynamoDB,
//                   Elasticsearch, Hive, MongoDB, Redshift, Snowflake, Spanner, SQLite,
//                   StarRocks, TiDB, Trino

import (
	// Core SQL Drivers.
	_ "github.com/bytebase/bytebase/backend/plugin/db/cockroachdb"
	_ "github.com/bytebase/bytebase/backend/plugin/db/mssql"
	_ "github.com/bytebase/bytebase/backend/plugin/db/mysql"
	_ "github.com/bytebase/bytebase/backend/plugin/db/oracle"
	_ "github.com/bytebase/bytebase/backend/plugin/db/pg"
	_ "github.com/bytebase/bytebase/backend/plugin/db/redis"

	// Core SQL Parsers.
	_ "github.com/bytebase/bytebase/backend/plugin/parser/mysql"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/pg"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/plsql"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/tsql"

	// Core SQL Advisors.
	_ "github.com/bytebase/bytebase/backend/plugin/advisor/mssql"
	_ "github.com/bytebase/bytebase/backend/plugin/advisor/mysql"
	_ "github.com/bytebase/bytebase/backend/plugin/advisor/oracle"
	_ "github.com/bytebase/bytebase/backend/plugin/advisor/pg"

	// Core Schema designers.
	_ "github.com/bytebase/bytebase/backend/plugin/schema/mssql"
	_ "github.com/bytebase/bytebase/backend/plugin/schema/mysql"
	_ "github.com/bytebase/bytebase/backend/plugin/schema/oracle"
	_ "github.com/bytebase/bytebase/backend/plugin/schema/pg"

	// IM webhooks — all included.
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/dingtalk"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/discord"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/feishu"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/googlechat"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/lark"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/slack"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/teams"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/wecom"
)
