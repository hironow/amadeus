package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnableTraverseRunHooks = true
}

// NewRootCommand creates the root cobra command for amadeus.
func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "amadeus",
		Short:         "Divergence meter for your codebase",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
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
	)

	return cmd
}
