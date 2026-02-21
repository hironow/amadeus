package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// BuildInfo holds version metadata injected at build time via ldflags.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func init() {
	cobra.EnableTraverseRunHooks = true
}

// NewRootCommand creates the root cobra command for amadeus.
// NOTE: NormalizeArgs (single-dash long-flag compat) was intentionally removed per MY-334.
// Only POSIX (-f) and GNU (--flag) forms are supported. See MY-335 for rationale.
func NewRootCommand(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "amadeus",
		Short:         "Divergence meter for your codebase",
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go handles error output
		SilenceUsage:  true,
		Version:       info.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no subcommand specified. Run 'amadeus help' for usage")
		},
	}

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
		newResolveCommand(),
		newArchivePruneCommand(),
		newVersionCommand(info),
		newUpdateCommand(info),
		newDocsCommand(),
	)

	return cmd
}
