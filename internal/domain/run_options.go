package domain

// RunOptions configures the amadeus run daemon loop.
type RunOptions struct {
	CheckOptions        // embedded check options
	BaseBranch   string // upstream branch for post-merge checks (empty = none)
	AutoMerge    bool   // auto-merge eligible PRs when no drift detected (default: true when BaseBranch is set)
	ReadyLabel   string // issue label that signals "ready to close" (default: "sightjack:ready")
}
