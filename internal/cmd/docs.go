package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newDocsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate CLI documentation in Markdown",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, _ := cmd.Flags().GetString("output")
			if output == "" {
				return fmt.Errorf("--output is required")
			}

			if err := os.MkdirAll(output, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}

			root := cmd.Root()
			root.DisableAutoGenTag = true
			return doc.GenMarkdownTree(root, output)
		},
	}

	cmd.Flags().StringP("output", "o", "", "output directory for generated docs")

	return cmd
}
