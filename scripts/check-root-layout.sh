#!/usr/bin/env bash
# check-root-layout.sh — verify root directory conforms to layout contract
# Ref: refs/opsx/root-layout-contract.md
set -euo pipefail

errors=0
tool=$(basename "$(pwd)")

# --- Mandatory items ---
for item in .dockerignore .github .gitignore .goreleaser.yaml \
    .markdownlint-cli2.yaml .markdownlint.json \
    .pre-commit-config.yaml .semgrep .semgrepignore \
    LICENSE README.md cmd doc.go docker docs go.mod go.sum \
    internal justfile mise.toml scripts tests; do
    if [[ ! -e "$item" ]]; then
        echo "ERROR: mandatory item missing: $item" >&2
        errors=$((errors + 1))
    fi
done

# --- Prohibited items ---
# Root-level binaries (tool name without extension)
if [[ -f "$tool" ]]; then
    echo "ERROR: root binary found: $tool (use dist/ instead)" >&2
    errors=$((errors + 1))
fi

# .DS_Store
if [[ -f .DS_Store ]]; then
    echo "ERROR: .DS_Store found in root" >&2
    errors=$((errors + 1))
fi

# firebase-debug.log
if [[ -f firebase-debug.log ]]; then
    echo "ERROR: firebase-debug.log found in root" >&2
    errors=$((errors + 1))
fi

# --- dist/ must be gitignored ---
if ! grep -q '/dist/' .gitignore 2>/dev/null; then
    echo "ERROR: /dist/ not found in .gitignore" >&2
    errors=$((errors + 1))
fi

# --- Result ---
if [[ $errors -gt 0 ]]; then
    echo "root-layout check FAILED ($errors errors)" >&2
    exit 1
fi
echo "root-layout check passed"
