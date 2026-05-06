package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFsnotifyReplacedByHironowFork asserts that the fsnotify dependency
// is redirected to the hironow fork via a replace directive in go.mod,
// and that go.sum records only the fork's hashes (no upstream entries).
//
// This pins the dependency to a specific commit and isolates the build
// from the upstream module path entirely. The literal "fsnotify/fsnotify"
// is split with `+` so this file does not match its own scan.
func TestFsnotifyReplacedByHironowFork(t *testing.T) {
	upstream := "github.com/fsnotify" + "/fsnotify"
	fork := "github.com/hironow/fsnotify"

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	modBytes, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !hasReplaceDirective(string(modBytes), upstream, fork) {
		t.Fatalf("expected replace directive %s => %s in go.mod, got:\n%s",
			upstream, fork, string(modBytes))
	}

	sumBytes, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("read go.sum: %v", err)
	}
	sum := string(sumBytes)
	if strings.Contains(sum, upstream+" ") {
		t.Fatalf("upstream module hash present in go.sum (replace not effective)")
	}
	if !strings.Contains(sum, fork+" ") {
		t.Fatalf("fork module hash missing from go.sum")
	}
}

// hasReplaceDirective scans go.mod content for a replace line that maps
// the upstream module to the fork. Handles both single-line `replace ... => ...`
// and block `replace ( ... )` forms.
func hasReplaceDirective(modContent, upstream, fork string) bool {
	inBlock := false
	for _, raw := range strings.Split(modContent, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "replace (") {
			inBlock = true
			continue
		}
		if inBlock {
			if line == ")" {
				inBlock = false
				continue
			}
			if strings.Contains(line, upstream) && strings.Contains(line, fork) {
				return true
			}
			continue
		}
		if strings.HasPrefix(line, "replace ") &&
			strings.Contains(line, upstream) && strings.Contains(line, fork) {
			return true
		}
	}
	return false
}
