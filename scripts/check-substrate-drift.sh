#!/usr/bin/env bash
# check-substrate-drift.sh — Verify canonical AI coding substrate files
# match expected checksums. Run from repo root.
#
# Checksums are computed after normalizing tool-specific import paths and
# tool name strings. This allows the same checksum to validate all 3 tools.
#
# Exit 0 = all canonical files match. Exit 1 = drift detected.

set -euo pipefail

TOOL_NAME="amadeus"
REPO_ROOT="${1:-.}"

# Canonical file → expected normalized SHA-256
declare -A EXPECTED=(
    ["internal/domain/coding_session.go"]="5774ac98a61c1e5b6c57425134bb5463539160a80448888e5605cc67eb16bdd0"
    ["internal/session/session_tracking_adapter.go"]="39e9a113f575b42089213e8f8bc963d810e467e3b0ba0da3a15f1d411eb6a866"
    ["internal/session/coding_session_store.go"]="599839924143802d391be3298ba0039f1cb2d02bbb16c75e04a3e52cad468027"
    ["internal/platform/stream_normalizer.go"]="76ac7c89eeb019d62dc463a00406273ebf882feb061a7c1c77db7969f272ab98"
    ["internal/harness/policy/run_guard.go"]="b7ad3880247798d94a776ecd43d03a911318c5d0ae176b0ffb05f51e91ec7c81"
    ["docs/shared-adr/S0037-coding-session-abstraction-layer.md"]="2515700a3e8d672863e6d10e2ab89e913eef85a9805a0c84fdd59dfac1de4a58"
)

rc=0
for file in "${!EXPECTED[@]}"; do
    path="${REPO_ROOT}/${file}"
    if [[ ! -f "$path" ]]; then
        echo "DRIFT: ${file} — file missing"
        rc=1
        continue
    fi
    actual=$(sed \
        -e "s|github.com/hironow/${TOOL_NAME}/|github.com/hironow/TOOL/|g" \
        -e "s|\"${TOOL_NAME}\"|\"TOOL\"|g" \
        -e "s|'${TOOL_NAME}'|'TOOL'|g" \
        "$path" | shasum -a 256 | cut -d' ' -f1)
    expected="${EXPECTED[$file]}"
    if [[ "$actual" != "$expected" ]]; then
        echo "DRIFT: ${file} — checksum mismatch"
        echo "  expected: ${expected}"
        echo "  actual:   ${actual}"
        rc=1
    else
        echo "  OK: ${file}"
    fi
done

if [[ $rc -eq 0 ]]; then
    echo "substrate-drift-check: all canonical files match"
fi
exit $rc
