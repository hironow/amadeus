//go:build e2e

package e2e

import (
	"context"
	"testing"
)

// TestE2E_Pipeline_GateNotFound tests data-plane commands that require .gate/.
func TestE2E_Pipeline_GateNotFound(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_gate_not_found"
	
	// Create raw directory without init (so no .gate/ exists)
	execInContainer(t, ctx, c, []string{"mkdir", "-p", dir})

	// These commands should fail without .gate/
	for _, cmd := range [][]string{
		{"sync"},
		{"log"},
		{"mark-commented", "x", "y"},
	} {
		_, _, err := runCmd(t, ctx, c, dir, cmd...)
		if err == nil {
			t.Errorf("expected error for %v without .gate/", cmd)
		}
	}
}
