package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/spf13/cobra"
)

func newDeadLettersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dead-letters",
		Short: "Manage dead-lettered outbox items",
		Long: `Inspect and manage outbox items that have exceeded the maximum retry
count and are permanently stuck.`,
	}

	cmd.AddCommand(newDeadLettersPurgeCommand())

	return cmd
}

func newDeadLettersPurgeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge [path]",
		Short: "Purge dead-lettered outbox items",
		Long: `Remove outbox items that have exceeded the maximum retry count.

By default, runs in dry-run mode showing the count of dead-lettered items.
Pass --execute to actually delete them.`,
		Example: `  # Dry-run: show count of dead-lettered items
  amadeus dead-letters purge

  # Delete dead-lettered items (with confirmation)
  amadeus dead-letters purge --execute

  # Delete without confirmation
  amadeus dead-letters purge --execute --yes

  # JSON output for scripting
  amadeus dead-letters purge -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			execute, _ := cmd.Flags().GetBool("execute")
			yes, _ := cmd.Flags().GetBool("yes")
			outputFmt, _ := cmd.Flags().GetString("output")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, domain.StateDir)

			// Pre-flight: check DB file exists before opening (avoid creating dirs/DB as side effect)
			dbPath := filepath.Join(divRoot, ".run", "outbox.db")
			if _, statErr := os.Stat(dbPath); statErr != nil {
				if outputFmt == "json" {
					data, jsonErr := json.Marshal(struct {
						Count  int `json:"count"`
						Purged int `json:"purged"`
					}{Count: 0, Purged: 0})
					if jsonErr != nil {
						return jsonErr
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
					return nil
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "No outbox database found — nothing to purge.")
				return nil
			}

			store, storeErr := session.NewOutboxStoreForDir(divRoot)
			if storeErr != nil {
				return fmt.Errorf("open outbox store: %w", storeErr)
			}
			defer store.Close()

			ctx := cmd.Context()

			count, countErr := store.DeadLetterCount(ctx)
			if countErr != nil {
				return countErr
			}

			if outputFmt == "json" {
				out := struct {
					Count  int `json:"count"`
					Purged int `json:"purged"`
				}{Count: count, Purged: 0}
				if execute && count > 0 {
					purged, purgeErr := store.PurgeDeadLetters(ctx)
					if purgeErr != nil {
						return purgeErr
					}
					out.Purged = purged
				}
				data, jsonErr := json.Marshal(out)
				if jsonErr != nil {
					return jsonErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// text output
			errW := cmd.ErrOrStderr()

			if count == 0 {
				fmt.Fprintln(errW, "No dead-lettered items.")
				return nil
			}

			fmt.Fprintf(errW, "%d dead-lettered outbox item(s) found.\n", count)

			if !execute {
				fmt.Fprintln(errW, "(dry-run — pass --execute to delete)")
				return nil
			}

			if !yes {
				fmt.Fprintf(errW, "Delete %d dead-lettered item(s)? [y/N] ", count)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() {
					if scanErr := scanner.Err(); scanErr != nil {
						return fmt.Errorf("read confirmation: %w", scanErr)
					}
					fmt.Fprintln(errW, "Cancelled.")
					return nil
				}
				answer := strings.TrimSpace(scanner.Text())
				if answer != "y" && answer != "Y" {
					fmt.Fprintln(errW, "Cancelled.")
					return nil
				}
			}

			purged, purgeErr := store.PurgeDeadLetters(ctx)
			if purgeErr != nil {
				return purgeErr
			}

			fmt.Fprintf(errW, "Purged %d dead-lettered item(s).\n", purged)
			return nil
		},
	}

	cmd.Flags().BoolP("execute", "x", false, "Execute purge (default: dry-run)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}
