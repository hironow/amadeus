//go:build e2e

package e2e

import "testing"

// TestE2E_Pipeline_GateNotFound tests data-plane commands that require .gate/.
func TestE2E_Pipeline_GateNotFound(t *testing.T) {
	dir := t.TempDir()

	// These commands should fail without .gate/
	for _, cmd := range [][]string{
		{"sync"},
		{"log"},
		{"mark-commented", "x", "y"},
	} {
		_, _, err := runCmd(t, dir, cmd...)
		if err == nil {
			t.Errorf("expected error for %v without .gate/", cmd)
		}
	}
}
