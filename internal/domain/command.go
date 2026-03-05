package domain

import "fmt"

// ExecuteCheckCommand represents the intent to run an amadeus check.
// Independent of cobra — framework concerns are separated at the cmd layer.
type ExecuteCheckCommand struct {
	RepoPath string
}

// Validate checks that the command has valid required fields.
func (c *ExecuteCheckCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}

// RunSyncCommand represents the intent to synchronize PR comments and state.
type RunSyncCommand struct {
	RepoPath string
}

// Validate checks that the command has valid required fields.
func (c *RunSyncCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}

// RebuildCommand represents the intent to rebuild amadeus state from events.
type RebuildCommand struct {
	RepoPath string
}

// Validate checks that the command has valid required fields.
func (c *RebuildCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}

// InitCommand represents the intent to initialize a .gate directory.
type InitCommand struct {
	RepoRoot string
}

// Validate checks that the command has valid required fields.
func (c *InitCommand) Validate() []error {
	var errs []error
	if c.RepoRoot == "" {
		errs = append(errs, fmt.Errorf("RepoRoot is required"))
	}
	return errs
}

// ArchivePruneCommand represents the intent to prune old archive files.
type ArchivePruneCommand struct {
	RepoPath string
	Days     int
	DryRun   bool
	Yes      bool
}

// Validate checks that the command has valid required fields.
func (c *ArchivePruneCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	if c.Days <= 0 {
		errs = append(errs, fmt.Errorf("Days must be positive"))
	}
	return errs
}
