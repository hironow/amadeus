// Command docgen generates markdown CLI documentation from cobra commands.
// Output goes to docs/cli/ for LLM consumption and llms.txt aggregation.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	cmd "github.com/hironow/amadeus/internal/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	outDir := filepath.Join("docs", "cli")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	rootCmd := cmd.NewRootCommand(cmd.BuildInfo{Version: "dev"})
	rootCmd.DisableAutoGenTag = true

	if err := doc.GenMarkdownTree(rootCmd, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "docgen: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "docs generated in %s/\n", outDir)
}
