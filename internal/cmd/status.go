package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
)

// newStatusCommand creates the status subcommand that displays operational status.
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [path]",
		Short: "Show amadeus operational status",
		Long: `Display operational status including check history, divergence,
success rate, and pending d-mail counts.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output to stdout.
Use --history N to show a sparkline of the last N baseline divergence points.`,
		Example: `  # Show status for current directory
  amadeus status

  # Show status for a specific project
  amadeus status /path/to/project

  # JSON output for scripting
  amadeus status -o json

  # Show sparkline of last 20 baseline points
  amadeus status --history 20`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, domain.StateDir)
			if _, err := os.Stat(divRoot); errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf(".gate/ not found. Run 'amadeus init' first")
			}

			logger := platform.NewLogger(cmd.ErrOrStderr(), false)
			report := session.Status(cmd.Context(), divRoot, logger)

			outputFmt := mustString(cmd, "output")
			if outputFmt == "json" {
				data, jsonErr := json.Marshal(report)
				if jsonErr != nil {
					return fmt.Errorf("marshal status: %w", jsonErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// Text output to stdout (human-readable, per S0027)
			fmt.Fprint(cmd.OutOrStdout(), report.FormatText())

			// Sparkline history display
			historyN := mustInt(cmd, "history")
			if historyN > 0 && len(report.BaselineHistory) > 0 {
				points := report.BaselineHistory
				if len(points) > historyN {
					points = points[len(points)-historyN:]
				}
				values := make([]float64, len(points))
				for i, p := range points {
					values[i] = p.Divergence
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Baseline history (%d points):\n  %s\n", len(points), domain.Sparkline(values))
			}

			return nil
		},
	}
	cmd.Flags().Int("history", 0, "show sparkline of last N baseline divergence points")
	return cmd
}
