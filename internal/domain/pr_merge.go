package domain

// MergeMethod represents the git merge strategy for a PR.
type MergeMethod string

const (
	// MergeMethodSquash squashes all commits into one (clean history).
	MergeMethodSquash MergeMethod = "squash"
	// MergeMethodMerge preserves commit hashes (for chain PRs with dependents).
	MergeMethodMerge MergeMethod = "merge"
)

// PRMergeReadiness holds the merge readiness evaluation for a single PR.
type PRMergeReadiness struct {
	Number           string
	MergeStateStatus string   // "CLEAN", "BLOCKED", "BEHIND", "DIRTY", "UNSTABLE"
	ReviewDecision   string   // "APPROVED", "REVIEW_REQUIRED", "CHANGES_REQUESTED", ""
	Mergeable        string   // "MERGEABLE", "CONFLICTING", "UNKNOWN"
	HasReviewLabel   bool     // amadeus:reviewed-* label exists
	Ready            bool     // all conditions met
	BlockReasons     []string // why not ready
}

// EvaluateMergeReadiness evaluates whether a PR is ready to merge.
// reviewDecision "" means no reviewers assigned (treated as OK).
func EvaluateMergeReadiness(number, mergeStateStatus, reviewDecision, mergeable string, hasReviewLabel bool) PRMergeReadiness {
	r := PRMergeReadiness{
		Number:           number,
		MergeStateStatus: mergeStateStatus,
		ReviewDecision:   reviewDecision,
		Mergeable:        mergeable,
		HasReviewLabel:   hasReviewLabel,
		Ready:            true,
	}

	if mergeStateStatus != "CLEAN" {
		r.Ready = false
		r.BlockReasons = append(r.BlockReasons, "merge state: "+mergeStateStatus)
	}

	if reviewDecision != "" && reviewDecision != "APPROVED" {
		r.Ready = false
		r.BlockReasons = append(r.BlockReasons, "review: "+reviewDecision)
	}

	if mergeable != "MERGEABLE" {
		r.Ready = false
		r.BlockReasons = append(r.BlockReasons, "mergeable: "+mergeable)
	}

	if !hasReviewLabel {
		r.Ready = false
		r.BlockReasons = append(r.BlockReasons, "missing amadeus review label")
	}

	return r
}

// FilterMergeReady returns only the PRs that are ready to merge.
func FilterMergeReady(readiness []PRMergeReadiness) []PRMergeReadiness {
	var ready []PRMergeReadiness
	for _, r := range readiness {
		if r.Ready {
			ready = append(ready, r)
		}
	}
	return ready
}

// DetermineMergeMethod returns the merge strategy for a PR based on chain position.
// Chain root/middle (has dependents after it): merge (preserve hash).
// Chain leaf, standalone, or nil chain: squash (clean history).
func DetermineMergeMethod(pr PRState, chain *PRChain) MergeMethod {
	if chain == nil || len(chain.PRs) <= 1 {
		return MergeMethodSquash
	}

	// Check if this PR has any dependents in the chain.
	// A PR has dependents if any subsequent PR in the chain uses this PR's
	// headBranch as its baseBranch.
	for _, other := range chain.PRs {
		if other.BaseBranch() == pr.HeadBranch() && other.Number() != pr.Number() {
			return MergeMethodMerge
		}
	}

	return MergeMethodSquash
}
