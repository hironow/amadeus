package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize .gate directory",
		Long: `Initialize the .gate/ state directory for amadeus divergence tracking.

Creates the directory structure required by amadeus: config.yaml,
events/, .run/, archive/, and insights/. If [path] is omitted, the
current working directory is used. Use --force to reinitialize an
existing .gate/ directory.

Optionally configure an OpenTelemetry backend (Jaeger or Weave) with
the --otel-backend flag. The generated .otel.env file is written into
.gate/ and loaded automatically on subsequent runs.`,
		Example: `  # Initialize in current directory
  amadeus init

  # Initialize a specific project directory
  amadeus init /path/to/project

  # Reinitialize (overwrite existing .gate/)
  amadeus init --force

  # Initialize with Jaeger OTel backend
  amadeus init --otel-backend jaeger`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			force := mustBool(cmd, "force")
			divRoot := filepath.Join(repoRoot, domain.StateDir)
			if _, err := os.Stat(divRoot); err == nil && !force {
				return fmt.Errorf("%s already exists\nUse --force to overwrite", divRoot)
			}
			lang := mustString(cmd, "lang")
			logger := loggerFrom(cmd)
			adapter := &session.InitAdapter{Logger: logger}
			var opts []port.InitOption
			if lang != "" {
				opts = append(opts, port.WithLang(lang))
			}
			if _, err := adapter.InitProject(repoRoot, opts...); err != nil {
				return fmt.Errorf("init: %w", err)
			}
			if adapter.LastResult != nil {
				session.PrintInitResult(cmd.ErrOrStderr(), adapter.LastResult)
			}

			otelBackend := mustString(cmd, "otel-backend")
			if otelBackend != "" {
				otelEntity := mustString(cmd, "otel-entity")
				otelProject := mustString(cmd, "otel-project")
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
	cmd.Flags().Bool("force", false, "Overwrite existing state directory (re-initialize)")
	cmd.Flags().String("lang", "", "Language (ja/en)")
	cmd.Flags().String("otel-backend", "", "OTel backend: jaeger, weave")
	cmd.Flags().String("otel-entity", "", "Weave entity/team (required for weave)")
	cmd.Flags().String("otel-project", "", "Weave project (required for weave)")
	return cmd
}
