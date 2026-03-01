package cmd

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newArchivePruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive-prune",
		Short: "Prune old archived files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			days, _ := cmd.Flags().GetInt("days")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			pruneCmd := amadeus.ArchivePruneCommand{
				RepoPath: repoRoot,
				Days:     days,
				DryRun:   dryRun,
				Yes:      yes,
			}

			// COMMAND → usecase (collect candidates)
			result, err := usecase.CollectPruneCandidates(pruneCmd)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, ".gate")
			eventsDir := filepath.Join(divRoot, "events")
			errW := cmd.ErrOrStderr()

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

			if dryRun {
				fmt.Fprintf(errW, "\n(dry-run — no files deleted)\n")
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
			totalCount, err := usecase.ExecutePrune(result, divRoot, eventsDir)
			if err != nil {
				return err
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
