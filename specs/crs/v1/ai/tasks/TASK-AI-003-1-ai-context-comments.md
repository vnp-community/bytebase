# TASK-AI-003-1: AI-CONTEXT Comments in Build-Tagged Files

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | — |
| Est. | S (~5 comment blocks, 0 functional change) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Add structured `// AI-CONTEXT:` comment blocks to all 5 build-tagged files. Zero functional change — comments only.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/ultimate.go` — add AI-CONTEXT block |
| MODIFY | `backend/server/enterprise_core.go` — add AI-CONTEXT block |
| MODIFY | `backend/server/minimal.go` — add AI-CONTEXT block |
| MODIFY | `backend/common/config_dev.go` — add AI-CONTEXT block |
| MODIFY | `backend/common/config_release.go` — add AI-CONTEXT block |

## Specification

Insert after the `//go:build` directive and before `package`:

```go
// AI-CONTEXT: Build Profile = "{profile_name}"
// AI-CONTEXT: This file is compiled when: {build_tag_condition}
// AI-CONTEXT: Available engines: {engine_list}
// AI-CONTEXT: Available plugins: {plugin_list}
// AI-CONTEXT: See BUILD_PROFILES.md for full profile comparison.
```

### Verification

```bash
go build ./backend/...
go build -tags enterprise_core ./backend/...
go build -tags minidemo ./backend/...
```

## Acceptance Criteria

- [x] 5 files have AI-CONTEXT comment blocks
- [x] All 3 build profiles compile successfully
- [x] Zero functional changes
