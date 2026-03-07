#!/usr/bin/env bash
set -euo pipefail

# ADR reference integrity check
# Verifies that all linked ADR files in docs/adr/README.md exist on disk.

errors=0

# Check tool-specific ADR links in README.md
while IFS= read -r line; do
    # Extract markdown link targets like (0006-foo.md)
    file=$(echo "$line" | sed -n 's/.*](\([^)]*\.md\)).*/\1/p')
    if [[ -n "$file" ]]; then
        if [[ ! -f "docs/adr/$file" ]]; then
            echo "ERROR: docs/adr/README.md links to non-existent file: docs/adr/$file"
            errors=$((errors + 1))
        fi
    fi
done < docs/adr/README.md

if [[ $errors -gt 0 ]]; then
    echo "ADR reference check FAILED ($errors errors)"
    exit 1
fi

echo "ADR reference check passed"
