package port

import (
	"context"
	"errors"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

// InitOption configures optional behavior for project initialization.
type InitOption func(*InitConfig)

// InitConfig holds per-invocation configuration for project initialization.
// Tools use only the fields relevant to their init flow.
type InitConfig struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- port contract family (InitConfig/Approver/Notifier/PolicyMetrics/PruneCandidate/NopImprovementTaskDispatcher + interfaces) is a cohesive set for the amadeus usecase boundary; splitting would fragment the port API surface [permanent]
	Team       string
	Project    string
	Lang       string
	Strictness string
}

// ApplyInitOptions applies InitOption functions to an InitConfig and returns it.
func ApplyInitOptions(opts ...InitOption) InitConfig {
	var c InitConfig
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// WithTeam sets the team identifier for project initialization.
func WithTeam(t string) InitOption { return func(c *InitConfig) { c.Team = t } }

// WithProject sets the project name for initialization.
func WithProject(p string) InitOption { return func(c *InitConfig) { c.Project = p } }

// WithLang sets the language for initialization (e.g. "ja", "en").
func WithLang(l string) InitOption { return func(c *InitConfig) { c.Lang = l } }

// WithStrictness sets the strictness level (e.g. "fog", "alert", "lockdown").
func WithStrictness(s string) InitOption { return func(c *InitConfig) { c.Strictness = s } }

// InitRunner handles project initialization I/O.
// Returns warnings for non-fatal issues (nil when none). Error for critical failures.
type InitRunner interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	InitProject(baseDir string, opts ...InitOption) (warnings []string, err error)
}

// EventDispatcher dispatches domain events to policy handlers.
// Implemented by usecase.PolicyEngine; injected into session via Amadeus struct.
type EventDispatcher interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	Dispatch(ctx context.Context, event domain.Event) error
}

// Approver determines whether an action should proceed.
// Implementations include StdinApprover (human prompt),
// CmdApprover (external command), and AutoApprover (always yes).
type Approver interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

// AutoApprover always approves without human interaction.
type AutoApprover struct{} // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- null-object for Approver; must co-locate with interface definition; port null-object family cohesive set [permanent]

func (*AutoApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// Notifier sends notifications about events.
type Notifier interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	Notify(ctx context.Context, title, message string) error
}

// NopNotifier is a no-op notifier for quiet mode or testing.
type NopNotifier struct{} // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- null-object for Notifier; must co-locate with interface definition; port null-object family cohesive set [permanent]

func (*NopNotifier) Notify(_ context.Context, _, _ string) error { return nil }

// PolicyMetrics records policy handler execution metrics.
type PolicyMetrics interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	RecordPolicyEvent(ctx context.Context, eventType string, status string)
}

// NopPolicyMetrics is a no-op metrics recorder for tests and quiet mode.
type NopPolicyMetrics struct{} // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- null-object for PolicyMetrics; must co-locate with interface definition; port null-object family cohesive set [permanent]

func (*NopPolicyMetrics) RecordPolicyEvent(_ context.Context, _, _ string) {}

// ContextEventApplier extends domain.EventApplier with context propagation.
// domain.EventApplier is ctx-free (pure domain); this port interface adds ctx
// so that session-layer implementations can propagate trace/cancel.
type ContextEventApplier interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	Apply(ctx context.Context, event domain.Event) error
	Rebuild(ctx context.Context, events []domain.Event) error
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// EventStore is the append-only event persistence interface.
type EventStore interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// Append persists one or more events. Validation is performed before any writes.
	Append(ctx context.Context, events ...domain.Event) (domain.AppendResult, error)

	// LoadAll returns all events in chronological order.
	LoadAll(ctx context.Context) ([]domain.Event, domain.LoadResult, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(ctx context.Context, after time.Time) ([]domain.Event, domain.LoadResult, error)

	// LoadAfterSeqNr returns all events with SeqNr > afterSeqNr,
	// ordered by SeqNr ascending. Used for snapshot-based recovery.
	LoadAfterSeqNr(ctx context.Context, afterSeqNr uint64) ([]domain.Event, domain.LoadResult, error)

	// LatestSeqNr returns the highest recorded SeqNr across all events.
	// Returns 0 if no events have a SeqNr assigned.
	LatestSeqNr(ctx context.Context) (uint64, error)
}

// SnapshotStore persists materialized projection state at a known SeqNr.
// Snapshots are an optimization — the system must function without them
// (falling back to full replay via LoadAll).
type SnapshotStore interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// Save persists a snapshot. aggregateType identifies the projection kind.
	Save(ctx context.Context, aggregateType string, seqNr uint64, state []byte) error

	// Load returns the latest snapshot for the given aggregateType.
	// Returns (0, nil, nil) if no snapshot exists.
	Load(ctx context.Context, aggregateType string) (seqNr uint64, state []byte, err error)
}

