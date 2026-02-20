// cmd/amadeus/main.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
		return fmt.Errorf("usage: amadeus <init|check|resolve|log|sync|link|doctor> [flags]")
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
	case "sync":
		return runSync(configPath, verbose)
	case "link":
		return runLink(configPath, verbose, jsonOut, fs.Args())
	case "init":
		return runInit()
	case "doctor":
		return runDoctor(configPath, jsonOut)
	default:
		return fmt.Errorf("unknown command: %s (available: init, check, resolve, log, sync, link, doctor)", cmd)
	}
}

func runCheck(configPath string, verbose, dryRun, full, quiet, jsonOut bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	divRoot := filepath.Join(repoRoot, ".divergence")

	if err := amadeus.InitDivergenceDir(divRoot); err != nil {
		return fmt.Errorf("init .divergence: %w", err)
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
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus init' first")
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

func runResolve(configPath string, verbose, jsonOut bool, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: amadeus resolve <id> --approve or --reject --reason \"...\"")
	}
	id := args[0]

	fs := flag.NewFlagSet("resolve-action", flag.ContinueOnError)
	var approve, reject bool
	var reason string
	fs.BoolVar(&approve, "approve", false, "approve D-Mail")
	fs.BoolVar(&reject, "reject", false, "reject D-Mail")
	fs.StringVar(&reason, "reason", "", "rejection reason (required with --reject)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if approve == reject {
		return fmt.Errorf("specify exactly one of --approve or --reject")
	}
	if reject && reason == "" {
		return fmt.Errorf("--reason is required with --reject")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus init' first")
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
	if jsonOut {
		return a.ResolveDMailJSON(context.Background(), id, action, reason)
	}
	return a.ResolveDMail(context.Background(), id, action, reason)
}

func runSync(configPath string, verbose bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus init' first")
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
	return a.PrintSync()
}

func runLink(configPath string, verbose, jsonOut bool, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: amadeus link <dmail-id> <linear-issue-id>")
	}
	dmailID := args[0]
	linearIssueID := args[1]

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus init' first")
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
		return a.LinkDMailJSON(dmailID, linearIssueID)
	}
	return a.LinkDMail(dmailID, linearIssueID)
}

func runInit() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	divRoot := filepath.Join(repoRoot, ".divergence")
	if err := amadeus.InitDivergenceDir(divRoot); err != nil {
		return fmt.Errorf("init .divergence: %w", err)
	}
	fmt.Printf("  Initialized %s\n", divRoot)
	return nil
}

func runDoctor(configPath string, jsonOut bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	divRoot := filepath.Join(repoRoot, ".divergence")
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
