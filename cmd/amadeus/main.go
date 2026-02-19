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
		return fmt.Errorf("usage: amadeus <check> [flags]")
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
	)

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.StringVar(&configPath, "c", "", "config file path")
	fs.StringVar(&configPath, "config", "", "config file path")
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.BoolVar(&dryRun, "dry-run", false, "generate prompt only")
	fs.BoolVar(&full, "full", false, "force full calibration check")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	switch cmd {
	case "check":
		return runCheck(configPath, verbose, dryRun, full)
	default:
		return fmt.Errorf("unknown command: %s (available: check)", cmd)
	}
}

func runCheck(configPath string, verbose, dryRun, full bool) error {
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
	})
}
