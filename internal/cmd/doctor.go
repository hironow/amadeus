package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Run health checks",
		Long: `Run health checks on the amadeus environment.

Each check reports one of four statuses: OK (passed), FAIL (exit 1),
SKIP (dependency missing), WARN (advisory, exit 0).

The context-budget check estimates token consumption per category
(tools, skills, plugins, mcp, hooks) and marks the heaviest.
When the threshold (20,000 tokens) is exceeded, a category-specific
hint recommends adjusting .claude/settings.json.`,
		Example: `  # Run environment check in current directory
  amadeus doctor

  # Check a specific project directory
  amadeus doctor /path/to/project

  # JSON output for scripting
  amadeus doctor -o json

  # Auto-fix repairable issues
  amadeus doctor --repair`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			jsonOut, _ := cmd.Flags().GetBool("json")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, domain.StateDir)
			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}

			logger := platform.NewLogger(cmd.ErrOrStderr(), false)
			repair, _ := cmd.Flags().GetBool("repair")
			linearFlag, _ := cmd.Flags().GetBool("linear")
			mode := domain.NewTrackingMode(linearFlag)
			results := runDoctor(cmd.Context(), configPath, repoRoot, logger, repair, mode)

			if jsonOut {
				return printDoctorJSON(cmd.OutOrStdout(), results)
			}
			return printDoctorText(cmd.ErrOrStderr(), logger, results)
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")
	cmd.Flags().Bool("repair", false, "Auto-fix repairable issues")

	return cmd
}

type jsonCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func printDoctorJSON(w io.Writer, results []domain.DoctorCheck) error {
	checks := make([]jsonCheck, len(results))
	hasFail := false
	for i, r := range results {
		checks[i] = jsonCheck{Name: r.Name, Status: r.Status.StatusLabel(), Message: r.Message, Hint: r.Hint}
		if r.Status == domain.CheckFail {
			hasFail = true
		}
	}
	data, err := json.MarshalIndent(struct {
		Checks []jsonCheck `json:"checks"`
	}{Checks: checks}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal doctor checks: %w", err)
	}
	fmt.Fprintln(w, string(data))
	if hasFail {
		return &domain.SilentError{Err: fmt.Errorf("some checks failed")}
	}
	return nil
}

func printDoctorText(w io.Writer, logger *platform.Logger, results []domain.DoctorCheck) error {
	fmt.Fprintln(w, "amadeus doctor — integrity health check")
	fmt.Fprintln(w)

	var fails, skips, warns int
	for _, r := range results {
		label := logger.Colorize(fmt.Sprintf("%-4s", r.Status.StatusLabel()), platform.StatusColor(r.Status))
		fmt.Fprintf(w, "  [%s] %-16s %s\n", label, r.Name, r.Message)
		if r.Hint != "" {
			fmt.Fprintf(w, "         %-16s hint: %s\n", "", r.Hint)
		}
		switch r.Status {
		case domain.CheckFail:
			fails++
		case domain.CheckSkip:
			skips++
		case domain.CheckWarn:
			warns++
		}
	}

	fmt.Fprintln(w)
	if fails == 0 && skips == 0 && warns == 0 {
		fmt.Fprintln(w, "All checks passed.")
		return nil
	}
	var parts []string
	if fails > 0 {
		parts = append(parts, fmt.Sprintf("%d check(s) failed", fails))
	}
	if warns > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warns))
	}
	if skips > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skips))
	}
	fmt.Fprintln(w, strings.Join(parts, ", ")+".")
	if fails > 0 {
		return &domain.SilentError{Err: fmt.Errorf("%d check(s) failed", fails)}
	}
	return nil
}

// runDoctor loads config and delegates to session.RunDoctorWithClaudeCmd.
func runDoctor(ctx context.Context, configPath string, repoRoot string, logger domain.Logger, repair bool, mode domain.TrackingMode) []domain.DoctorCheck {
	claudeCmd := domain.DefaultClaudeCmd
	if cfg, err := loadConfig(configPath); err == nil {
		claudeCmd = cfg.ClaudeCmd
	}
	return session.RunDoctorWithClaudeCmd(ctx, configPath, repoRoot, claudeCmd, logger, repair, mode, checkSuccessRate)
}

// checkSuccessRate calculates and reports the event-based success rate.
// Wires session.NewEventStore to usecase.ComputeSuccessRate.
func checkSuccessRate(gateDir string, logger domain.Logger) domain.DoctorCheck {
	eventStore := session.NewEventStore(gateDir, logger)
	rate, clean, total, err := usecase.ComputeSuccessRate(context.Background(), eventStore, logger)
	if err != nil || total == 0 {
		return domain.DoctorCheck{
			Name:    "success-rate",
			Status:  domain.CheckOK,
			Message: "no events",
		}
	}

	return domain.DoctorCheck{
		Name:    "success-rate",
		Status:  domain.CheckOK,
		Message: domain.FormatSuccessRate(rate, clean, total),
	}
}
