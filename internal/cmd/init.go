package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize .gate directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, domain.StateDir)
			if _, err := os.Stat(divRoot); err == nil {
				return fmt.Errorf("%s already exists", divRoot)
			}
			initCmd := domain.InitCommand{RepoRoot: repoRoot}
			if err := usecase.RunInit(initCmd, &session.InitAdapter{}); err != nil {
				return fmt.Errorf("init: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  Initialized %s\n", divRoot)

			otelBackend, _ := cmd.Flags().GetString("otel-backend")
			if otelBackend != "" {
				otelEntity, _ := cmd.Flags().GetString("otel-entity")
				otelProject, _ := cmd.Flags().GetString("otel-project")
				content, otelErr := platform.OtelEnvContent(otelBackend, otelEntity, otelProject)
				if otelErr != nil {
					return otelErr
				}
				otelPath := filepath.Join(divRoot, ".otel.env")
				if err := os.WriteFile(otelPath, []byte(content), 0o644); err != nil {
					return fmt.Errorf("write .otel.env: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "OTel backend configured: %s → %s\n", otelBackend, otelPath)
			}

			return nil
		},
	}
	cmd.Flags().String("otel-backend", "", "OTel backend: jaeger, weave")
	cmd.Flags().String("otel-entity", "", "Weave entity/team (required for weave)")
	cmd.Flags().String("otel-project", "", "Weave project (required for weave)")
	return cmd
}
