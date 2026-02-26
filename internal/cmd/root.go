package cmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

type loggerKeyType struct{}

var loggerKey loggerKeyType

// Version, Commit, Date are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// shutdownTracer holds the OTel tracer shutdown function registered by
// PersistentPreRunE. cobra.OnFinalize calls it after Execute completes.
var (
	shutdownTracer func(context.Context) error
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
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go handles error output
		SilenceUsage:  true,
		Version:       Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := amadeus.NewLogger(cmd.ErrOrStderr(), verbose)
			ctx := context.WithValue(cmd.Context(), loggerKey, logger)
			shutdownTracer = initTracer("amadeus", Version)
			cmd.SetContext(ctx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no subcommand specified. Run 'amadeus help' for usage")
		},
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			if shutdownTracer != nil {
				shutdownTracer(context.Background())
			}
		})
	})

	cmd.PersistentFlags().StringP("config", "c", "", "config file path")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	cmd.PersistentFlags().StringP("lang", "l", "", "output language (ja, en)")

	cmd.AddCommand(
		newInitCommand(),
		newValidateCommand(),
		newInstallHookCommand(),
		newUninstallHookCommand(),
		newLogCommand(),
		newDoctorCommand(),
		newCheckCommand(),
		newSyncCommand(),
		newMarkCommentedCommand(),
		newArchivePruneCommand(),
		newRebuildCommand(),
		newVersionCommand(),
		newUpdateCommand(),
	)

	return cmd
}

// loggerFrom extracts the *amadeus.Logger from the cobra command context.
// Falls back to a stderr logger if PersistentPreRunE was not executed (e.g., in tests).
func loggerFrom(cmd *cobra.Command) *amadeus.Logger {
	if l, ok := cmd.Context().Value(loggerKey).(*amadeus.Logger); ok {
		return l
	}
	return amadeus.NewLogger(cmd.ErrOrStderr(), false)
}
