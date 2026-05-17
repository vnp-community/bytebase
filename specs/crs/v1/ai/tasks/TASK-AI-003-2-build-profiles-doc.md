# TASK-AI-003-2: BUILD_PROFILES.md Documentation

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2026-05-09 |
| Verified | 2026-05-10 |
| Est. | S (documentation only) |

## Objective

Create `BUILD_PROFILES.md` co-located with build-tagged files to document the 3 binary profiles.

## Delivered

**File**: `backend/server/BUILD_PROFILES.md` (79 lines, 2833 bytes)

Covers:
1. **Profile comparison matrix** — build tags, source files, engines, parsers, advisors, binary size
2. **Engine availability matrix** — 24 engines × 3 profiles
3. **Build commands** — ultimate, enterprise_core, minidemo, release combos
4. **File → profile mapping** — 5 build-tagged files with constraints
5. **Mutual exclusion rules** — only ONE profile compiled per binary

### Verification (2026-05-10 re-verified)

Manual review — documentation only. Content matches actual build behavior.

## Acceptance Criteria

- [x] Document covers all 3 profiles (ultimate, enterprise_core, minidemo)
- [x] Engine/plugin availability matrix complete (24 engines)
- [x] Build commands documented (6 variants)
