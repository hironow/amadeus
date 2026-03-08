package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/spf13/cobra"
)

type loggerKeyType struct{}

var loggerKey loggerKeyType

// Version, Commit, and Date are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "dev"
	Date    = "dev"
)

// shutdownTracer holds the OTel tracer shutdown function registered by
// PersistentPreRunE. cobra.OnFinalize calls it after Execute completes.
var (
	shutdownTracer func(context.Context) error
	shutdownMeter  func(context.Context) error
	finalizerOnce  sync.Once
)

func init() {
	cobra.EnableTraverseRunHooks = true
}

// NewRootCommand creates the root cobra command for amadeus.
// NOTE: NormalizeArgs (single-dash long-flag compat) was intentionally removed per MY-334.
// Only POSIX (-f) and GNU (--flag) forms are supported. See MY-335 for rationale.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "amadeus",
		Short:         "Divergence meter for your codebase",
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go handles error output [permanent]
		SilenceUsage:  true,
		Version:       Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			if cfgPath != "" {
				applyOtelEnv(filepath.Dir(cfgPath))
			} else {
				applyOtelEnv(domain.StateDir)
			}
			noColor, _ := cmd.Flags().GetBool("no-color")
			if noColor {
				os.Setenv("NO_COLOR", "1")
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)
			ctx := context.WithValue(cmd.Context(), loggerKey, logger)
			shutdownTracer = initTracer("amadeus", Version)
			shutdownMeter = initMeter("amadeus", Version)
			spanCtx := startRootSpan(ctx, cmd.Name())
			cmd.SetContext(spanCtx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no subcommand specified. Run 'amadeus help' for usage")
		},
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			endRootSpan()
			if shutdownMeter != nil {
				shutdownMeter(context.Background())
			}
			if shutdownTracer != nil {
				shutdownTracer(context.Background())
			}
		})
	})

	cmd.PersistentFlags().StringP("config", "c", "", "config file path")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	cmd.PersistentFlags().Bool("no-color", false, "Disable colored output (respects NO_COLOR env)")
	cmd.PersistentFlags().StringP("lang", "l", "", "output language (ja, en)")
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")

	cmd.AddCommand(
		newInitCommand(),
		newValidateCommand(),
		newInstallHookCommand(),
		newUninstallHookCommand(),
		newLogCommand(),
		newDoctorCommand(),
		newCheckCommand(),
		newRunCommand(),
		newSyncCommand(),
		newMarkCommentedCommand(),
		newArchivePruneCommand(),
		newCleanCommand(),
		newRebuildCommand(),
		newStatusCommand(),
		newVersionCommand(),
		newUpdateCommand(),
		newConfigCommand(),
	)

	return cmd
}

// loggerFrom extracts the domain.Logger from the cobra command context.
// Falls back to a stderr logger if PersistentPreRunE was not executed (e.g., in tests).
func loggerFrom(cmd *cobra.Command) domain.Logger {
	if l, ok := cmd.Context().Value(loggerKey).(domain.Logger); ok {
		return l
	}
	return platform.NewLogger(cmd.ErrOrStderr(), false)
}
