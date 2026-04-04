package port

import (
	"context"
	"errors"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

// InitRunner handles state directory initialization I/O.
type InitRunner interface {
	InitGateDir(stateDir string) error
}

// EventDispatcher dispatches domain events to policy handlers.
// Implemented by usecase.PolicyEngine; injected into session via Amadeus struct.
type EventDispatcher interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// Approver determines whether an action should proceed.
// Implementations include StdinApprover (human prompt),
// CmdApprover (external command), and AutoApprover (always yes).
type Approver interface {
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

// AutoApprover always approves without human interaction.
type AutoApprover struct{}

func (*AutoApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// Notifier sends notifications about events.
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// NopNotifier is a no-op notifier for quiet mode or testing.
type NopNotifier struct{}

func (*NopNotifier) Notify(_ context.Context, _, _ string) error { return nil }

// PolicyMetrics records policy handler execution metrics.
type PolicyMetrics interface {
	RecordPolicyEvent(ctx context.Context, eventType string, status string)
}

// NopPolicyMetrics is a no-op metrics recorder for tests and quiet mode.
type NopPolicyMetrics struct{}

func (*NopPolicyMetrics) RecordPolicyEvent(_ context.Context, _, _ string) {}

// EventStore is the append-only event persistence interface.
type EventStore interface {
	// Append persists one or more events. Validation is performed before any writes.
	Append(events ...domain.Event) (domain.AppendResult, error)

	// LoadAll returns all events in chronological order.
	LoadAll() ([]domain.Event, domain.LoadResult, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error)

	// LoadAfterSeqNr returns all events with SeqNr > afterSeqNr,
	// ordered by SeqNr ascending. Used for snapshot-based recovery.
	LoadAfterSeqNr(afterSeqNr uint64) ([]domain.Event, domain.LoadResult, error)

	// LatestSeqNr returns the highest recorded SeqNr across all events.
	// Returns 0 if no events have a SeqNr assigned.
	LatestSeqNr() (uint64, error)
}

// SnapshotStore persists materialized projection state at a known SeqNr.
// Snapshots are an optimization — the system must function without them
// (falling back to full replay via LoadAll).
type SnapshotStore interface {
	// Save persists a snapshot. aggregateType identifies the projection kind.
	Save(ctx context.Context, aggregateType string, seqNr uint64, state []byte) error

	// Load returns the latest snapshot for the given aggregateType.
	// Returns (0, nil, nil) if no snapshot exists.
	Load(ctx context.Context, aggregateType string) (seqNr uint64, state []byte, err error)
}

// SeqAllocator assigns globally monotonic sequence numbers to events.
// Implemented by eventsource.SeqCounter (SQLite-backed).
type SeqAllocator interface {
	AllocSeqNr(ctx context.Context) (uint64, error)
}

// OutboxStore is the transactional outbox interface for D-Mail delivery.
// Stage writes to a write-ahead log (SQLite); Flush materialises staged
// items to archive/ and outbox/ using atomic file writes.
type OutboxStore interface {
	Stage(ctx context.Context, name string, data []byte) error
	Flush(ctx context.Context) (int, error)
	Close() error
}

// StateReader is the interface for reading materialized projection state.
type StateReader interface {
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
type Git interface {
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
type GitHubPRReader interface {
	// ListOpenPRs returns all open PRs targeting the given branch.
	ListOpenPRs(ctx context.Context, targetBranch string) ([]domain.PRState, error)
	// GetPRDiff returns the unified diff for the given PR number.
	GetPRDiff(ctx context.Context, prNumber string) (string, error)
	// GetPRMergeReadiness returns the merge readiness state for the given PR.
	GetPRMergeReadiness(ctx context.Context, prNumber string) (*domain.PRMergeReadiness, error)
}

// GitHubPRWriter writes labels and merges PRs on GitHub.
// Implemented by session-layer adapter using `gh` CLI.
type GitHubPRWriter interface {
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
type GitHubIssueWriter interface {
	// ListOpenIssuesByLabel returns issue numbers with the given label that are still open.
	ListOpenIssuesByLabel(ctx context.Context, label string) ([]string, error)
	// CloseIssue closes the given issue with a comment.
	CloseIssue(ctx context.Context, issueNumber, comment string) error
}

// PRPipelineRunner executes the pre-merge PR convergence pipeline.
// Implemented in usecase layer, injected into session by cmd (composition root).
type PRPipelineRunner interface {
	RunPreMergePipeline(ctx context.Context, integrationBranch string) ([]domain.DMail, error)
}

// PruneCandidate represents a file eligible for pruning.
type PruneCandidate struct {
	Path    string
	ModTime time.Time
}

// ArchiveOps handles file pruning and lifecycle operations.
// Implemented by session-layer adapter; injected into usecase by cmd.
type ArchiveOps interface {
	FindPruneCandidates(archiveDir string, maxAge time.Duration) ([]PruneCandidate, error)
	PruneFiles(candidates []PruneCandidate) (int, error)
	ListExpiredEventFiles(ctx context.Context, stateDir string, days int) ([]string, error)
	PruneEventFiles(ctx context.Context, stateDir string, files []string) ([]string, error)
	PruneFlushedOutbox(ctx context.Context, root string) (int, error)
}

// CheckEventEmitter wraps aggregate event production + persistence + projection + dispatch.
// Implemented in usecase layer, injected into session by usecase.RunCheck.
// Emit chain: agg.Record*() → store.Append() → projector.Apply() → dispatch (best-effort).
type CheckEventEmitter interface {
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
type CheckStateProvider interface {
	ShouldFullCheck(forceFlag bool) bool
	ForceFullNext() bool
	SetForceFullNext(v bool)
	ShouldPromoteToFull(prevDiv, currDiv float64) bool
	AdvanceCheckCount(fullCheck bool, wasForced bool)
	Restore(result domain.CheckResult)
}

// Orchestrator is the session-layer I/O orchestration interface.
// Implemented by session.Amadeus; injected into usecase by cmd (composition root).
type Orchestrator interface {
	RunCheck(ctx context.Context, opts domain.CheckOptions, emitter CheckEventEmitter, state CheckStateProvider) error
	Run(ctx context.Context, opts domain.RunOptions, emitter CheckEventEmitter, state CheckStateProvider) error
	// SetPRPipeline injects the PR convergence pipeline runner.
	SetPRPipeline(runner PRPipelineRunner)
	PrintSync() error
	PrintLog() error
	PrintLogJSON() error
	MarkCommented(dmailName, issueID string) error
	// EventStore returns the event persistence store.
	EventStore() EventStore
	// EventApplier returns the projection applier.
	EventApplier() domain.EventApplier
}
