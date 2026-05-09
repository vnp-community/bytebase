# TASK-WEAK-004-4: CI File Size Lint

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P2 |
| Depends On | TASK-WEAK-004-1 |
| Est. | S (~20 LoC) |

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

- [ ] Script catches files > 1500 lines
- [ ] Excludes test files and generated protobuf files
- [ ] CI step runs on PR and fails pipeline if violated
