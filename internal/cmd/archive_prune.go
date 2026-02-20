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
			archiveDir := filepath.Join(repoRoot, ".gate", "archive")

			maxAge := time.Duration(days) * 24 * time.Hour
			candidates, err := amadeus.FindPruneCandidates(archiveDir, maxAge)
			if err != nil {
				return fmt.Errorf("find prune candidates: %w", err)
			}

			if candidates == nil {
				fmt.Fprintf(os.Stderr, "Archive directory does not exist: %s\n", archiveDir)
				return nil
			}
			if len(candidates) == 0 {
				fmt.Fprintf(os.Stderr, "No files older than %d days in %s\n", days, archiveDir)
				return nil
			}

			fmt.Fprintf(os.Stderr, "Files to prune in %s (older than %d days):\n", archiveDir, days)
			for _, c := range candidates {
				fmt.Fprintf(os.Stderr, "  %s (modified %s)\n", filepath.Base(c.Path), c.ModTime.Format("2006-01-02"))
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "\n(dry-run — no files deleted)\n")
				return nil
			}

			if !yes {
				fmt.Fprintf(os.Stderr, "\nDelete these %d file(s)? [y/N] ", len(candidates))
				scanner := bufio.NewScanner(os.Stdin)
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return fmt.Errorf("read confirmation: %w", err)
					}
					fmt.Fprintln(os.Stderr, "Cancelled.")
					return nil
				}
				answer := strings.TrimSpace(scanner.Text())
				if answer != "y" && answer != "Y" {
					fmt.Fprintln(os.Stderr, "Cancelled.")
					return nil
				}
			}

			count, err := amadeus.PruneFiles(candidates)
			if err != nil {
				return fmt.Errorf("prune: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Pruned %d file(s).\n", count)
			return nil
		},
	}

	cmd.Flags().Int("days", 30, "prune files older than N days")
	cmd.Flags().Bool("dry-run", false, "show what would be pruned without deleting")
	cmd.Flags().Bool("yes", false, "skip confirmation prompt")

	return cmd
}
