package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Run health checks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			jsonOut, _ := cmd.Flags().GetBool("json")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, ".gate")
			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}

			logger := platform.NewLogger(cmd.ErrOrStderr(), false)
			results := runDoctor(cmd.Context(), configPath, repoRoot, logger)

			if jsonOut {
				return printDoctorJSON(cmd.OutOrStdout(), results)
			}
			return printDoctorText(cmd.ErrOrStderr(), results)
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}

type jsonCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func printDoctorJSON(w io.Writer, results []domain.DoctorCheckResult) error {
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
		return fmt.Errorf("some checks failed")
	}
	return nil
}

func printDoctorText(w io.Writer, results []domain.DoctorCheckResult) error {
	hasFail := false
	for _, r := range results {
		fmt.Fprintf(w, "  [%-4s] %-16s %s\n", r.Status.StatusLabel(), r.Name, r.Message)
		if r.Hint != "" {
			fmt.Fprintf(w, "         %-16s hint: %s\n", "", r.Hint)
		}
		if r.Status == domain.CheckFail {
			hasFail = true
		}
	}
	if hasFail {
		return fmt.Errorf("some checks failed")
	}
	return nil
}
