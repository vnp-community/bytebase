# Solution: CR-SEC-017 — Dependency Vulnerability Scanning Pipeline

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-SEC-017                |
| **Solution**   | SOL-SEC-017               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Triển khai multi-layer vulnerability scanning trong CI/CD pipeline: `govulncheck` cho Go (backend/), `npm audit` cho frontend (Vue/React), `trivy` cho container images. Pre-commit hooks via `gitleaks` cho secret scanning. SBOM generation via `syft`. Toàn bộ pipeline-level — không ảnh hưởng runtime architecture (L1-L10).

---

## 2. Architectural Alignment

```
Pipeline-level changes (không ảnh hưởng L1-L10 runtime):

┌─────────────────────────────────────────┐
│  Pre-commit Hooks                        │
│  gitleaks → secret detection             │
│  gosec → Go security linting             │
├─────────────────────────────────────────┤
│  CI Pipeline (PR gate)                   │
│  govulncheck → Go vulnerability scan    │
│  npm audit → npm vulnerability scan      │
│  trivy → container image scan            │
│  go-licenses → license compliance        │
├─────────────────────────────────────────┤
│  Release Pipeline                        │
│  syft → SBOM generation (CycloneDX)     │
│  cosign → image signing                  │
├─────────────────────────────────────────┤
│  Scheduled (Daily)                       │
│  govulncheck → daily Go scan             │
│  trivy → daily image scan                │
│  alerting → Slack/PagerDuty              │
└─────────────────────────────────────────┘
```

---

## 3. Chi tiết Implementation

### 3.1 Pre-commit Hooks

**File**: `.pre-commit-config.yaml` (new)

```yaml
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.18.0
    hooks:
      - id: gitleaks
        name: Detect hardcoded secrets

  - repo: local
    hooks:
      - id: gosec
        name: Go security linter
        entry: gosec -exclude=G101,G304 -fmt=text ./backend/...
        language: system
        types: [go]

      - id: sql-injection-check
        name: SQL injection pattern check
        entry: |
          grep -rn 'fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE' backend/store/ && exit 1 || exit 0
        language: system
        types: [go]
```

### 3.2 CI Pipeline — PR Gate

**File**: `.github/workflows/security-scan.yml` (new)

```yaml
name: Security Scan
on:
  pull_request:
    branches: [main, release/*]
  schedule:
    - cron: '0 6 * * *'  # Daily 6am UTC

jobs:
  go-vulnerability:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - name: govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck -json ./backend/... > vuln-report.json
      - name: Check critical vulnerabilities
        run: |
          CRITICAL=$(jq '[.[] | select(.vulnerability.severity == "CRITICAL")] | length' vuln-report.json)
          if [ "$CRITICAL" -gt 0 ]; then
            echo "::error::$CRITICAL critical vulnerabilities found"
            exit 1
          fi

  npm-audit:
    runs-on: ubuntu-latest
    defaults:
      run: { working-directory: frontend }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - run: npm ci
      - run: npm audit --audit-level=high --json > npm-audit-report.json || true
      - name: Check high/critical
        run: |
          HIGH=$(jq '.metadata.vulnerabilities.high + .metadata.vulnerabilities.critical' npm-audit-report.json)
          if [ "$HIGH" -gt 0 ]; then
            echo "::error::$HIGH high/critical npm vulnerabilities found"
            exit 1
          fi

  container-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build image
        run: docker build -t bytebase:scan .
      - name: Trivy scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: 'bytebase:scan'
          format: 'json'
          output: 'trivy-report.json'
          severity: 'CRITICAL,HIGH'
          exit-code: '1'

  secret-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: gitleaks/gitleaks-action@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 3.3 SBOM Generation (Release Pipeline)

**File**: `scripts/generate-sbom.sh` (new)

```bash
#!/bin/bash
set -euo pipefail

VERSION=${1:-"dev"}
OUTPUT_DIR="dist/sbom"
mkdir -p "$OUTPUT_DIR"

# Go SBOM
syft dir:./backend -o cyclonedx-json > "$OUTPUT_DIR/go-sbom-${VERSION}.json"

# npm SBOM
syft dir:./frontend -o cyclonedx-json > "$OUTPUT_DIR/npm-sbom-${VERSION}.json"

# Container SBOM
syft bytebase:${VERSION} -o cyclonedx-json > "$OUTPUT_DIR/container-sbom-${VERSION}.json"

# Sign SBOM
cosign sign-blob --key cosign.key "$OUTPUT_DIR/go-sbom-${VERSION}.json" \
  --output-signature "$OUTPUT_DIR/go-sbom-${VERSION}.json.sig"

echo "SBOM generated and signed in $OUTPUT_DIR"
```

### 3.4 Makefile Targets

**File**: `Makefile` (extend existing)

```makefile
.PHONY: security-scan security-audit sbom

security-scan:
	govulncheck ./backend/...
	cd frontend && npm audit --audit-level=high
	gosec -fmt=text ./backend/...

security-audit: security-scan
	gitleaks detect --source . --report-format json --report-path gitleaks-report.json
	trivy image bytebase:latest --severity CRITICAL,HIGH

sbom:
	./scripts/generate-sbom.sh $(VERSION)
```

---

## 4. Vulnerability Suppression

**File**: `.govulncheck-ignore.yaml` (new)

```yaml
# Vulnerability suppressions with justification
suppressions:
  - id: GO-2024-XXXX
    reason: "Not exploitable in our usage context (CVE applies to server mode only)"
    reviewer: "security-team"
    expiry: "2026-12-31"
```

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-SEC-010 (SIEM) | Vulnerability alerts forwarded as security events |
| CR-SEC-016 (SQL Injection) | gosec rules for SQL patterns |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Go + npm vulnerability scanning in CI | Sprint 1 |
| 2 | Container image scanning + secret detection | Sprint 1 |
| 3 | Pre-commit hooks setup | Sprint 2 |
| 4 | SBOM generation + signing | Sprint 3 |
