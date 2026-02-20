package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			jsonOut, _ := cmd.Flags().GetBool("json")

			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			divRoot := filepath.Join(repoRoot, ".gate")
			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}

			ctx := context.Background()
			results := amadeus.RunDoctor(ctx, configPath, repoRoot)

			if jsonOut {
				return printDoctorJSON(results)
			}
			return printDoctorText(results)
		},
	}

	cmd.Flags().Bool("json", false, "output as JSON")

	return cmd
}

type jsonCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func printDoctorJSON(results []amadeus.DoctorCheckResult) error {
	checks := make([]jsonCheck, len(results))
	hasFail := false
	for i, r := range results {
		checks[i] = jsonCheck{Name: r.Name, Status: r.Status.StatusLabel(), Message: r.Message}
		if r.Status == amadeus.CheckFail {
			hasFail = true
		}
	}
	data, err := json.MarshalIndent(struct {
		Checks []jsonCheck `json:"checks"`
	}{Checks: checks}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal doctor checks: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(data))
	if hasFail {
		return fmt.Errorf("some checks failed")
	}
	return nil
}

func printDoctorText(results []amadeus.DoctorCheckResult) error {
	hasFail := false
	for _, r := range results {
		fmt.Printf("  [%-4s] %-16s %s\n", r.Status.StatusLabel(), r.Name, r.Message)
		if r.Status == amadeus.CheckFail {
			hasFail = true
		}
	}
	if hasFail {
		return fmt.Errorf("some checks failed")
	}
	return nil
}
