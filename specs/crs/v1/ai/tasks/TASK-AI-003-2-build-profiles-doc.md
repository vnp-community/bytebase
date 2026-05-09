# TASK-AI-003-2: BUILD_PROFILES.md Documentation

| Field | Value |
|-------|-------|
| Solution | SOL-AI-003 |
| Priority | P0 |
| Depends On | — |
| Est. | S (documentation only) |
| **Status** | **✅ DONE** (2026-05-09) |

## Objective

Create `BUILD_PROFILES.md` co-located with build-tagged files to document the 3 binary profiles.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/server/BUILD_PROFILES.md` |

## Specification

Include:
1. Profile comparison matrix (engines × profiles)
2. Build commands for each profile
3. File → profile mapping
4. Plugin availability per profile

Reference SOL-AI-003 §2.1 for full content.

### Verification

Manual review — documentation only.

## Acceptance Criteria

- [ ] Document covers all 3 profiles (ultimate, enterprise_core, minidemo)
- [ ] Engine/plugin availability matrix complete
- [ ] Build commands documented
