package testdata

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// === ERROR severity rules ===

var cmdBadContext = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: cobra-no-context-background-in-cmd
		ctx := context.Background()
		_ = ctx
		return nil
	},
}

var cmdBadExit = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: cobra-no-os-exit-in-cmd
		os.Exit(1)
		return nil
	},
}

// === WARNING severity rules ===

var cmdBadFmtPrint = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: cobra-no-fmt-print-in-cmd
		fmt.Println("hello")
		return nil
	},
}

var cmdBadStdout = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: cobra-no-os-stdout-in-cmd
		_ = os.Stdout
		return nil
	},
}

var cmdBadStderr = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: cobra-no-os-stderr-in-cmd
		_ = os.Stderr
		return nil
	},
}

var cmdBadStdin = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: cobra-no-os-stdin-in-cmd
		_ = os.Stdin
		return nil
	},
}

func badTraverseHooks() {
	// ruleid: cobra-traverse-hooks-in-init
	cobra.EnableTraverseRunHooks = true
}

func badOnFinalize() {
	// ruleid: cobra-onfinalize-needs-once
	cobra.OnFinalize(func() {})
}