// SeqAllocator assigns globally monotonic sequence numbers to events.
// Implemented by eventsource.SeqCounter (SQLite-backed).
type SeqAllocator interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	AllocSeqNr(ctx context.Context) (uint64, error)
}

// OutboxStore is the transactional outbox interface for D-Mail delivery.
// Stage writes to a write-ahead log (SQLite); Flush materialises staged
// items to archive/ and outbox/ using atomic file writes.
type OutboxStore interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	Stage(ctx context.Context, name string, data []byte) error
	Flush(ctx context.Context) (int, error)
	Close() error
}

// StateReader is the interface for reading materialized projection state.
type StateReader interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// LoadLatest returns the most recent check result.
	LoadLatest() (domain.CheckResult, error)

	// ScanInbox consumes inbound D-Mails from the inbox directory (one-shot).
	// Used by RunCheck. The Run daemon uses MonitorInbox (fsnotify) instead.
	ScanInbox(ctx context.Context) ([]domain.DMail, error)

	// NextDMailName generates a unique D-Mail name for the given kind.
	NextDMailName(kind domain.DMailKind) (string, error)

	// LoadAllDMails returns all D-Mails from the archive.
	LoadAllDMails() ([]domain.DMail, error)

	// LoadConsumed returns consumed inbox records.
	LoadConsumed() ([]domain.ConsumedRecord, error)

	// LoadSyncState returns the current sync state.
	LoadSyncState() (domain.SyncState, error)
}

// Git is the interface for repository version control operations.
type Git interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// CurrentCommit returns the short SHA of the current HEAD.
	CurrentCommit() (string, error)

	// CurrentBranch returns the name of the currently checked-out branch.
	CurrentBranch() (string, error)

	// MergedPRsSince returns merged PRs between the given commit and HEAD.
	MergedPRsSince(since string) ([]domain.MergedPR, error)

	// DiffSince returns the unified diff between the given commit and HEAD.
	DiffSince(since string) (string, error)
}

// GitHubPRReader reads open PR state from GitHub (read-only).
// Implemented by session-layer adapter using `gh` CLI.
type GitHubPRReader interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// ListOpenPRs returns all open PRs targeting the given branch.
	ListOpenPRs(ctx context.Context, targetBranch string) ([]domain.PRState, error)
	// GetPRDiff returns the unified diff for the given PR number.
	GetPRDiff(ctx context.Context, prNumber string) (string, error)
	// GetPRMergeReadiness returns the merge readiness state for the given PR.
	GetPRMergeReadiness(ctx context.Context, prNumber string) (*domain.PRMergeReadiness, error)
}

// GitHubPRWriter writes labels and merges PRs on GitHub.
// Implemented by session-layer adapter using `gh` CLI.
type GitHubPRWriter interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// ApplyLabel adds a label to the given PR. Creates the label if it doesn't exist.
	ApplyLabel(ctx context.Context, prNumber, label string) error
	// RemoveLabel removes a label from the given PR.
	RemoveLabel(ctx context.Context, prNumber, label string) error
	// DeleteLabel deletes a label definition from the repository.
	DeleteLabel(ctx context.Context, label string) error
	// MergePR merges the given PR using the specified method.
	MergePR(ctx context.Context, prNumber string, method domain.MergeMethod) error
	// ClosePR closes the given PR with a comment.
	// Used to clean up stale pipeline PRs whose base branch has been merged.
	ClosePR(ctx context.Context, prNumber, comment string) error
}

// GitHubIssueWriter closes issues on GitHub.
// Implemented by session-layer adapter using `gh` CLI.
type GitHubIssueWriter interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// ListOpenIssuesByLabel returns issue numbers with the given label that are still open.
	ListOpenIssuesByLabel(ctx context.Context, label string) ([]string, error)
	// CloseIssue closes the given issue with a comment.
	CloseIssue(ctx context.Context, issueNumber, comment string) error
}

// PRPipelineRunner executes the pre-merge PR convergence pipeline.
// Implemented in usecase layer, injected into session by cmd (composition root).
type PRPipelineRunner interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	RunPreMergePipeline(ctx context.Context, integrationBranch string) ([]domain.DMail, error)
}

// PruneCandidate represents a file eligible for pruning.
type PruneCandidate struct { // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- PruneCandidate co-locates with ArchiveOps as the parameter type for the same port; port contract family cohesive set [permanent]
	Path    string
	ModTime time.Time
}

