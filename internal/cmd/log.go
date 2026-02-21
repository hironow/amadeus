package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

func newLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show divergence log",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			jsonOut, _ := cmd.Flags().GetBool("json")

			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, ".gate")

			if _, err := os.Stat(divRoot); errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf(".gate/ not found. Run 'amadeus init' first")
			}

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := amadeus.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger := amadeus.NewLogger(cmd.ErrOrStderr(), verbose)
			a := &amadeus.Amadeus{
				Config:  cfg,
				Store:   amadeus.NewStateStore(divRoot),
				Logger:  logger,
				DataOut: cmd.OutOrStdout(),
			}
			if jsonOut {
				return a.PrintLogJSON()
			}
			return a.PrintLog()
		},
	}

	cmd.Flags().Bool("json", false, "output as JSON")

	return cmd
}
