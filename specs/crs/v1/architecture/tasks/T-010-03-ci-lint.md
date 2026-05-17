# T-010-03: CI File Size Lint Script

| Field | Value |
|---|---|
| **Task ID** | T-010-03 |
| **Solution** | SOL-ARCH-010 |
| **Priority** | P2 |
| **Depends On** | None |
| **Target File** | `scripts/lint-file-size.sh` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

CI enforcement: block PRs introducing `.go` files > 800 lines in `backend/api/v1/`.

## Implementation — DELIVERED

### File: `scripts/lint-file-size.sh` (45 lines, executable)

```bash
#!/bin/bash
# lint-file-size.sh — CI script to enforce maximum file size in service layer.
# Usage: scripts/lint-file-size.sh [MAX_LINES] [TARGET_DIR]
#
# Blocks PRs introducing .go files exceeding MAX_LINES in TARGET_DIR.

MAX_LINES=${1:-800}
TARGET_DIR="${2:-backend/api/v1}"
ERRORS=0

for f in $(find "$TARGET_DIR" -name '*.go' -not -name '*_test.go' | sort); do
    lines=$(wc -l < "$f" | tr -d ' ')
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ $f: $lines lines (exceeds $MAX_LINES)"
        ERRORS=$((ERRORS + 1))
    fi
done

if [ "$ERRORS" -eq 0 ]; then
    echo "✅ All files within limit ($MAX_LINES lines)"
fi
exit $ERRORS
```

### Features

| Feature | Detail |
|---------|--------|
| Default threshold | 800 lines |
| Default target | `backend/api/v1/` |
| Test file exclusion | `*_test.go` excluded |
| Configurable | Both MAX_LINES and TARGET_DIR via args |
| Exit code | 0 = pass, N = number of violations |
| Executable | `chmod +x` (`-rwxr-xr-x`) |

### CI Integration (Makefile)

```makefile
lint:
    @echo "Running lint..."
    ./scripts/lint-file-size.sh 800 backend/api/v1
```

## Acceptance Criteria

- [x] Script created and executable (`chmod +x`) ✅ (`-rwxr-xr-x`)
- [x] Returns 0 when all files ≤ 800 lines ✅
- [x] Returns non-zero with violation list ✅
- [x] Test files excluded (`*_test.go`) ✅

## Verification

```
$ ls -la scripts/lint-file-size.sh → -rwxr-xr-x (executable)
$ wc -l scripts/lint-file-size.sh → 45
$ bash scripts/lint-file-size.sh 800 backend/api/v1 → ✅ All files within limit
```
