package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log [path]",
		Short: "Show divergence log",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			jsonOut, _ := cmd.Flags().GetBool("json")

			repoRoot, err := resolveTargetDir(args)
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

			logger := loggerFrom(cmd)
			if jsonOut {
				return usecase.RunLogJSON(divRoot, cfg, cmd.OutOrStdout(), logger)
			}
			return usecase.RunLog(divRoot, cfg, cmd.OutOrStdout(), logger)
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}
