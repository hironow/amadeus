package domain

import (
	"fmt"
	"strings"
)

// PRState represents an open PR's convergence-relevant state.
// All fields are unexported; use NewPRState to construct with validation.
type PRState struct {
	number        string
	title         string
	baseBranch    string
	headBranch    string
	mergeable     bool
	behindBy      int
	conflictFiles []string
	labels        []string
	headSHA       string
}

// NewPRState creates a validated PRState. Returns an error if required fields
// (number, baseBranch, headBranch) are empty.
func NewPRState(number, title, baseBranch, headBranch string, mergeable bool, behindBy int, conflictFiles, labels []string, headSHA string) (PRState, error) { // nosemgrep: domain-primitives.multiple-string-params-go — number/title/baseBranch/headBranch/headSHA are semantically distinct PR identity fields [permanent]
	if number == "" {
		return PRState{}, fmt.Errorf("PRState number is required")
	}
	if baseBranch == "" {
		return PRState{}, fmt.Errorf("PRState baseBranch is required")
	}
	if headBranch == "" {
		return PRState{}, fmt.Errorf("PRState headBranch is required")
	}
	// Defensive copy of conflictFiles to prevent mutation.
	var files []string
	if len(conflictFiles) > 0 {
		files = make([]string, len(conflictFiles))
		copy(files, conflictFiles)
	}
	// Defensive copy of labels.
	var lbls []string
	if len(labels) > 0 {
		lbls = make([]string, len(labels))
		copy(lbls, labels)
	}
	return PRState{
		number:        number,
		title:         title,
		baseBranch:    baseBranch,
		headBranch:    headBranch,
		mergeable:     mergeable,
		behindBy:      behindBy,
		conflictFiles: files,
		labels:        lbls,
		headSHA:       headSHA,
	}, nil
}

// Number returns the PR number (e.g. "#42").
func (p PRState) Number() string { return p.number }

// Title returns the PR title.
func (p PRState) Title() string { return p.title }

// BaseBranch returns the base branch of the PR.
func (p PRState) BaseBranch() string { return p.baseBranch }

// HeadBranch returns the head branch of the PR.
func (p PRState) HeadBranch() string { return p.headBranch }

// Mergeable returns whether the PR is currently mergeable.
func (p PRState) Mergeable() bool { return p.mergeable }

// BehindBy returns how many commits the PR is behind the base branch.
func (p PRState) BehindBy() int { return p.behindBy }

// ConflictFiles returns the list of files with merge conflicts.
func (p PRState) ConflictFiles() []string { return p.conflictFiles }

// HasConflict reports whether the PR has any merge conflict files.
func (p PRState) HasConflict() bool { return len(p.conflictFiles) > 0 }

// Labels returns the labels applied to the PR.
func (p PRState) Labels() []string { return p.labels }

// HasLabel reports whether the PR has the given label (exact match).
func (p PRState) HasLabel(label string) bool {
	for _, l := range p.labels {
		if l == label {
			return true
		}
	}
	return false
}

// HasLabelPrefix reports whether the PR has any label starting with the given prefix.
func (p PRState) HasLabelPrefix(prefix string) bool {
	for _, l := range p.labels {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// HeadSHA returns the full HEAD commit SHA of the PR.
func (p PRState) HeadSHA() string { return p.headSHA }

// HeadSHAShort returns the first 8 characters of the HEAD commit SHA.
func (p PRState) HeadSHAShort() string {
	if len(p.headSHA) >= 8 {
		return p.headSHA[:8]
	}
	return p.headSHA
}

// PRChain represents a dependency chain of PRs ordered root to leaf.
type PRChain struct {
	ID          string
	PRs         []PRState
	HasConflict bool
}

// PRConvergenceReport is the result of pre-merge convergence analysis.
type PRConvergenceReport struct {
	IntegrationBranch string
	Chains            []PRChain
	OrphanedPRs       []PRState
	TotalOpenPRs      int
}

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

