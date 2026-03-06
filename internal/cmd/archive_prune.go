package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newArchivePruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive-prune [path]",
		Short: "Prune old archived files",
		Long: `Prune archived d-mail files and expired event files.

By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.`,
		Example: `  # Dry-run: list expired files (default 30 days)
  amadeus archive-prune

  # Delete expired files (with confirmation)
  amadeus archive-prune --execute

  # Delete without confirmation
  amadeus archive-prune --execute --yes

  # Custom retention period
  amadeus archive-prune --days 7 --execute

  # JSON output for scripting
  amadeus archive-prune -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			days, _ := cmd.Flags().GetInt("days")
			execute, _ := cmd.Flags().GetBool("execute")
			dryRunExplicit := cmd.Flags().Changed("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")
			outputFmt, _ := cmd.Flags().GetString("output")
			logger := platform.NewLogger(cmd.ErrOrStderr(), false)

			if execute && dryRunExplicit {
				return fmt.Errorf("--execute and --dry-run are mutually exclusive")
			}

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			rp, rpErr := domain.NewRepoPath(repoRoot)
			if rpErr != nil {
				return rpErr
			}
			d, dErr := domain.NewDays(days)
			if dErr != nil {
				return dErr
			}
			pruneCmd := domain.NewArchivePruneCommand(rp, d, !execute, yes)

			// Composition root: wire ArchiveOps and EventStore
			archiveOps := session.NewArchiveOps()

			// COMMAND → usecase (collect candidates)
			result, err := usecase.CollectPruneCandidates(cmd.Context(), pruneCmd, archiveOps)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, domain.StateDir)
			errW := cmd.ErrOrStderr()

			// Extract file names for output.
			archiveFiles := make([]string, 0, len(result.ArchiveCandidates))
			for _, c := range result.ArchiveCandidates {
				archiveFiles = append(archiveFiles, filepath.Base(c.Path))
			}

			if outputFmt == "json" {
				out := struct {
					ArchiveCandidates int      `json:"archive_candidates"`
					ArchiveDeleted    int      `json:"archive_deleted"`
					ArchiveFiles      []string `json:"archive_files"`
					EventCandidates   int      `json:"event_candidates"`
					EventDeleted      int      `json:"event_deleted"`
					EventFiles        []string `json:"event_files"`
				}{
					ArchiveCandidates: len(result.ArchiveCandidates),
					ArchiveFiles:      archiveFiles,
					EventCandidates:   len(result.EventCandidates),
					EventFiles:        result.EventCandidates,
				}
				if execute {
					eventStore := session.NewEventStore(divRoot, logger)
					totalCount, execErr := usecase.ExecutePrune(cmd.Context(), result, eventStore, archiveOps, divRoot, logger)
					if execErr != nil {
						return execErr
					}
					out.ArchiveDeleted = len(result.ArchiveCandidates)
					out.EventDeleted = totalCount - len(result.ArchiveCandidates)
				}
				data, jsonErr := json.Marshal(out)
				if jsonErr != nil {
					return jsonErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// text output — all metadata to stderr
			totalFiles := len(result.ArchiveCandidates) + len(result.EventCandidates)
			if totalFiles == 0 {
				if result.ArchiveCandidates == nil && result.EventCandidates == nil {
					fmt.Fprintf(errW, "No prune directories found under %s\n", divRoot)
				} else {
					fmt.Fprintf(errW, "No files older than %d days to prune\n", days)
				}
				return nil
			}

			if len(result.ArchiveCandidates) > 0 {
				fmt.Fprintf(errW, "Archive files to prune (older than %d days):\n", days)
				for _, c := range result.ArchiveCandidates {
					fmt.Fprintf(errW, "  %s (modified %s)\n", filepath.Base(c.Path), c.ModTime.Format("2006-01-02"))
				}
			}
			if len(result.EventCandidates) > 0 {
				fmt.Fprintf(errW, "Event files to prune (older than %d days):\n", days)
				for _, c := range result.EventCandidates {
					fmt.Fprintf(errW, "  %s\n", c)
				}
			}

			if !execute {
				fmt.Fprintln(errW, "(dry-run — pass --execute to delete)")
				return nil
			}

			if !yes {
				fmt.Fprintf(errW, "\nDelete these %d file(s)? [y/N] ", totalFiles)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return fmt.Errorf("read confirmation: %w", err)
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

			// usecase → execute prune + emit event
			eventStore := session.NewEventStore(divRoot, logger)
			totalCount, err := usecase.ExecutePrune(cmd.Context(), result, eventStore, archiveOps, divRoot, logger)
			if err != nil {
				return err
			}

			fmt.Fprintf(errW, "Pruned %d file(s).\n", totalCount)
			return nil
		},
	}

	cmd.Flags().IntP("days", "d", 30, "Retention days")
	cmd.Flags().BoolP("execute", "x", false, "Execute pruning (default: dry-run)")
	cmd.Flags().BoolP("dry-run", "n", false, "Dry-run mode (default behavior, explicit for scripting)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}
