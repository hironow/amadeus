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
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()

	rootCmd := cmd.NewRootCommand()
	args := os.Args[1:]
	if cmd.NeedsDefaultRun(rootCmd, args) {
		args = append([]string{"run"}, args...)
	}
	rootCmd.SetArgs(args)

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		var silent *domain.SilentError
		if !errors.As(err, &silent) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return domain.ExitCode(err)
}
