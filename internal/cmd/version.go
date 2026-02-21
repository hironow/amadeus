package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")

			if jsonOut {
				data, err := json.Marshal(map[string]string{
					"version": info.Version,
					"commit":  info.Commit,
					"date":    info.Date,
				})
				if err != nil {
					return fmt.Errorf("marshal version info: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "amadeus version %s (commit: %s, built: %s)\n",
				info.Version, info.Commit, info.Date)
			return nil
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}
