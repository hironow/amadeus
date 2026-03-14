package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean [path]",
		Short: "Remove state directory (.gate/)",
		Long:  "Delete the .gate/ directory to reset to a clean state. Use 'amadeus init' to reinitialize.",
		Example: `  # Clean current directory (interactive confirmation)
  amadeus clean

  # Clean a specific project directory
  amadeus clean /path/to/project

  # Skip confirmation prompt
  amadeus clean --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			stateDir := filepath.Join(repoRoot, domain.StateDir)

			info, statErr := os.Stat(stateDir)
			if statErr != nil || !info.IsDir() {
				fmt.Fprintf(cmd.ErrOrStderr(), "Nothing to clean at %s\n", repoRoot)
				return nil
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "The following will be deleted:\n  %s/\n\nDelete? [y/N]: ", stateDir)
				var answer string
				fmt.Fscanln(cmd.InOrStdin(), &answer)
				if answer != "y" && answer != "Y" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Aborted.\n")
					return nil
				}
			}

			if err := os.RemoveAll(stateDir); err != nil {
				return fmt.Errorf("remove %s: %w", stateDir, err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Cleaned %s\n", stateDir)
			return nil
		},
	}
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	return cmd
}
