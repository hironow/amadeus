// cmd/amadeus/main.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus"
)

var version = "dev"

func main() {
	shutdown := amadeus.InitTracer("amadeus", version)

	err := run()
	code := amadeus.ExitCode(err)
	if code == 1 {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	} else if code == 2 {
		fmt.Fprintf(os.Stderr, "drift detected: %v\n", err)
	}
	shutdown(context.Background())
	os.Exit(code)
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: amadeus <init|check|resolve|log|doctor|validate|install-hook|uninstall-hook> [flags]")
	}

	if os.Args[1] == "--version" || os.Args[1] == "-version" {
		fmt.Printf("amadeus %s\n", version)
		return nil
	}

	cmd := os.Args[1]

	var (
		configPath string
		verbose    bool
		dryRun     bool
		full       bool
		quiet      bool
		jsonOut    bool
	)

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.StringVar(&configPath, "c", "", "config file path")
	fs.StringVar(&configPath, "config", "", "config file path")
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.BoolVar(&dryRun, "dry-run", false, "generate prompt only")
	fs.BoolVar(&full, "full", false, "force full calibration check")
	fs.BoolVar(&quiet, "quiet", false, "summary-only output")
	fs.BoolVar(&quiet, "q", false, "summary-only output")
	fs.BoolVar(&jsonOut, "json", false, "output as JSON")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	switch cmd {
	case "check":
		return runCheck(configPath, verbose, dryRun, full, quiet, jsonOut)
	case "resolve":
		return runResolve(configPath, verbose, jsonOut, fs.Args())
	case "log":
		return runLog(configPath, verbose, jsonOut)
	case "init":
		return runInit()
	case "doctor":
		return runDoctor(configPath, jsonOut)
	case "validate":
		return runValidate(configPath)
	case "install-hook":
		return runInstallHook()
	case "uninstall-hook":
		return runUninstallHook()
	default:
		return fmt.Errorf("unknown command: %s (available: init, check, resolve, log, doctor, validate, install-hook, uninstall-hook)", cmd)
	}
}

func runCheck(configPath string, verbose, dryRun, full, quiet, jsonOut bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	divRoot := filepath.Join(repoRoot, ".gate")

	if err := amadeus.InitGateDir(divRoot); err != nil {
		return fmt.Errorf("init .gate: %w", err)
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
		Git:     amadeus.NewGitClient(repoRoot),
		Logger:  logger,
		DataOut: os.Stdout,
	}

	return a.RunCheck(context.Background(), amadeus.CheckOptions{
		Full:   full,
		DryRun: dryRun,
		Quiet:  quiet,
		JSON:   jsonOut,
	})
}

func runLog(configPath string, verbose, jsonOut bool) error {
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
	if jsonOut {
		return a.PrintLogJSON()
	}
	return a.PrintLog()
}

// resolveArgs holds the parsed arguments for the resolve command.
type resolveArgs struct {
	approve bool
	reject  bool
	reason  string
	names   []string
}

// parseResolveArgs extracts resolve-specific flags (--approve, --reject, --reason)
// from a mixed list of flags and positional names.
// Go's flag parser stops at the first non-flag argument, so flags appearing
// after a name (e.g. "feedback-001 --approve") would be left unparsed.
// This manual scan handles interspersed flags and names in any order.
func parseResolveArgs(args []string) resolveArgs {
	var ra resolveArgs
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--approve":
			ra.approve = true
		case args[i] == "--reject":
			ra.reject = true
		case args[i] == "--reason" && i+1 < len(args):
			ra.reason = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--reason="):
			ra.reason = strings.TrimPrefix(args[i], "--reason=")
		default:
			ra.names = append(ra.names, args[i])
		}
	}
	return ra
}

func runResolve(configPath string, verbose, jsonOut bool, rawArgs []string) error {
	ra := parseResolveArgs(rawArgs)

	// Also collect names from stdin if none provided as args
	if len(ra.names) == 0 {
		stdinNames, err := readNamesFromStdin()
		if err != nil {
			return err
		}
		ra.names = stdinNames
	}

	if ra.approve == ra.reject {
		return fmt.Errorf("specify exactly one of --approve or --reject")
	}
	if ra.reject && ra.reason == "" {
		return fmt.Errorf("--reason is required with --reject")
	}
	if len(ra.names) == 0 {
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
	if ra.reject {
		action = "reject"
	}

	ctx := context.Background()

	if jsonOut {
		// Batch mode: collect all results and write as a JSON array.
		var results []amadeus.ResolveOutput
		var firstErr error
		for _, name := range ra.names {
			result, resolveErr := a.ResolveDMailResult(ctx, name, action, ra.reason)
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

	// Text mode: print each result individually.
	var firstErr error
	for _, name := range ra.names {
		if resolveErr := a.ResolveDMail(ctx, name, action, ra.reason); resolveErr != nil {
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

func runInit() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	divRoot := filepath.Join(repoRoot, ".gate")
	if err := amadeus.InitGateDir(divRoot); err != nil {
		return fmt.Errorf("init .gate: %w", err)
	}
	fmt.Printf("  Initialized %s\n", divRoot)
	return nil
}

func runValidate(configPath string) error {
	if configPath == "" {
		repoRoot, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		configPath = filepath.Join(repoRoot, ".gate", "config.yaml")
	}
	cfg, err := amadeus.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	errs := amadeus.ValidateConfig(cfg)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  [FAIL] %s\n", e)
		}
		return fmt.Errorf("%d validation error(s)", len(errs))
	}
	fmt.Printf("  [OK] %s is valid\n", configPath)
	return nil
}

func runInstallHook() error {
	gitDir, err := findGitDir()
	if err != nil {
		return err
	}
	if err := amadeus.InstallHook(gitDir); err != nil {
		return err
	}
	fmt.Printf("  Installed post-merge hook in %s\n", filepath.Join(gitDir, "hooks", "post-merge"))
	return nil
}

func runUninstallHook() error {
	gitDir, err := findGitDir()
	if err != nil {
		return err
	}
	if err := amadeus.UninstallHook(gitDir); err != nil {
		return err
	}
	fmt.Println("  Removed amadeus post-merge hook")
	return nil
}

func findGitDir() (string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	gitDir := filepath.Join(repoRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", fmt.Errorf("not a git repository (no .git directory)")
	}
	return gitDir, nil
}

func runDoctor(configPath string, jsonOut bool) error {
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
		type jsonCheck struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		}
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
