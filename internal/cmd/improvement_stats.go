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
	"github.com/hironow/amadeus/internal/session"
)

// newImprovementStatsCommand creates the improvement-stats subcommand.
func newImprovementStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "improvement-stats [path]",
		Short: "Show improvement outcome statistics",
		Long: `Display outcome statistics from the self-improving loop.
Shows resolved/failed_again/escalated/pending counts grouped by failure type.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output.`,
		Example: `  # Show improvement stats for current directory
  amadeus improvement-stats

  # JSON output for scripting
  amadeus improvement-stats -o json`,
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

			dbPath := filepath.Join(divRoot, ".run", "improvement-ingestion.db")
			store, storeErr := session.NewSQLiteImprovementCollectorStore(dbPath)
			if storeErr != nil {
				return fmt.Errorf("open improvement store: %w", storeErr)
			}
			defer store.Close()

			stats, queryErr := store.GetOutcomeStats(cmd.Context())
			if queryErr != nil {
				return fmt.Errorf("query stats: %w", queryErr)
			}

			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt == "json" {
				data, jsonErr := json.Marshal(stats)
				if jsonErr != nil {
					return fmt.Errorf("marshal stats: %w", jsonErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			if len(stats) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No improvement signals recorded yet.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Improvement Outcome Stats:")
			for _, s := range stats {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-25s resolved=%d  failed_again=%d  escalated=%d  pending=%d\n",
					s.FailureType+":", s.Resolved, s.FailedAgain, s.Escalated, s.Pending)
			}
			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output format: json")
	return cmd
}
