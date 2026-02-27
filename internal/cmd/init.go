package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/session"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize .gate directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, ".gate")
			if _, err := os.Stat(divRoot); err == nil {
				return fmt.Errorf("%s already exists", divRoot)
			}
			if err := session.InitGateDir(divRoot); err != nil {
				return fmt.Errorf("init .gate: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  Initialized %s\n", divRoot)
			return nil
		},
	}
}
