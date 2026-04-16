package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/spf13/cobra"
)

const sessionsToolName = "amadeus"

// resolveSessionsDir resolves the repository root and state directory for sessions commands.
// Resolution order: --path flag -> --config flag -> cwd.
func resolveSessionsDir(cmd *cobra.Command) (repoRoot, stateDirPath string, err error) {
	if cmd.Flags().Changed("path") {
		p := mustString(cmd, "path")
		repoRoot, err = filepath.Abs(p)
		if err != nil {
			return "", "", fmt.Errorf("resolve path: %w", err)
		}
	} else if f := cmd.Flags().Lookup("config"); f != nil && cmd.Flags().Changed("config") {
		configAbs, absErr := filepath.Abs(f.Value.String())
		if absErr != nil {
			return "", "", fmt.Errorf("resolve config path: %w", absErr)
		}
		repoRoot = filepath.Dir(filepath.Dir(configAbs))
	} else {
		repoRoot, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("resolve cwd: %w", err)
		}
	}
	stateDirPath = filepath.Join(repoRoot, domain.StateDir)
	if info, statErr := os.Stat(stateDirPath); statErr != nil || !info.IsDir() {
		return "", "", fmt.Errorf("state directory not found: %s (run '%s init' first)", stateDirPath, sessionsToolName)
	}
	return repoRoot, stateDirPath, nil
}
