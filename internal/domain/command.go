package domain

// ExecuteCheckCommand represents the intent to run an amadeus check.
// Independent of cobra — framework concerns are separated at the cmd layer.
// Fields are unexported; use NewExecuteCheckCommand to construct a valid instance.
type ExecuteCheckCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus command family (ExecuteCheckCommand/RunSyncCommand/RebuildCommand/InitCommand/ArchivePruneCommand) is a cohesive command schema; splitting would fragment the command contract [permanent]
	repoPath RepoPath
}

// NewExecuteCheckCommand creates an ExecuteCheckCommand from validated primitives.
func NewExecuteCheckCommand(repoPath RepoPath) ExecuteCheckCommand {
	return ExecuteCheckCommand{repoPath: repoPath}
}

// RepoPath returns the validated repository path.
func (c ExecuteCheckCommand) RepoPath() RepoPath { return c.repoPath }

// RunSyncCommand represents the intent to synchronize PR comments and state.
// Fields are unexported; use NewRunSyncCommand to construct a valid instance.
type RunSyncCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus command family cohesive set; see ExecuteCheckCommand [permanent]
	repoPath RepoPath
}

// NewRunSyncCommand creates a RunSyncCommand from validated primitives.
func NewRunSyncCommand(repoPath RepoPath) RunSyncCommand {
	return RunSyncCommand{repoPath: repoPath}
}

// RepoPath returns the validated repository path.
func (c RunSyncCommand) RepoPath() RepoPath { return c.repoPath }

// RebuildCommand represents the intent to rebuild amadeus state from events.
// Fields are unexported; use NewRebuildCommand to construct a valid instance.
type RebuildCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus command family cohesive set; see ExecuteCheckCommand [permanent]
	repoPath RepoPath
}

// NewRebuildCommand creates a RebuildCommand from validated primitives.
func NewRebuildCommand(repoPath RepoPath) RebuildCommand {
	return RebuildCommand{repoPath: repoPath}
}

// RepoPath returns the validated repository path.
func (c RebuildCommand) RepoPath() RepoPath { return c.repoPath }

// InitCommand represents the intent to initialize a .gate directory.
// Fields are unexported; use NewInitCommand to construct a valid instance.
type InitCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus command family cohesive set; see ExecuteCheckCommand [permanent]
	repoRoot RepoPath
	lang     string
}

// NewInitCommand creates an InitCommand from validated primitives.
func NewInitCommand(repoRoot RepoPath, lang string) InitCommand {
	return InitCommand{repoRoot: repoRoot, lang: lang}
}

// RepoRoot returns the validated repository root path.
func (c InitCommand) RepoRoot() RepoPath { return c.repoRoot }

// Lang returns the language override (empty means use default/existing).
func (c InitCommand) Lang() string { return c.lang }

// ArchivePruneCommand represents the intent to prune old archive files.
// Fields are unexported; use NewArchivePruneCommand to construct a valid instance.
type ArchivePruneCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus command family cohesive set; see ExecuteCheckCommand [permanent]
	repoPath RepoPath
	days     Days
	dryRun   bool
	yes      bool
}

// NewArchivePruneCommand creates an ArchivePruneCommand from validated primitives.
func NewArchivePruneCommand(repoPath RepoPath, days Days, dryRun, yes bool) ArchivePruneCommand {
	return ArchivePruneCommand{repoPath: repoPath, days: days, dryRun: dryRun, yes: yes}
}

// RepoPath returns the validated repository path.
func (c ArchivePruneCommand) RepoPath() RepoPath { return c.repoPath }

// Days returns the validated retention day count.
func (c ArchivePruneCommand) Days() Days { return c.days }

// DryRun returns whether to perform a dry run.
func (c ArchivePruneCommand) DryRun() bool { return c.dryRun }

// Yes returns whether to skip confirmation.
func (c ArchivePruneCommand) Yes() bool { return c.yes }

// ExecuteRunCommand represents the intent to run the amadeus daemon loop.
// Fields are unexported; use NewExecuteRunCommand to construct a valid instance.
type ExecuteRunCommand struct {
	repoPath   RepoPath
	baseBranch string
}

// NewExecuteRunCommand creates an ExecuteRunCommand from validated primitives.
func NewExecuteRunCommand(repoPath RepoPath, baseBranch string) ExecuteRunCommand {
	return ExecuteRunCommand{repoPath: repoPath, baseBranch: baseBranch}
}

// RepoPath returns the validated repository path.
func (c ExecuteRunCommand) RepoPath() RepoPath { return c.repoPath }

// BaseBranch returns the upstream branch for post-merge checks (empty = none).
func (c ExecuteRunCommand) BaseBranch() string { return c.baseBranch }
