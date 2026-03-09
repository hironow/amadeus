package main

import (
	"context"
	"fmt"
	"os"

	cmd "github.com/hironow/amadeus/internal/cmd"
	"github.com/hironow/amadeus/internal/domain"
)

func main() {
	os.Exit(run())
}

func run() int {
	root := cmd.NewRootCommand()
	// NOTE: No NormalizeArgs — single-dash long flags (e.g. -config) are intentionally
	// unsupported per MY-334 POSIX-compliant flags policy. Use --config or -c instead.
	args := os.Args[1:]
	if cmd.NeedsDefaultRun(root, args) {
		args = append([]string{"run"}, args...)
	}
	root.SetArgs(args)

	err := root.ExecuteContext(context.Background())
	code := domain.ExitCode(err)
	if code == 1 {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	} else if code == 2 {
		fmt.Fprintf(os.Stderr, "drift detected: %v\n", err)
	}
	return code
}
