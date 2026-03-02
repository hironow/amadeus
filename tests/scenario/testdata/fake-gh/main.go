// fake-gh is a minimal test double for the GitHub CLI.
//
// It handles the subset of gh commands used by the 4-tool ecosystem:
//   - gh pr create → prints a fake PR URL to stdout.
//   - gh pr view   → prints minimal JSON to stdout.
//
// All other invocations exit 0 silently.
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		os.Exit(0)
	}

	sub := args[0] + " " + args[1] // e.g. "pr create"

	switch {
	case strings.HasPrefix(sub, "pr create"):
		fmt.Println("https://github.com/test/repo/pull/42")
	case strings.HasPrefix(sub, "pr view"):
		fmt.Println(`{"number":42,"state":"open","url":"https://github.com/test/repo/pull/42"}`)
	}
}
