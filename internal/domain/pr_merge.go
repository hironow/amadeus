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

