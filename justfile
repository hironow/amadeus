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

# Run semgrep rules (ERROR only — blocks CI)
semgrep:
    semgrep scan --config .semgrep/ --error --severity ERROR .

# Run semgrep WARNING rules (informational — does not block CI)
semgrep-warnings:
    semgrep scan --config .semgrep/ --severity WARNING .

# Test semgrep rules against known-bad fixtures
semgrep-test:
    semgrep scan --test --config .semgrep/cobra.yaml .semgrep/cobra.go
    semgrep scan --test --config .semgrep/shared-adr.yaml .semgrep/shared-adr.go
    semgrep scan --test --config .semgrep/layers.yaml .semgrep/layers.go
    semgrep scan --test --config .semgrep/stdio.yaml .semgrep/stdio.go

# Audit test package convention (external vs same-package test functions)
test-package-audit:
    #!/usr/bin/env bash
    audit_dir() {
        local dir="$1" label="$2"
        ext=$(grep -rl '^package .*_test$' "$dir" --include='*_test.go' 2>/dev/null | xargs grep -c '^func Test' 2>/dev/null | awk -F: '{s+=$2}END{print s+0}')
        same=$(grep -rL '^package .*_test$' "$dir" --include='*_test.go' 2>/dev/null | xargs grep -c '^func Test' 2>/dev/null | awk -F: '{s+=$2}END{print s+0}')
        total=$((ext + same))
        if [ $total -gt 0 ]; then pct=$((ext * 100 / total)); else pct=0; fi
        echo "$label: external $ext (${pct}%) / same $same ($((100 - pct))%)"
    }
    audit_dir "internal" "internal/"
    audit_dir "internal/session" "internal/session/"

# Verify root package contains only doc.go (no code at root)
root-guard:
    @if ls *.go 2>/dev/null | grep -qv '^doc\.go$'; then \
        echo "ERROR: root package may only contain doc.go. Found:" >&2; \
        ls *.go | grep -v '^doc\.go$' >&2; \
        exit 1; \
    fi

# Lint (fmt check + vet + markdown lint)
lint: vet semgrep root-guard nosemgrep-audit lint-md
    @gofmt -l . | grep . && echo "gofmt: files need formatting" && exit 1 || true

# Format, vet, test — full check before commit
check: fmt vet semgrep root-guard nosemgrep-audit test docs-check

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

# Run live W&B Weave trace delivery test (requires WANDB_API_KEY)
test-weave-live:
    go test ./tests/integration/ -run TestWeave_LiveTraceDelivery -count=1 -v -timeout=60s

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
    @GO_VER=$(mise exec -- go version | awk '{print $3}'); \
    COMPILE_VER=$(mise exec -- go tool compile -V 2>&1 | awk '{print $3}') || true; \
    if [ "$GO_VER" != "$COMPILE_VER" ]; then \
        echo "ERROR: go binary ($GO_VER) != go tool compile ($COMPILE_VER)"; \
        echo "  go version:       $(mise exec -- go version)"; \
        echo "  go tool compile:  $(mise exec -- go tool compile -V 2>&1)"; \
        echo "  GOROOT:           $(mise exec -- go env GOROOT)"; \
        echo "  GOTOOLDIR:        $(mise exec -- go env GOTOOLDIR)"; \
        echo ""; \
        echo "Fix: mise install go && mise reshim"; \
        exit 1; \
    fi

# Run L1 scenario test
test-scenario-min: check-go
    mise exec -- go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s

# Run L2 scenario test
test-scenario-small: check-go
    mise exec -- go test -tags scenario ./tests/scenario/ -run TestScenario_L2 -count=1 -v -timeout=180s

# Run L3 scenario test
test-scenario-middle: check-go
    mise exec -- go test -tags scenario ./tests/scenario/ -run TestScenario_L3 -count=1 -v -timeout=300s

# Run L4 scenario test
test-scenario-hard: check-go
    mise exec -- go test -tags scenario ./tests/scenario/ -run TestScenario_L4 -count=1 -v -timeout=600s

# Run L1+L2 scenario tests (CI default)
test-scenario: check-go
    mise exec -- go test -tags scenario ./tests/scenario/ -run "TestScenario_L[12]" -count=1 -v -timeout=300s

# Run all scenario tests
test-scenario-all: check-go
    mise exec -- go test -tags scenario ./tests/scenario/ -count=1 -v -timeout=900s

# Audit nosemgrep annotations: require [permanent] or [expires: YYYY-MM-DD] tag
nosemgrep-audit:
    #!/usr/bin/env bash
    set -euo pipefail
    rc=0
    today=$(date +%Y-%m-%d)
    while IFS= read -r line; do
        file=$(echo "$line" | cut -d: -f1)
        lineno=$(echo "$line" | cut -d: -f2)
        if echo "$line" | grep -q '\[permanent\]'; then
            continue
        fi
        if echo "$line" | grep -qoE '\[expires: [0-9]{4}-[0-9]{2}-[0-9]{2}\]'; then
            expiry=$(echo "$line" | grep -oE '\[expires: [0-9]{4}-[0-9]{2}-[0-9]{2}\]' | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}')
            if [[ "$expiry" < "$today" || "$expiry" == "$today" ]]; then
                echo "EXPIRED: $file:$lineno (expired $expiry)" >&2
                rc=1
            fi
        else
            echo "MISSING TAG: $file:$lineno — add [permanent] or [expires: YYYY-MM-DD]" >&2
            rc=1
        fi
    done < <(grep -rn "nosemgrep:" --include="*.go" . || true)
    if [ $rc -eq 0 ]; then echo "nosemgrep-audit: all annotations tagged"; fi
    exit $rc

# Check docs for stale references (e.g. deprecated internal/port path)
docs-check:
    @echo "Checking for stale references..."
    @! grep -rn 'internal/port[^/]' docs/ internal/domain/doc.go 2>/dev/null || (echo "ERROR: stale internal/port references found" && exit 1)
    @! grep -n 'usecase は session' .semgrep/layers.yaml 2>/dev/null || (echo "ERROR: stale usecase->session allowance in semgrep" && exit 1)
    @! grep -rin 'eventsource.*廃止\|eventsource.*吸収\|eventsource.*session.*移' docs/ 2>/dev/null || (echo "ERROR: stale eventsource terminology — eventsource is retained per todo 73" && exit 1)
    @bash scripts/check_adr_refs.sh
    @echo "docs-check passed"

# Audit white-box-reason comments on same-package test files
test-package-rationale-audit:
    #!/usr/bin/env bash
    missing=0
    while IFS= read -r f; do
      if ! grep -q 'white-box-reason:' "$f" 2>/dev/null; then
        echo "MISSING white-box-reason: $f" >&2
        missing=$((missing + 1))
      fi
    done < <(grep -rL '^package .*_test$' internal/ --include='*_test.go' 2>/dev/null)
    if [ "$missing" -gt 0 ]; then
      echo "FAIL: $missing same-package test file(s) missing white-box-reason comment" >&2
      exit 1
    fi
    echo "OK: all same-package test files have white-box-reason comments"

# Clean build artifacts
clean:
    rm -f {{TOOL}} coverage.out
    rm -rf dist/
    go clean
