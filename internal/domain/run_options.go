package domain

import "time"

// RunOptions configures the amadeus run daemon loop.
type RunOptions struct {
	CheckOptions                // embedded check options
	BaseBranch   string        // upstream branch for post-merge checks (empty = none)
	PollInterval time.Duration // inbox scan interval
}

// DefaultPollInterval is the default inbox scan interval.
const DefaultPollInterval = 5 * time.Second
