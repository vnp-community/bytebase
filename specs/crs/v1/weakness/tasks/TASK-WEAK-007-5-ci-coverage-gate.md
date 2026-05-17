# TASK-WEAK-007-5: CI Coverage Gate Workflow

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P1 |
| Depends On | — |
| Est. | S (~40 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Add GitHub Actions workflow that fails PR if unit test coverage drops below threshold (50% initial, incrementally raised).

## Files

| Action | Path |
|--------|------|
| CREATE | `.github/workflows/coverage.yml` |

## Implementation Notes

### Workflow: `.github/workflows/coverage.yml`

```yaml
name: Coverage Gate
on: [pull_request]
permissions:
  contents: read
jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - name: Run unit tests with coverage
        run: |
          go test -coverprofile=coverage.out -covermode=atomic \
            ./backend/api/v1/... \
            ./backend/component/... \
            ./backend/store/...
      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | awk '/^total:/ { gsub(/%/,"",$3); print $3 }')
          echo "Total coverage: ${COVERAGE}%"
          if [ "$(echo "$COVERAGE < 50" | bc -l)" -eq 1 ]; then
            echo "::error::FAIL: coverage ${COVERAGE}% is below the 50% threshold"
            exit 1
          fi
          echo "::notice::Coverage ${COVERAGE}% meets the 50% threshold"
      - uses: codecov/codecov-action@v4
        if: always()
        with:
          file: coverage.out
          fail_ci_if_error: false
```

### Key Design Decisions

1. **50% threshold** — initial floor, intended to be incrementally raised as test suite grows
2. **Unit tests only** — no testcontainers or DB dependencies for fast CI feedback (~30s)
3. **Scoped packages** — only `backend/api/v1`, `backend/component`, `backend/store` (core domain)
4. **Codecov upload** — `always()` condition ensures report is uploaded even on threshold failure
5. **`bc -l` comparison** — handles decimal coverage percentages correctly

## Acceptance Criteria

- [x] Workflow triggers on PR
- [x] Fails if coverage < 50%
- [x] Coverage report uploaded to Codecov
- [x] Runs only unit tests (no testcontainers)
