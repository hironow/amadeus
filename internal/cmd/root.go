package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// NormalizeArgs rewrites legacy single-dash long flags for backward compatibility.
// The old stdlib flag-based CLI accepted -config, -json, etc.; cobra/pflag requires --.
// Only tokens whose name (after stripping the leading dash) matches a registered flag
// are rewritten, so values like -bad or filenames like -foo.yaml are left intact.
func NormalizeArgs(cmd *cobra.Command, args []string) []string {
	flags := collectFlags(cmd)
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			name := a[1:]
			if idx := strings.Index(name, "="); idx >= 0 {
				name = name[:idx]
			}
			if flags[name] {
				out[i] = "-" + a
			}
		}
	}
	return out
}

// collectFlags gathers all registered flag names from cmd and its subcommands.
func collectFlags(cmd *cobra.Command) map[string]bool {
	flags := map[string]bool{"help": true, "version": true}
	var visit func(*cobra.Command)
	visit = func(c *cobra.Command) {
		addFlags := func(fs *pflag.FlagSet) {
			fs.VisitAll(func(f *pflag.Flag) { flags[f.Name] = true })
		}
		addFlags(c.Flags())
		addFlags(c.PersistentFlags())
		for _, sub := range c.Commands() {
			visit(sub)
		}
	}
	visit(cmd)
	return flags
}

// NewRootCommand creates the root cobra command for amadeus.
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
