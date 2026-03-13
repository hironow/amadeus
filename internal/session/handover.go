package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// FileHandoverWriter writes a handover markdown document to the state directory.
type FileHandoverWriter struct{}

// WriteHandover renders state to markdown and writes it to {stateDir}/handover.md.
// The file is always overwritten (latest only). Respects context cancellation.
func (w *FileHandoverWriter) WriteHandover(ctx context.Context, stateDir string, state domain.HandoverState) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("handover aborted: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Handover - %s\n\n", state.Timestamp.UTC().Format("2006-01-02T15:04:05Z"))
	b.WriteString("## Status: INTERRUPTED\n\n")

	if state.InProgress != "" {
		b.WriteString("## In Progress\n")
		fmt.Fprintf(&b, "- %s\n\n", state.InProgress)
	}

	if len(state.Completed) > 0 {
		b.WriteString("## Completed\n")
		for _, item := range state.Completed {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		b.WriteString("\n")
	}

	if len(state.Remaining) > 0 {
		b.WriteString("## Remaining\n")
		for _, item := range state.Remaining {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		b.WriteString("\n")
	}

	if len(state.PartialState) > 0 {
		b.WriteString("## Partial State\n")
		for k, v := range state.PartialState {
			fmt.Fprintf(&b, "- %s: %s\n", k, v)
		}
		b.WriteString("\n")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("handover aborted: %w", err)
	}

	return os.WriteFile(filepath.Join(stateDir, "handover.md"), []byte(b.String()), 0o644)
}
