package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate config file",
		Long: `Validate the amadeus configuration file.

Reads .gate/config.yaml (or the path specified by --config) and checks
all fields against the configuration schema. Reports individual
validation errors with [FAIL] markers. If [path] is omitted, the
current working directory is used.`,
		Example: `  # Validate config in current directory
  amadeus validate

  # Validate a specific project
  amadeus validate /path/to/project

  # Validate a specific config file
  amadeus validate --config /path/to/config.yaml`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")

			if configPath == "" {
				repoRoot, err := resolveTargetDir(args)
				if err != nil {
					return err
				}
				configPath = filepath.Join(repoRoot, domain.StateDir, "config.yaml")
			}
			if _, err := os.Stat(configPath); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("config not found: %s", configPath)
				}
				return fmt.Errorf("stat config: %w", err)
			}
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			errs := domain.ValidateConfig(cfg)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(cmd.ErrOrStderr(), "  [FAIL] %s\n", e)
				}
				return fmt.Errorf("%d validation error(s)", len(errs))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  [OK] %s is valid\n", configPath)
			return nil
		},
	}
}
