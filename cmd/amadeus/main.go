package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hironow/amadeus"
	cmd "github.com/hironow/amadeus/internal/cmd"
)

var version = "dev"

func main() {
	shutdown := amadeus.InitTracer("amadeus", version)

	root := cmd.NewRootCommand(version)
	err := root.ExecuteContext(context.Background())
	code := amadeus.ExitCode(err)
	if code == 1 {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	} else if code == 2 {
		fmt.Fprintf(os.Stderr, "drift detected: %v\n", err)
	}
	shutdown(context.Background())
	os.Exit(code)
}