// ArchiveOps handles file pruning and lifecycle operations.
// Implemented by session-layer adapter; injected into usecase by cmd.
type ArchiveOps interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	FindPruneCandidates(archiveDir string, maxAge time.Duration) ([]PruneCandidate, error)
	PruneFiles(candidates []PruneCandidate) (int, error)
	ListExpiredEventFiles(ctx context.Context, stateDir string, days int) ([]string, error)
	PruneEventFiles(ctx context.Context, stateDir string, files []string) ([]string, error)
	PruneFlushedOutbox(ctx context.Context, root string) (int, error)
}

// CheckEventEmitter wraps aggregate event production + persistence + projection + dispatch.
// Implemented in usecase layer, injected into session by usecase.RunCheck.
// Emit chain: agg.Record*() → store.Append() → projector.Apply() → dispatch (best-effort).
type CheckEventEmitter interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	EmitInboxConsumed(data domain.InboxConsumedData, now time.Time) error
	EmitForceFullNextSet(prevDiv, currDiv float64, now time.Time) error
	EmitDMailGenerated(dmail domain.DMail, now time.Time) error
	EmitConvergenceDetected(alert domain.ConvergenceAlert, now time.Time) error
	EmitDMailCommented(dmailName, issueID string, now time.Time) error
	EmitCheck(result domain.CheckResult, now time.Time) error
	EmitRunStarted(data domain.RunStartedData, now time.Time) error
	EmitRunStopped(data domain.RunStoppedData, now time.Time) error
	EmitPRConvergenceChecked(data domain.PRConvergenceCheckedData, now time.Time) error
	EmitPRMerged(data domain.PRMergedData, now time.Time) error
	EmitPRMergeSkipped(data domain.PRMergeSkippedData, now time.Time) error
}

// CheckStateProvider provides aggregate state read/write without exposing the aggregate type.
// Implemented in usecase layer, injected into session by usecase.RunCheck.
type CheckStateProvider interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	ShouldFullCheck(forceFlag bool) bool
	ForceFullNext() bool
	SetForceFullNext(v bool)
	ShouldPromoteToFull(prevDiv, currDiv float64) bool
	AdvanceCheckCount(fullCheck bool, wasForced bool)
	Restore(result domain.CheckResult)
}

// RunLockStore provides cross-process run locking backed by persistent storage.
// Prevents duplicate runs when multiple CLI instances target the same state directory.
type RunLockStore interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	// TryAcquire attempts to acquire a lock for the given run key.
	// Returns (true, "", nil) if acquired, (false, holder, nil) if already held.
	// Stale locks (past expires_at) are automatically cleaned up.
	TryAcquire(ctx context.Context, runKey string, ttl time.Duration) (acquired bool, holder string, err error)
	// Release releases a lock previously acquired by this holder.
	Release(ctx context.Context, runKey string, holder string) error
	// IsHeld returns whether the lock is currently held and by whom.
	IsHeld(ctx context.Context, runKey string) (held bool, holder string, err error)
	// Close releases database resources.
	Close() error
}

// ImprovementTaskDispatcher dispatches improvement tasks with dedup.
// Implemented by session.ImprovementTaskDispatcher (SQLite-backed).
type ImprovementTaskDispatcher interface { // nosemgrep: structure.multiple-exported-interfaces-go -- port contract family cohesive set; see InitConfig [permanent]
	Dispatch(ctx context.Context, task domain.ImprovementTask, correlationID string) error
	Close() error
}

// NopImprovementTaskDispatcher is a no-op dispatcher for dry-run and tests.
type NopImprovementTaskDispatcher struct{} // nosemgrep: structure.multiple-exported-structs-go,structure.exported-struct-and-interface-go -- null-object for ImprovementTaskDispatcher; must co-locate with interface definition; port null-object family cohesive set [permanent]

func (NopImprovementTaskDispatcher) Dispatch(context.Context, domain.ImprovementTask, string) error {
	return nil
}
func (NopImprovementTaskDispatcher) Close() error { return nil }

// Orchestrator is the session-layer I/O orchestration interface.
// Implemented by session.Amadeus; injected into usecase by cmd (composition root).
type Orchestrator interface {
	RunCheck(ctx context.Context, opts domain.CheckOptions, emitter CheckEventEmitter, state CheckStateProvider) error
	Run(ctx context.Context, opts domain.RunOptions, emitter CheckEventEmitter, state CheckStateProvider) error
	// SetPRPipeline injects the PR convergence pipeline runner.
	SetPRPipeline(runner PRPipelineRunner)
	PrintSync() error
	PrintLog(ctx context.Context) error
	PrintLogJSON(ctx context.Context) error
	MarkCommented(ctx context.Context, dmailName, issueID string) error
	// EventStore returns the event persistence store.
	EventStore() EventStore
	// EventApplier returns the projection applier (ctx-aware port interface).
	EventApplier() ContextEventApplier
	// SeqAllocator returns the global SeqNr allocator (ADR S0040). May return nil.
	SeqAllocator() SeqAllocator
}
