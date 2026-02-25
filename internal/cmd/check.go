package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

func newCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run divergence check",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			full, _ := cmd.Flags().GetBool("full")
			quiet, _ := cmd.Flags().GetBool("quiet")
			jsonOut, _ := cmd.Flags().GetBool("json")
			lang, _ := cmd.Flags().GetString("lang")

			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			divRoot := filepath.Join(repoRoot, ".gate")

			if err := amadeus.InitGateDir(divRoot); err != nil {
				return fmt.Errorf("init .gate: %w", err)
			}

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := amadeus.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if lang != "" {
				if !amadeus.ValidLang(lang) {
					return fmt.Errorf("unsupported language: %s (supported: ja, en)", lang)
				}
				cfg.Lang = lang
			}

			logger := amadeus.NewLogger(cmd.ErrOrStderr(), verbose)

			store := amadeus.NewProjectionStore(divRoot)
			eventStore := &amadeus.FileEventStore{Dir: filepath.Join(divRoot, "events")}

			a := &amadeus.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    eventStore,
				Projector: &amadeus.Projector{Store: store},
				Git:       amadeus.NewGitClient(repoRoot),
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
			}

			return a.RunCheck(cmd.Context(), amadeus.CheckOptions{
				Full:   full,
				DryRun: dryRun,
				Quiet:  quiet,
				JSON:   jsonOut,
			})
		},
	}

	cmd.Flags().BoolP("dry-run", "n", false, "generate prompt only")
	cmd.Flags().BoolP("full", "f", false, "force full calibration check")
	cmd.Flags().BoolP("quiet", "q", false, "summary-only output")
	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}
