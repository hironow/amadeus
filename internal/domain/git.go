package domain

// MergedPR represents a merged pull request.
type MergedPR struct {
	Number string
	Title  string
}

// Git is the port interface for repository version control operations.
type Git interface {
	// CurrentCommit returns the short SHA of the current HEAD.
	CurrentCommit() (string, error)

	// MergedPRsSince returns merged PRs between the given commit and HEAD.
	MergedPRsSince(since string) ([]MergedPR, error)

	// DiffSince returns the unified diff between the given commit and HEAD.
	DiffSince(since string) (string, error)
}
