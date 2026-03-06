package domain

import "fmt"

// RepoPath is an always-valid non-empty repository path.
type RepoPath struct{ v string }

// NewRepoPath parses a raw string into a RepoPath.
// Returns an error if the path is empty.
func NewRepoPath(raw string) (RepoPath, error) {
	if raw == "" {
		return RepoPath{}, fmt.Errorf("RepoPath is required")
	}
	return RepoPath{v: raw}, nil
}

// String returns the underlying path string.
func (r RepoPath) String() string { return r.v }

// Days is an always-valid positive integer representing a day count.
type Days struct{ v int }

// NewDays parses a raw integer into a Days value.
// Returns an error if the value is not positive.
func NewDays(raw int) (Days, error) {
	if raw <= 0 {
		return Days{}, fmt.Errorf("Days must be positive")
	}
	return Days{v: raw}, nil
}

// Int returns the underlying day count.
func (d Days) Int() int { return d.v }
