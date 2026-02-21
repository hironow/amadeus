package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hironow/amadeus"
	cmd "github.com/hironow/amadeus/internal/cmd"
)

func main() {
	os.Exit(run())
}

func run() int {
	shutdown := amadeus.InitTracer("amadeus", cmd.Version)
	defer shutdown(context.Background())

	root := cmd.NewRootCommand()
	// NOTE: No NormalizeArgs — single-dash long flags (e.g. -config) are intentionally
	// unsupported per MY-334 POSIX-compliant flags policy. Use --config or -c instead.
	err := root.ExecuteContext(context.Background())
	code := amadeus.ExitCode(err)
	if code == 1 {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	} else if code == 2 {
		fmt.Fprintf(os.Stderr, "drift detected: %v\n", err)
	}
	return code
}
