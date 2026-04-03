package session

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// attemptAutoMerge discovers merge-ready PRs and merges them in dependency order.
// It is called after a successful post-merge check (no DriftError) when auto-merge is enabled.
// Returns the number of PRs successfully merged.
func (a *Amadeus) attemptAutoMerge(ctx context.Context, integrationBranch string) int {
	if a.PRReader == nil || a.PRWriter == nil {
		return 0
	}

	// 1. List ALL open PRs (not filtered by base branch).
	// We need the full set to correctly build dependency chains.
	// ListOpenPRs with "" returns all open PRs regardless of base branch.
	prs, err := a.PRReader.ListOpenPRs(ctx, "")
	if err != nil {
		a.Logger.Warn("auto-merge: list PRs: %v", err)
		return 0
	}
	if len(prs) == 0 {
		return 0
	}

	// 2. Build chain structure for merge order and strategy
	report := domain.BuildPRConvergenceReport(integrationBranch, prs)

	// 3. Build a map of PR number -> chain for merge method determination
	prChainMap := buildPRChainMap(report)

	// 4. Evaluate merge readiness for each PR
	var candidates []mergeCandidate
	for _, pr := range prs {
		readiness, err := a.PRReader.GetPRMergeReadiness(ctx, pr.Number())
		if err != nil {
			a.Logger.Warn("auto-merge: readiness check for %s: %v", pr.Number(), err)
			continue
		}
		chain := prChainMap[pr.Number()]
		method := domain.DetermineMergeMethod(pr, chain)
		candidates = append(candidates, mergeCandidate{
			pr:        pr,
			readiness: *readiness,
			method:    method,
			chain:     chain,
		})
	}

	// 5. Merge in chain order: chains first (root→leaf), then orphans
	merged := 0
	for _, chain := range report.Chains {
		for _, chainPR := range chain.PRs {
			mc := findCandidate(candidates, chainPR.Number())
			if mc == nil {
				continue
			}
			if a.tryMergePR(ctx, mc) {
				merged++
			}
		}
	}
	// Pre-fetch sightjack:ready issue numbers once for issue-link warnings.
	var sightjackIssues []string
	if len(report.OrphanedPRs) > 0 && a.IssueWriter != nil {
		var issueErr error
		sightjackIssues, issueErr = a.IssueWriter.ListOpenIssuesByLabel(ctx, "sightjack:ready")
		if issueErr != nil {
			a.Logger.Debug("auto-merge: list sightjack:ready issues: %v", issueErr)
		}
	}

	for _, orphan := range report.OrphanedPRs {
		// Pipeline-generated orphans (wave/expedition/amadeus branches or
		// paintress labels) have a stale base branch — close them so the
		// pipeline can re-create from the correct base.
		if domain.IsPipelinePR(orphan) {
			a.closePipelineOrphan(ctx, orphan)
			continue
		}
		// Issue-link-only match: warn but do NOT close.
		// Closing based solely on issue reference risks false positives
		// for release/hotfix PRs that happen to mention the same issue.
		if !domain.IsPipelinePR(orphan) && domain.IsPipelinePRWithIssueContext(orphan, sightjackIssues) {
			a.Logger.Warn("auto-merge: orphan %s (%s) references a sightjack:ready issue but lacks pipeline branch/label — skipping auto-close (manual review recommended)",
				orphan.Number(), orphan.Title())
		}
		mc := findCandidate(candidates, orphan.Number())
		if mc == nil {
			continue
		}
		if a.tryMergePR(ctx, mc) {
			merged++
		}
	}

	if merged > 0 {
		a.Logger.OK("auto-merge: merged %d PR(s)", merged)
	}
	return merged
}

// closePipelineOrphan closes an orphaned pipeline PR whose base branch
// has been merged. Logs a banner and emits a merge-skipped event.
func (a *Amadeus) closePipelineOrphan(ctx context.Context, pr domain.PRState) {
	comment := fmt.Sprintf("Closed by amadeus: this PR targets branch `%s` which is no longer the integration branch. "+
		"The pipeline will re-create a PR from the correct base if the linked issue is still open.", pr.BaseBranch())

	a.Logger.Warn("auto-merge: closing pipeline orphan %s (%s) — base %s is stale",
		pr.Number(), pr.Title(), pr.BaseBranch())

	if a.PRWriter != nil {
		if err := a.PRWriter.ClosePR(ctx, pr.Number(), comment); err != nil {
			a.Logger.Warn("auto-merge: close pipeline orphan %s: %v", pr.Number(), err)
		}
	}

	if a.Emitter != nil {
		now := time.Now().UTC()
		_ = a.emitMergeSkipped(pr, []string{"pipeline orphan: base branch " + pr.BaseBranch() + " is stale"}, now)
	}
}

