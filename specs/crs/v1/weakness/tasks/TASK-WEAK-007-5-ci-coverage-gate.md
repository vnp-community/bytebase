# TASK-WEAK-007-5: CI Coverage Gate Workflow

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P1 |
| Depends On | — |
| Est. | S (~40 LoC) |

## Objective

Add GitHub Actions workflow that fails PR if unit test coverage drops below threshold (50% initial, incrementally raised).

## Files

| Action | Path |
|--------|------|
| CREATE | `.github/workflows/coverage.yml` |

## Specification

```yaml
name: Coverage Gate
on: [pull_request]
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
          go tool cover -func=coverage.out | \
          awk '/^total:/ { gsub(/%/,"",$3); if ($3 < 50) { print "FAIL: " $3 "% < 50%"; exit 1 } }'
      - uses: codecov/codecov-action@v4
        with: { file: coverage.out }
```

## Acceptance Criteria

- [ ] Workflow triggers on PR
- [ ] Fails if coverage < 50%
- [ ] Coverage report uploaded to Codecov
- [ ] Runs only unit tests (no testcontainers)
