package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	cmd "github.com/hironow/amadeus/internal/cmd"
	"github.com/hironow/amadeus/internal/domain"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer cancel()

	root := cmd.NewRootCommand()
	// NOTE: No NormalizeArgs — single-dash long flags (e.g. -config) are intentionally
	// unsupported per MY-334 POSIX-compliant flags policy. Use --config or -c instead.
	args := os.Args[1:]
	if cmd.NeedsDefaultRun(root, args) {
		args = append([]string{"run"}, args...)
	}
	root.SetArgs(args)

	err := root.ExecuteContext(ctx)
	if err != nil {
		var silent *domain.SilentError
		if !errors.As(err, &silent) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return domain.ExitCode(err)
}
