package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

func newArchivePruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive-prune",
		Short: "Prune old archived files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			days, _ := cmd.Flags().GetInt("days")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")

			if days < 1 {
				return fmt.Errorf("--days must be >= 1 (got %d)", days)
			}

			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			divRoot := filepath.Join(repoRoot, ".gate")
			archiveDir := filepath.Join(divRoot, "archive")
			eventsDir := filepath.Join(divRoot, "events")

			maxAge := time.Duration(days) * 24 * time.Hour
			errW := cmd.ErrOrStderr()

			// Collect archive candidates (.md files)
			archiveCandidates, err := amadeus.FindPruneCandidates(archiveDir, maxAge)
			if err != nil {
				return fmt.Errorf("find prune candidates: %w", err)
			}

			// Collect event file candidates (.jsonl files)
			eventCandidates, err := amadeus.FindExpiredEventFiles(eventsDir, maxAge)
			if err != nil {
				return fmt.Errorf("find expired event files: %w", err)
			}

			// Merge all candidates for display
			allCandidates := append(archiveCandidates, eventCandidates...)

			if len(allCandidates) == 0 {
				if archiveCandidates == nil && eventCandidates == nil {
					fmt.Fprintf(errW, "No prune directories found under %s\n", divRoot)
				} else {
					fmt.Fprintf(errW, "No files older than %d days to prune\n", days)
				}
				return nil
			}

			if len(archiveCandidates) > 0 {
				fmt.Fprintf(errW, "Archive files to prune (older than %d days):\n", days)
				for _, c := range archiveCandidates {
					fmt.Fprintf(errW, "  %s (modified %s)\n", filepath.Base(c.Path), c.ModTime.Format("2006-01-02"))
				}
			}
			if len(eventCandidates) > 0 {
				fmt.Fprintf(errW, "Event files to prune (older than %d days):\n", days)
				for _, c := range eventCandidates {
					fmt.Fprintf(errW, "  %s (modified %s)\n", filepath.Base(c.Path), c.ModTime.Format("2006-01-02"))
				}
			}

			if dryRun {
				fmt.Fprintf(errW, "\n(dry-run — no files deleted)\n")
				return nil
			}

			if !yes {
				fmt.Fprintf(errW, "\nDelete these %d file(s)? [y/N] ", len(allCandidates))
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

			totalCount := 0
			if len(archiveCandidates) > 0 {
				count, err := amadeus.PruneFiles(archiveCandidates)
				if err != nil {
					return fmt.Errorf("prune archive: %w", err)
				}
				totalCount += count
			}
			if len(eventCandidates) > 0 {
				count, err := amadeus.PruneFiles(eventCandidates)
				if err != nil {
					return fmt.Errorf("prune event files: %w", err)
				}
				totalCount += count
			}

			// Emit archive.pruned event (store filenames only for portability).
			// This MUST succeed: without the event, a future rebuild would replay
			// historical dmail.generated events and recreate the pruned files.
			var paths []string
			for _, c := range allCandidates {
				paths = append(paths, filepath.Base(c.Path))
			}
			eventStore := &amadeus.FileEventStore{Dir: eventsDir}
			ev, evErr := amadeus.NewEvent(amadeus.EventArchivePruned, amadeus.ArchivePrunedData{
				Paths: paths,
				Count: totalCount,
			}, time.Now().UTC())
			if evErr != nil {
				return fmt.Errorf("pruned %d file(s) but failed to create archive.pruned event: %w", totalCount, evErr)
			}
			if appendErr := eventStore.Append(ev); appendErr != nil {
				return fmt.Errorf("pruned %d file(s) but failed to record archive.pruned event: %w", totalCount, appendErr)
			}

			fmt.Fprintf(errW, "Pruned %d file(s).\n", totalCount)
			return nil
		},
	}

	cmd.Flags().IntP("days", "d", 30, "prune files older than N days")
	cmd.Flags().BoolP("dry-run", "n", false, "show what would be pruned without deleting")
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")

	return cmd
}
