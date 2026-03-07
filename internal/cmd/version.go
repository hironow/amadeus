package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build information",
		Long: `Print version, commit hash, build date, Go version, and OS/arch.

By default outputs a human-readable single line. Use --json
for structured output suitable for scripts and CI.`,
		Args: cobra.NoArgs,
		Example: `  amadeus version
  amadeus version -j`,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := map[string]string{
				"version": Version,
				"commit":  Commit,
				"date":    Date,
				"go":      runtime.Version(),
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "amadeus v%s (commit: %s, date: %s, go: %s)\n",
				Version, Commit, Date, runtime.Version())
			return nil
		},
	}

	cmd.Flags().BoolP("json", "j", false, "Output version info as JSON")

	return cmd
}
