# T-011-02: Makefile Build Profiles

| Field | Value |
|---|---|
| **Task ID** | T-011-02 |
| **Solution** | SOL-ARCH-011 |
| **Priority** | P3 |
| **Depends On** | T-011-01 |
| **Target File** | `Makefile` |
| **Type** | Modify existing |

---

## Objective

Add build profiles to Makefile for different driver selections. Reduces binary size for deployment variants.

## Implementation

```makefile
.PHONY: build build-relational build-cloud build-minimal

# Default: all 23 drivers
build:
	go build -tags plugin_all -o bytebase ./backend/cmd/server

# Relational DBs only (~60% binary size)
build-relational:
	go build -tags "plugin_pg,plugin_mysql,plugin_mssql,plugin_oracle,plugin_sqlite" \
		-o bytebase-relational ./backend/cmd/server

# Cloud warehouses
build-cloud:
	go build -tags "plugin_pg,plugin_mysql,plugin_snowflake,plugin_bigquery,plugin_redshift" \
		-o bytebase-cloud ./backend/cmd/server

# Minimal: PostgreSQL only (~40% binary size)
build-minimal:
	go build -tags "plugin_pg" -o bytebase-minimal ./backend/cmd/server
```

## Acceptance Criteria

- [ ] 4 build targets in Makefile
- [ ] `make build` → full binary (backward compatible)
- [ ] `make build-minimal` → PG-only binary
- [ ] Binary size comparison documented
