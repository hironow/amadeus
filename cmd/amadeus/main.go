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
	// Two-context pattern for graceful shutdown with handover.
	// 1st signal: cancel workCtx → interrupt active work, write handover.
	// 2nd signal: cancel outerCtx → abort handover, let defers run.
	outerCtx, outerCancel := context.WithCancel(context.Background())
	defer outerCancel()

	workCtx, workCancel := context.WithCancel(outerCtx)
	defer workCancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, shutdownSignals...)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh        // 1st signal: cancel work
		workCancel()
		<-sigCh        // 2nd signal: cancel outer (NO os.Exit!)
		outerCancel()
	}()

	// Embed outerCtx in workCtx so commands can retrieve it for handover writing.
	workCtx = context.WithValue(workCtx, domain.ShutdownKey, outerCtx)

	rootCmd := cmd.NewRootCommand()
	args := os.Args[1:]
	if cmd.NeedsDefaultRun(rootCmd, args) {
		args = append([]string{"run"}, args...)
	}
	rootCmd.SetArgs(args)

	err := rootCmd.ExecuteContext(workCtx)

	// Signal-induced context cancellation is not an application error.
	// Exit with 128+SIGINT=130 per UNIX convention instead of printing
	// "error: context canceled" and exiting with code 1.
	if err != nil && errors.Is(err, context.Canceled) && workCtx.Err() != nil {
		return 130
	}

	if err != nil {
		var silent *domain.SilentError
		if !errors.As(err, &silent) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	return domain.ExitCode(err)
}