// tryMergePR attempts to merge a single PR. Returns true on success.
// For CONFLICTING PRs, generates a D-Mail to paintress for conflict resolution.
func (a *Amadeus) tryMergePR(ctx context.Context, mc *mergeCandidate) bool {
	now := time.Now().UTC()

	if !mc.readiness.Ready {
		a.Logger.Info("auto-merge: skip %s (%s) — %v", mc.pr.Number(), mc.pr.Title(), mc.readiness.BlockReasons)
		if a.Emitter != nil {
			_ = a.emitMergeSkipped(mc.pr, mc.readiness.BlockReasons, now)
		}
		// Generate D-Mail for conflicting PRs so paintress can fix them
		if mc.readiness.Mergeable == "CONFLICTING" && a.Emitter != nil {
			a.emitConflictDMail(mc.pr, now)
		}
		return false
	}

	a.Logger.Warn("auto-merge: merging %s (%s) via %s", mc.pr.Number(), mc.pr.Title(), mc.method)

	if err := a.PRWriter.MergePR(ctx, mc.pr.Number(), mc.method); err != nil {
		a.Logger.Warn("auto-merge: merge failed for %s: %v", mc.pr.Number(), err)
		if a.Emitter != nil {
			_ = a.emitMergeSkipped(mc.pr, []string{err.Error()}, now)
		}
		return false
	}

	domain.LogBanner(a.Logger, domain.BannerSend, "merge", mc.pr.Number(), mc.pr.Title())
	if a.Emitter != nil {
		_ = a.emitMerged(mc.pr, mc.method, now)
	}

	// Remove review label from merged PR (cleanup)
	if a.PRWriter != nil {
		_ = a.PRWriter.RemoveLabel(ctx, mc.pr.Number(), PRReviewLabel)
	}
	return true
}

func (a *Amadeus) emitMerged(pr domain.PRState, method domain.MergeMethod, now time.Time) error {
	return a.Emitter.EmitPRMerged(domain.PRMergedData{
		PRNumber: pr.Number(),
		Title:    pr.Title(),
		Method:   string(method),
	}, now)
}

// emitConflictDMail generates a KindImplFeedback D-Mail for a conflicting PR.
// The D-Mail is routed to paintress via the outbox → phonewave path.
func (a *Amadeus) emitConflictDMail(pr domain.PRState, now time.Time) {
	name := fmt.Sprintf("am-conflict-%s-%s", pr.Number(), pr.HeadSHAShort())
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          name,
		Kind:          domain.KindImplFeedback,
		Description:   fmt.Sprintf("PR %s has merge conflicts — rebase needed", pr.Number()),
		Severity:      domain.SeverityMedium,
		Action:        domain.ActionRetry,
		Targets:       []string{"paintress"},
		Metadata: map[string]string{
			"pr_number":       pr.Number(),
			"pr_title":        pr.Title(),
			"conflict_reason": "CONFLICTING",
			"type":            "merge-conflict",
			"created_at":      now.Format(time.RFC3339),
		},
		Body: fmt.Sprintf("PR %s (%s) has merge conflicts with the base branch and cannot be merged automatically.\n\nAction needed: rebase this PR against %s and resolve conflicts.", pr.Number(), pr.Title(), pr.BaseBranch()),
	}
	if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
		a.Logger.Warn("auto-merge: invalid conflict D-Mail for %s: %v", pr.Number(), errs)
		return
	}
	domain.LogBanner(a.Logger, domain.BannerSend, string(dmail.Kind), dmail.Name, dmail.Description)
	if err := a.Emitter.EmitDMailGenerated(dmail, now); err != nil {
		a.Logger.Warn("auto-merge: emit conflict D-Mail for %s: %v", pr.Number(), err)
	}
}

func (a *Amadeus) emitMergeSkipped(pr domain.PRState, reasons []string, now time.Time) error {
	return a.Emitter.EmitPRMergeSkipped(domain.PRMergeSkippedData{
		PRNumber: pr.Number(),
		Title:    pr.Title(),
		Reasons:  reasons,
	}, now)
}

// mergeCandidate holds a PR with its readiness evaluation and merge strategy.
type mergeCandidate struct {
	pr        domain.PRState
	readiness domain.PRMergeReadiness
	method    domain.MergeMethod
	chain     *domain.PRChain
}

// buildPRChainMap returns a map from PR number to its containing chain.
func buildPRChainMap(report domain.PRConvergenceReport) map[string]*domain.PRChain {
	m := make(map[string]*domain.PRChain)
	for i := range report.Chains {
		chain := &report.Chains[i]
		for _, pr := range chain.PRs {
			m[pr.Number()] = chain
		}
	}
	return m
}

// findCandidate finds a mergeCandidate by PR number.
func findCandidate(candidates []mergeCandidate, number string) *mergeCandidate {
	for i := range candidates {
		if candidates[i].pr.Number() == number {
			return &candidates[i]
		}
	}
	return nil
}
