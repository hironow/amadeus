// cmd/amadeus/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: amadeus <check|resolve|log> [flags]")
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

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	switch cmd {
	case "check":
		return runCheck(configPath, verbose, dryRun, full, quiet)
	case "resolve":
		return runResolve(configPath, verbose, fs.Args())
	case "log":
		return runLog(configPath, verbose)
	default:
		return fmt.Errorf("unknown command: %s (available: check, resolve, log)", cmd)
	}
}

func runCheck(configPath string, verbose, dryRun, full, quiet bool) error {
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

	logger := amadeus.NewLogger(os.Stdout, verbose)
	claude := amadeus.NewClaudeClient()
	claude.DryRun = dryRun

	a := &amadeus.Amadeus{
		Config: cfg,
		Store:  amadeus.NewStateStore(divRoot),
		Git:    amadeus.NewGitClient(repoRoot),
		Claude: claude,
		Logger: logger,
	}

	return a.RunCheck(context.Background(), amadeus.CheckOptions{
		Full:   full,
		DryRun: dryRun,
		Quiet:  quiet,
	})
}

func runLog(configPath string, verbose bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus check' first")
	}

	if configPath == "" {
		configPath = filepath.Join(divRoot, "config.yaml")
	}
	cfg, err := amadeus.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := amadeus.NewLogger(os.Stdout, verbose)
	a := &amadeus.Amadeus{
		Config: cfg,
		Store:  amadeus.NewStateStore(divRoot),
		Logger: logger,
	}
	return a.PrintLog()
}

func runResolve(configPath string, verbose bool, args []string) error {
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
		return fmt.Errorf(".divergence/ not found. Run 'amadeus check' first")
	}

	if configPath == "" {
		configPath = filepath.Join(divRoot, "config.yaml")
	}
	cfg, err := amadeus.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := amadeus.NewLogger(os.Stdout, verbose)
	a := &amadeus.Amadeus{
		Config: cfg,
		Store:  amadeus.NewStateStore(divRoot),
		Logger: logger,
	}

	action := "approve"
	if reject {
		action = "reject"
	}
	return a.ResolveDMail(id, action, reason)
}
