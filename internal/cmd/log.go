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
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger := amadeus.NewLogger(cmd.ErrOrStderr(), verbose)
			store := amadeus.NewProjectionStore(divRoot)
			a := &amadeus.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    &amadeus.FileEventStore{Dir: filepath.Join(divRoot, "events")},
				Projector: &amadeus.Projector{Store: store},
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
			}
			if jsonOut {
				return a.PrintLogJSON()
			}
			return a.PrintLog()
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}
