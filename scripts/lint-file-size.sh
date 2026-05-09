#!/bin/bash
# lint-file-size.sh — CI script to enforce maximum file size in service layer.
# Usage: scripts/lint-file-size.sh [MAX_LINES] [TARGET_DIR]
#
# Blocks PRs introducing .go files exceeding MAX_LINES in TARGET_DIR.
# Test files (*_test.go) are excluded.
# Generated files (containing "Code generated" on line 1) are excluded.
#
# Examples:
#   scripts/lint-file-size.sh               # 800 lines, backend/api/v1/
#   scripts/lint-file-size.sh 600           # 600 lines, backend/api/v1/
#   scripts/lint-file-size.sh 800 backend/  # 800 lines, all of backend/

set -euo pipefail

MAX_LINES=${1:-800}
TARGET_DIR=${2:-"backend/api/v1"}
ERRORS=0

echo "🔍 Checking .go files in $TARGET_DIR (max: $MAX_LINES lines)"
echo ""

for f in $(find "$TARGET_DIR" -name '*.go' -not -name '*_test.go' -not -name 'mock_*.go' | sort); do
    # Skip generated files
    if head -1 "$f" | grep -q "Code generated"; then
        continue
    fi

    lines=$(wc -l < "$f" | tr -d ' ')
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ $f: $lines lines (exceeds $MAX_LINES)"
        ERRORS=$((ERRORS + 1))
    fi
done

echo ""
if [ "$ERRORS" -gt 0 ]; then
    echo "❌ $ERRORS file(s) exceed the $MAX_LINES-line limit."
    echo "   Consider splitting large files into domain-focused sub-files."
    echo "   See: specs/crs/v1/architecture/tasks/T-010-01-split-auth.md"
    exit 1
else
    echo "✅ All files within $MAX_LINES-line limit."
    exit 0
fi
