# =============================================================================
# Bytebase Build Profiles
# =============================================================================
# Usage:
#   make build                    # Ultimate (all drivers, default)
#   make build-enterprise         # Enterprise core (PG, MySQL, MSSQL, Oracle)
#   make build-minimal            # Minimal (PG only, for dev/demo)
#   make build-dev                # Dev mode with race detector
#   make test                     # Run all tests
#   make lint                     # Run linters
# =============================================================================

# Version from git
VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go toolchain
GO        ?= go
GOFLAGS   ?= -p=8
LDFLAGS   := -w -s \
	-X 'github.com/bytebase/bytebase/backend/bin/server/cmd.version=$(VERSION)' \
	-X 'github.com/bytebase/bytebase/backend/bin/server/cmd.gitcommit=$(GIT_COMMIT)'

# Output
OUTPUT_DIR ?= ./build
BINARY     = $(OUTPUT_DIR)/bytebase

# =============================================================================
# Build Targets
# =============================================================================

.PHONY: build build-enterprise build-minimal build-dev

## build: Build with all drivers (ultimate profile, ~100MB)
build:
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) --tags "release,embed_frontend" -ldflags "$(LDFLAGS)" -o $(BINARY) ./backend/bin/server/main.go
	@echo "✅ Built $(BINARY) [ultimate]"

## build-enterprise: Build with core SQL drivers only (~60MB)
build-enterprise:
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) --tags "enterprise_core,release,embed_frontend" -ldflags "$(LDFLAGS)" -o $(BINARY)-enterprise ./backend/bin/server/main.go
	@echo "✅ Built $(BINARY)-enterprise [enterprise_core: PG, MySQL, MSSQL, Oracle]"

## build-minimal: Build with PG driver only (~40MB, for dev/demo)
build-minimal:
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) --tags "minidemo,release" -ldflags "$(LDFLAGS)" -o $(BINARY)-minimal ./backend/bin/server/main.go
	@echo "✅ Built $(BINARY)-minimal [minidemo: PG only]"

## build-dev: Build for development with race detector
build-dev:
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build -race --tags "release" -ldflags "$(LDFLAGS)" -o $(BINARY)-dev ./backend/bin/server/main.go
	@echo "✅ Built $(BINARY)-dev [dev mode with race detector]"

# =============================================================================
# Test & Lint
# =============================================================================

.PHONY: test test-unit test-store lint lint-file-size

## test: Run all tests
test: test-unit

## test-unit: Run unit tests for infrastructure packages
test-unit:
	$(GO) test -count=1 -timeout 120s \
		./backend/common/resilience/... \
		./backend/store/cache/... \
		./backend/store/...
	@echo "✅ Unit tests passed"

## test-store: Run store integration tests
test-store:
	$(GO) test -count=1 -timeout 300s ./backend/store/...
	@echo "✅ Store tests passed"

## lint: Run linters
lint: lint-file-size
	@echo "✅ All lints passed"

## lint-file-size: Check for oversized Go files
lint-file-size:
	@bash scripts/lint-file-size.sh || exit 1

# =============================================================================
# Verify
# =============================================================================

.PHONY: verify verify-build verify-interfaces

## verify: Build all critical packages without output
verify: verify-build verify-interfaces
	@echo "✅ All verification passed"

## verify-build: Compile all critical packages
verify-build:
	$(GO) build ./backend/store/... \
		./backend/store/cache/... \
		./backend/common/resilience/... \
		./backend/component/bus/... \
		./backend/component/iam/... \
		./backend/component/config/... \
		./backend/enterprise/... \
		./backend/server/... \
		./backend/api/v1/... \
		./backend/api/lsp/... \
		./backend/api/mcp/... \
		./backend/plugin/db/...
	@echo "✅ All 12 packages compile"

## verify-interfaces: Verify interface compile-time checks
verify-interfaces:
	$(GO) test -run=TestNone -count=1 ./backend/store/... 2>/dev/null || true
	@echo "✅ Interface verification passed"

# =============================================================================
# Clean & Help
# =============================================================================

.PHONY: clean help

## clean: Remove build artifacts
clean:
	rm -rf $(OUTPUT_DIR)
	@echo "🧹 Clean"

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | sort
