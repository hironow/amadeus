# amadeus — task runner
# https://just.systems

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

# Tool name
TOOL := "amadeus"

# External commands
MARKDOWNLINT := "bunx markdownlint-cli2"

# Version from git tags
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
DATE := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

# Default: show help
default: help

# Help: list available recipes
help:
    @just --list --unsorted

# Install prek hooks (pre-commit + pre-push) with quiet mode
prek-install:
    prek install -t pre-commit -t pre-push --overwrite
    @sed 's/-- "\$@"/--quiet -- "\$@"/' .git/hooks/pre-commit > .git/hooks/pre-commit.tmp && mv .git/hooks/pre-commit.tmp .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
    @sed 's/-- "\$@"/--quiet -- "\$@"/' .git/hooks/pre-push > .git/hooks/pre-push.tmp && mv .git/hooks/pre-push.tmp .git/hooks/pre-push && chmod +x .git/hooks/pre-push
    @echo "prek hooks installed (quiet mode)"

# Run all prek hooks on all files
prek-run:
    prek run --all-files

# Lint markdown files
lint-md:
    @{{MARKDOWNLINT}} --fix "*.md" "docs/**/*.md"

# Build the binary with version info
build:
    go build -ldflags "-X github.com/hironow/{{TOOL}}/internal/cmd.Version={{VERSION}} -X github.com/hironow/{{TOOL}}/internal/cmd.Commit={{COMMIT}} -X github.com/hironow/{{TOOL}}/internal/cmd.Date={{DATE}}" -o {{TOOL}} ./cmd/{{TOOL}}/

# Build and install to /usr/local/bin
install: build
    mv {{TOOL}} /usr/local/bin/

# Run all tests
test:
    go test ./... -count=1 -timeout=300s

# Run tests with verbose output
test-v:
    go test ./... -count=1 -timeout=300s -v

# Run tests with race detector
test-race:
    go test ./... -race -count=1 -timeout=300s

# Run tests with coverage report
cover:
    go test ./... -coverprofile=coverage.out -count=1 -timeout=300s
    go tool cover -func=coverage.out

# Open coverage in browser
cover-html: cover
    go tool cover -html=coverage.out

# Format code
fmt:
    gofmt -w .

# Run go vet
vet:
    go vet ./...

# Run semgrep rules
semgrep:
    semgrep scan --config .semgrep/ --error --severity ERROR .

# Lint (fmt check + vet + markdown lint)
lint: vet lint-md
    @gofmt -l . | grep . && echo "gofmt: files need formatting" && exit 1 || true

# Format, vet, test — full check before commit
check: fmt vet test

# Start Jaeger v2 (OTel trace viewer + MCP) on http://localhost:16686
jaeger:
    docker compose -f docker/compose.yaml up -d
    @echo "Jaeger UI:      http://localhost:16686"
    @echo "OTLP endpoint:  http://localhost:4318"
    @echo "MCP endpoint:   http://localhost:16687/mcp"
    @echo ""
    @echo "Run {{TOOL}} with tracing:"
    @echo "  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 {{TOOL}} check"

# Stop Jaeger
jaeger-down:
    docker compose -f docker/compose.yaml down

# Generate CLI documentation in Markdown
docgen:
    go run ./internal/tools/docgen/

# Validate goreleaser config
release-check:
    goreleaser check

# Snapshot GoReleaser build (no publish)
release-snapshot:
    goreleaser release --snapshot --clean

# Build and run E2E tests in Docker
test-e2e:
    docker compose -f tests/e2e/compose-e2e.yaml build
    docker compose -f tests/e2e/compose-e2e.yaml run --rm e2e go test -tags e2e ./tests/e2e/ -count=1 -v -timeout=600s

# Interactive shell inside E2E container
test-e2e-shell:
    docker compose -f tests/e2e/compose-e2e.yaml build
    docker compose -f tests/e2e/compose-e2e.yaml run --rm -it e2e /bin/sh

# Clean E2E containers and volumes
test-e2e-down:
    docker compose -f tests/e2e/compose-e2e.yaml down --rmi local --volumes

# Verify Go toolchain consistency (GOROOT vs go binary)
[private]
check-go:
    @GO_VER=$(go version | awk '{print $3}'); \
    TOOL_VER=$(go env GOVERSION); \
    if [ "$GO_VER" != "$TOOL_VER" ]; then \
        echo "ERROR: go binary ($GO_VER) != GOVERSION ($TOOL_VER)"; \
        echo "Fix: unset GOROOT GOTOOLDIR, or run 'mise install go'"; \
        exit 1; \
    fi

# Run L1 scenario test
test-scenario-min: check-go
    go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s

# Run L2 scenario test
test-scenario-small: check-go
    go test -tags scenario ./tests/scenario/ -run TestScenario_L2 -count=1 -v -timeout=180s

# Run L3 scenario test
test-scenario-middle: check-go
    go test -tags scenario ./tests/scenario/ -run TestScenario_L3 -count=1 -v -timeout=300s

# Run L4 scenario test
test-scenario-hard: check-go
    go test -tags scenario ./tests/scenario/ -run TestScenario_L4 -count=1 -v -timeout=600s

# Run L1+L2 scenario tests (CI default)
test-scenario: check-go
    go test -tags scenario ./tests/scenario/ -run "TestScenario_L[12]" -count=1 -v -timeout=300s

# Run all scenario tests
test-scenario-all: check-go
    go test -tags scenario ./tests/scenario/ -count=1 -v -timeout=900s

# Clean build artifacts
clean:
    rm -f {{TOOL}} coverage.out
    rm -rf dist/
    go clean
