package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// TestMultiplexCWDIsolation_SQLiteWAL locks the existing CWD-relative state
// design under an explicit regression test (Issue 0014 軸 3 short-term).
//
// If a future PR moves any SQLite path to $HOME/.cache/<tool> or
// /tmp/<tool>-state.db (PID-singleton anti-pattern), this test fails
// immediately.
//
// Sameform principle: this test body is byte-identical across the 5 tools
// (phonewave / sightjack / paintress / amadeus / dominator) modulo the
// `package` declaration and the `domain` import path, because every tool
// exposes the canonical state directory as `domain.StateDir`.
func TestMultiplexCWDIsolation_SQLiteWAL(t *testing.T) {
	// given: two distinct temp dirs simulating two project roots
	projA := t.TempDir()
	projB := t.TempDir()

	// when: derive the canonical state dir for each project root using the
	// tool's StateDir constant (NOT a hard-coded literal — keeps the test
	// portable across the 5-tool fleet).
	expectStateDirA := filepath.Join(projA, domain.StateDir)
	expectStateDirB := filepath.Join(projB, domain.StateDir)

	// create the state dirs and a .run/ subdirectory each, mirroring the
	// runtime layout that hosts the SQLite WAL state DB.
	if err := os.MkdirAll(filepath.Join(expectStateDirA, ".run"), 0o755); err != nil {
		t.Fatalf("mkdir A: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(expectStateDirB, ".run"), 0o755); err != nil {
		t.Fatalf("mkdir B: %v", err)
	}

	// then: the two state dirs MUST be distinct — a single shared global
	// path would mean two CWDs collapse onto the same SQLite file.
	if expectStateDirA == expectStateDirB {
		t.Fatalf("expected distinct state dirs, got same: %s", expectStateDirA)
	}

	// then: each state dir MUST be rooted inside its project root. If a
	// future PR changes domain.StateDir to a global absolute path (e.g.
	// "/tmp/<tool>-state.db" or "$HOME/.cache/<tool>"), filepath.Join
	// would still produce a path under projA/projB only when StateDir
	// stays relative — this is the CWD-isolation contract.
	if !strings.HasPrefix(expectStateDirA, projA+string(filepath.Separator)) {
		t.Errorf("StateDir A escaped project root: state=%s root=%s", expectStateDirA, projA)
	}
	if !strings.HasPrefix(expectStateDirB, projB+string(filepath.Separator)) {
		t.Errorf("StateDir B escaped project root: state=%s root=%s", expectStateDirB, projB)
	}

	// then: regression contract — the StateDir constant MUST stay
	// CWD-relative. It must start with "." (hidden dir convention) and
	// must NOT be an absolute path. If a future commit changes the
	// constant to an absolute path (e.g. "/tmp/<tool>-state.db" or
	// "$HOME/.cache/<tool>"), these assertions fail. This is the direct
	// guard against the PID-singleton anti-pattern.
	if !strings.HasPrefix(domain.StateDir, ".") {
		t.Errorf("StateDir must be CWD-relative (start with '.'), got: %s", domain.StateDir)
	}
	if filepath.IsAbs(domain.StateDir) {
		t.Errorf("StateDir must NOT be absolute, got: %s", domain.StateDir)
	}

	// then: the StateDir constant must not contain global location markers
	// that would indicate escape from the project root contract.
	if strings.Contains(domain.StateDir, "/tmp/") {
		t.Errorf("StateDir must not reference /tmp/: %s", domain.StateDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	if home != "" && strings.Contains(domain.StateDir, home) {
		t.Errorf("StateDir must not reference $HOME: state=%s home=%s", domain.StateDir, home)
	}
}
