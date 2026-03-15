package testdata

import "path/filepath"

// ==========================================================================
// layers-local.yaml test fixture
// Covers amadeus-specific rules:
//   - no-state-dir-literal-in-path-join
// ==========================================================================

// --- Rule: no-state-dir-literal-in-path-join ---

func badStateDirLiteral() {
	// ruleid: no-state-dir-literal-in-path-join
	filepath.Join("/home", ".gate")
}

func badStateDirLiteralWithSuffix() {
	// ruleid: no-state-dir-literal-in-path-join
	filepath.Join("/home", ".gate", "events")
}

func goodStateDirConst(stateDir string) {
	// ok: no-state-dir-literal-in-path-join
	filepath.Join("/home", stateDir)
}

var _ = filepath.Join
