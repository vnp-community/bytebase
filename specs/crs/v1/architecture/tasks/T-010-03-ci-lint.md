# T-010-03: CI File Size Lint Script

| Field | Value |
|---|---|
| **Task ID** | T-010-03 |
| **Solution** | SOL-ARCH-010 |
| **Priority** | P2 |
| **Depends On** | None |
| **Target File** | `scripts/lint-file-size.sh` |
| **Type** | New file |

---

## Objective

CI enforcement: block PRs introducing `.go` files > 800 lines in `backend/api/v1/`.

## Implementation

```bash
#!/bin/bash
MAX_LINES=${1:-800}
TARGET_DIR="backend/api/v1"
ERRORS=0
for f in $(find "$TARGET_DIR" -name '*.go' -not -name '*_test.go' | sort); do
    lines=$(wc -l < "$f" | tr -d ' ')
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ $f: $lines lines (exceeds $MAX_LINES)"
        ERRORS=$((ERRORS + 1))
    fi
done
exit $ERRORS
```

## Acceptance Criteria

- [ ] Script created and executable (`chmod +x`)
- [ ] Returns 0 when all files ≤ 800 lines
- [ ] Returns non-zero with violation list
- [ ] Test files excluded
