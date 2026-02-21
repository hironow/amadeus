package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hironow/amadeus"
	cmd "github.com/hironow/amadeus/internal/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	shutdown := amadeus.InitTracer("amadeus", version)
	defer shutdown(context.Background())

	info := cmd.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
	root := cmd.NewRootCommand(info)
	root.SetArgs(cmd.NormalizeArgs(root, os.Args[1:]))
	err := root.ExecuteContext(context.Background())
	code := amadeus.ExitCode(err)
	if code == 1 {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	} else if code == 2 {
		fmt.Fprintf(os.Stderr, "drift detected: %v\n", err)
	}
	return code
}
