package session

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// attemptAutoMerge discovers merge-ready PRs and merges them in dependency order.
// It is called after a successful post-merge check (no DriftError) when auto-merge is enabled.
func (a *Amadeus) attemptAutoMerge(ctx context.Context, integrationBranch string) {
	if a.PRReader == nil || a.PRWriter == nil {
		return
	}

	// 1. List ALL open PRs (not filtered by base branch).
	// We need the full set to correctly build dependency chains.
	// ListOpenPRs with "" returns all open PRs regardless of base branch.
	prs, err := a.PRReader.ListOpenPRs(ctx, "")
	if err != nil {
		a.Logger.Warn("auto-merge: list PRs: %v", err)
		return
	}
	if len(prs) == 0 {
		return
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
	for _, orphan := range report.OrphanedPRs {
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
}

// tryMergePR attempts to merge a single PR. Returns true on success.
func (a *Amadeus) tryMergePR(ctx context.Context, mc *mergeCandidate) bool {
	now := time.Now().UTC()

	if !mc.readiness.Ready {
		a.Logger.Info("auto-merge: skip %s (%s) — %v", mc.pr.Number(), mc.pr.Title(), mc.readiness.BlockReasons)
		if a.Emitter != nil {
			_ = a.emitMergeSkipped(mc.pr, mc.readiness.BlockReasons, now)
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
	return true
}

func (a *Amadeus) emitMerged(pr domain.PRState, method domain.MergeMethod, now time.Time) error {
	return a.Emitter.EmitPRMerged(domain.PRMergedData{
		PRNumber: pr.Number(),
		Title:    pr.Title(),
		Method:   string(method),
	}, now)
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
