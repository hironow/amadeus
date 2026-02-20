package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

func newResolveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [names...]",
		Short: "Resolve D-Mail items",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			approve, _ := cmd.Flags().GetBool("approve")
			reject, _ := cmd.Flags().GetBool("reject")
			reason, _ := cmd.Flags().GetString("reason")
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			jsonOut, _ := cmd.Flags().GetBool("json")

			names := args
			if len(names) == 0 {
				stdinNames, err := readNamesFromStdin()
				if err != nil {
					return err
				}
				names = stdinNames
			}

			if approve == reject {
				return fmt.Errorf("specify exactly one of --approve or --reject")
			}
			if reject && reason == "" {
				return fmt.Errorf("--reason is required with --reject")
			}
			if len(names) == 0 {
				return fmt.Errorf("usage: amadeus resolve <name> --approve or --reject --reason \"...\"")
			}

			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, ".gate")

			if _, err := os.Stat(divRoot); os.IsNotExist(err) {
				return fmt.Errorf(".gate/ not found. Run 'amadeus init' first")
			}

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := amadeus.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger := amadeus.NewLogger(os.Stderr, verbose)
			a := &amadeus.Amadeus{
				Config:  cfg,
				Store:   amadeus.NewStateStore(divRoot),
				Logger:  logger,
				DataOut: os.Stdout,
			}

			action := "approve"
			if reject {
				action = "reject"
			}

			ctx := context.Background()

			if jsonOut {
				return resolveJSON(ctx, a, names, action, reason)
			}
			return resolveText(ctx, a, names, action, reason)
		},
	}

	cmd.Flags().Bool("approve", false, "approve the D-Mail")
	cmd.Flags().Bool("reject", false, "reject the D-Mail")
	cmd.Flags().String("reason", "", "reason for rejection")
	cmd.Flags().Bool("json", false, "output as JSON")

	return cmd
}

func resolveJSON(ctx context.Context, a *amadeus.Amadeus, names []string, action, reason string) error {
	var results []amadeus.ResolveOutput
	var firstErr error
	for _, name := range names {
		result, resolveErr := a.ResolveDMailResult(ctx, name, action, reason)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, resolveErr)
			if firstErr == nil {
				firstErr = resolveErr
			}
			continue
		}
		results = append(results, result)
	}
	if results == nil {
		results = []amadeus.ResolveOutput{}
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal resolve results: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(data))
	return firstErr
}

func resolveText(ctx context.Context, a *amadeus.Amadeus, names []string, action, reason string) error {
	var firstErr error
	for _, name := range names {
		if resolveErr := a.ResolveDMail(ctx, name, action, reason); resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, resolveErr)
			if firstErr == nil {
				firstErr = resolveErr
			}
		}
	}
	return firstErr
}

// readNamesFromStdin reads D-Mail names from stdin when piped (non-TTY).
// Returns nil if stdin is a terminal.
func readNamesFromStdin() ([]string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, nil
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil // stdin is a terminal, not a pipe
	}
	var names []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			names = append(names, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	return names, nil
}
