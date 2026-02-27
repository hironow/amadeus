package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove state directory (.gate/)",
		Long:  "Delete the .gate/ directory to reset to a clean state. Use 'amadeus init' to reinitialize.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			stateDir := filepath.Join(repoRoot, ".gate")

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
