package cmd

import (
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
func NewRootCommand(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "amadeus",
		Short:         "Divergence meter for your codebase",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       info.Version,
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
