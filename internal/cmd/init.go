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
		Use:   "init",
		Short: "Initialize .gate directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			divRoot := filepath.Join(repoRoot, ".gate")
			if err := session.InitGateDir(divRoot); err != nil {
				return fmt.Errorf("init .gate: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  Initialized %s\n", divRoot)
			return nil
		},
	}
}
