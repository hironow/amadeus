package session

import (
	"context"
	"fmt"
)

// closeReadyIssues finds open issues with the ready label and closes them.
// Called after successful auto-merge to complete the issue lifecycle.
// Idempotent: gh issue close on an already-closed issue is a no-op,
// and ListOpenIssuesByLabel only returns open issues.
func (a *Amadeus) closeReadyIssues(ctx context.Context, readyLabel string) {
	if a.IssueWriter == nil || readyLabel == "" {
		return
	}

	issues, err := a.IssueWriter.ListOpenIssuesByLabel(ctx, readyLabel)
	if err != nil {
		a.Logger.Warn("close-ready: list issues: %v", err)
		return
	}
	if len(issues) == 0 {
		return
	}

	for _, num := range issues {
		comment := fmt.Sprintf("Closed by amadeus: all waves completed (label: %s)", readyLabel)
		if err := a.IssueWriter.CloseIssue(ctx, num, comment); err != nil {
			a.Logger.Warn("close-ready: close issue #%s: %v", num, err)
			continue
		}
		a.Logger.Info("close-ready: closed issue #%s", num)
	}
}
