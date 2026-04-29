package domain

import "fmt"

// TrackingMode determines the issue tracking backend.
// Wave mode (default) uses D-Mail archive as event source.
// Linear mode uses Linear MCP for issue tracking (legacy).
type TrackingMode string

const (
	// ModeWave is the default mode: waves and steps drive expedition targeting.
	// D-Mail archive/ is the single source of truth for wave state.
	ModeWave TrackingMode = "wave"

	// ModeLinear uses Linear MCP for issue tracking (existing behavior).
	ModeLinear TrackingMode = "linear"
)

// NewTrackingMode returns ModeLinear when linear is true, ModeWave otherwise.
func NewTrackingMode(linear bool) TrackingMode {
	if linear {
		return ModeLinear
	}
	return ModeWave
}

// IsLinear returns true when operating in Linear MCP mode.
func (m TrackingMode) IsLinear() bool { return m == ModeLinear }

// IsWave returns true when operating in Wave-centric mode.
func (m TrackingMode) IsWave() bool { return m == ModeWave }

// String returns the mode name.
func (m TrackingMode) String() string { return string(m) }

// RepoPath is an always-valid non-empty repository path.
type RepoPath struct{ v string } // nosemgrep: structure.multiple-exported-structs-go -- primitives family (RepoPath/Days) is a cohesive domain value object set [permanent]

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
