package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
)

// newDashboardCommand creates the dashboard subcommand that displays cross-repo divergence.
func newDashboardCommand() *cobra.Command {
	var toolDirsFlag string

	cmd := &cobra.Command{
		Use:   "dashboard [path]",
		Short: "Show TAP ecosystem divergence dashboard",
		Long: `Display cross-repository divergence status for the TAP ecosystem.

Shows divergence scores for all tools (phonewave, sightjack, paintress, amadeus)
with an aggregated ecosystem score.

Default tool directory resolution uses sibling directory convention:
  ../phonewave/.phonewave/  ../sightjack/.siren/
  ../paintress/.expedition/  ./.gate/

Output goes to stdout (human-readable text by default).
Use -o json for machine-readable JSON output.`,
		Example: `  # Show ecosystem dashboard
  amadeus dashboard

  # Show dashboard for a specific project root
  amadeus dashboard /path/to/project

  # JSON output for scripting
  amadeus dashboard -o json

  # Custom tool directories (comma-separated tool=dir pairs)
  amadeus dashboard --tool-dirs "phonewave=/other/.phonewave,sightjack=/other/.siren"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			logger := loggerFrom(cmd)

			// Build tool state dirs
			siblingRoot := filepath.Dir(repoRoot)
			toolStateDirs := session.ResolveToolStateDirs(repoRoot, siblingRoot)

			// Apply --tool-dirs overrides
			if toolDirsFlag != "" {
				if err := applyToolDirOverrides(toolStateDirs, toolDirsFlag); err != nil {
					return err
				}
			}

			reader := session.NewFileCrossRepoReader(logger)
			snapshot, err := usecase.ReadCrossRepoSnapshot(cmd.Context(), toolStateDirs, reader)
			if err != nil {
				return fmt.Errorf("read cross-repo snapshot: %w", err)
			}

			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt == "json" {
				data, jsonErr := json.Marshal(snapshot)
				if jsonErr != nil {
					return fmt.Errorf("marshal snapshot: %w", jsonErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprint(cmd.OutOrStdout(), formatDashboardText(snapshot))
			return nil
		},
	}

	cmd.Flags().StringVar(&toolDirsFlag, "tool-dirs", "", "Override tool state dirs (comma-separated tool=dir pairs)")

	return cmd
}

// applyToolDirOverrides parses the --tool-dirs flag and updates the map.
func applyToolDirOverrides(dirs map[domain.ToolName]string, flag string) error {
	for _, pair := range strings.Split(flag, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid tool-dirs entry %q: expected tool=dir", pair)
		}
		tool := domain.ToolName(strings.TrimSpace(parts[0]))
		dir := strings.TrimSpace(parts[1])
		if domain.ToolStateDir(tool) == "" {
			return fmt.Errorf("unknown tool %q in --tool-dirs", tool)
		}
		dirs[tool] = dir
	}
	return nil
}

// formatDashboardText renders the dashboard as human-readable text.
func formatDashboardText(snap domain.CrossRepoSnapshot) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "TAP Ecosystem Dashboard  %s\n\n", snap.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&sb, "  %-14s %-12s %-10s %s\n", "tool", "divergence", "severity", "last_check")

	for _, ts := range snap.Snapshots {
		if !ts.Available {
			fmt.Fprintf(&sb, "  %-14s %-12s %-10s %s\n", ts.Tool, "-", "-", "(unavailable)")
			continue
		}

		divStr := "-"
		sevStr := "-"
		lastStr := "(available)"

		if ts.Divergence > 0 || !ts.LastCheck.IsZero() {
			divStr = fmt.Sprintf("%.2f", ts.Divergence)
			sevStr = string(ts.Severity)
		}
		if !ts.LastCheck.IsZero() {
			lastStr = ts.LastCheck.Format("2006-01-02T15:04:05Z")
		}

		fmt.Fprintf(&sb, "  %-14s %-12s %-10s %s\n", ts.Tool, divStr, sevStr, lastStr)
	}

	fmt.Fprintln(&sb)

	ecoDiv := fmt.Sprintf("%.2f", snap.EcosystemScore)
	fmt.Fprintf(&sb, "  %-14s %-12s %s\n", "ecosystem", ecoDiv, snap.MaxSeverity)

	return sb.String()
}
