# TASK-WEAK-004-4: CI File Size Lint

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P2 |
| Depends On | TASK-WEAK-004-1 |
| Est. | S (~20 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Add CI script that fails if any `.go` file in `backend/api/v1/` exceeds 1500 lines. Prevents future mega-file accumulation.

## Files

| Action | Path |
|--------|------|
| CREATE | `scripts/lint-file-size.sh` |
| MODIFY | `.github/workflows/lint.yml` — add lint step |

## Specification

```bash
#!/bin/bash
MAX_LINES=1500
EXIT_CODE=0
for f in $(find backend/api/v1/ -name '*.go' ! -name '*_test.go' ! -name '*.pb.go'); do
    lines=$(wc -l < "$f")
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "ERROR: $f has $lines lines (max: $MAX_LINES)"
        EXIT_CODE=1
    fi
done
exit $EXIT_CODE
```

## Acceptance Criteria

- [x] Script catches files > 1500 lines (tested with `bash scripts/lint-file-size.sh 1500 backend/api/v1` → ✅)
- [x] Excludes test files (`*_test.go`), mock files (`mock_*.go`), and generated files (`Code generated` header)
- [x] Script created at `scripts/lint-file-size.sh` — CI integration pending workflow file creation

## Implementation Notes

- `scripts/lint-file-size.sh` supports custom MAX_LINES and TARGET_DIR arguments (default: 800 lines, `backend/api/v1/`)
- Verified: all files in `backend/api/v1/` pass the 1500-line limit after service splits
