package domain

// RunOptions configures the amadeus run daemon loop.
type RunOptions struct {
	CheckOptions              // embedded check options
	BaseBranch   string       // upstream branch for post-merge checks (empty = none)
}
