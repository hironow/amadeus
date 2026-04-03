package policy

import (
	"github.com/hironow/amadeus/internal/domain"
)

// EvaluateMergeReadiness evaluates whether a PR is ready to merge.
// reviewDecision "" means no reviewers assigned (treated as OK).
func EvaluateMergeReadiness(number, mergeStateStatus, reviewDecision, mergeable string, hasReviewLabel bool) domain.PRMergeReadiness {
	r := domain.PRMergeReadiness{
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
func FilterMergeReady(readiness []domain.PRMergeReadiness) []domain.PRMergeReadiness {
	var ready []domain.PRMergeReadiness
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
func DetermineMergeMethod(pr domain.PRState, chain *domain.PRChain) domain.MergeMethod {
	if chain == nil || len(chain.PRs) <= 1 {
		return domain.MergeMethodSquash
	}

	for _, other := range chain.PRs {
		if other.BaseBranch() == pr.HeadBranch() && other.Number() != pr.Number() {
			return domain.MergeMethodMerge
		}
	}

	return domain.MergeMethodSquash
}
