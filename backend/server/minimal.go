//go:build minidemo

// AI-CONTEXT: Build Profile = "minidemo"
// AI-CONTEXT: This file is compiled ONLY when: -tags minidemo
// AI-CONTEXT: Available engines: PostgreSQL ONLY (1 engine)
// AI-CONTEXT: Available plugins: pg parser, pg advisor, pg schema, limited webhooks (dingtalk, feishu, googlechat, slack, wecom)
// AI-CONTEXT: Excluded: ALL non-PostgreSQL engines and their plugins
// AI-CONTEXT: See BUILD_PROFILES.md for full profile comparison.

package server

import (
	// This includes the first-class database, Postgres.

	// Drivers.
	_ "github.com/bytebase/bytebase/backend/plugin/db/pg"

	// Parsers.
	_ "github.com/bytebase/bytebase/backend/plugin/parser/pg"

	// Schema designer.
	_ "github.com/bytebase/bytebase/backend/plugin/schema/pg"

	// Advisors.
	_ "github.com/bytebase/bytebase/backend/plugin/advisor/pg"

	// IM webhooks.
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/dingtalk"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/feishu"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/googlechat"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/slack"
	_ "github.com/bytebase/bytebase/backend/plugin/webhook/wecom"
)
