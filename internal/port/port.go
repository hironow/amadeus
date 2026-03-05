package port

import (
	"context"
	"errors"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

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

// ClaudeRunner executes the Claude CLI and returns raw JSON output.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string) ([]byte, error)
}

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
	Append(events ...domain.Event) error

	// LoadAll returns all events in chronological order.
	LoadAll() ([]domain.Event, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]domain.Event, error)
}

// OutboxStore is the transactional outbox interface for D-Mail delivery.
// Stage writes to a write-ahead log (SQLite); Flush materialises staged
// items to archive/ and outbox/ using atomic file writes.
type OutboxStore interface {
	Stage(name string, data []byte) error
	Flush() (int, error)
	Close() error
}

// StateReader is the interface for reading materialized projection state.
type StateReader interface {
	// LoadLatest returns the most recent check result.
	LoadLatest() (domain.CheckResult, error)

	// ScanInbox consumes inbound D-Mails from the inbox directory.
	ScanInbox() ([]domain.DMail, error)

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

	// MergedPRsSince returns merged PRs between the given commit and HEAD.
	MergedPRsSince(since string) ([]domain.MergedPR, error)

	// DiffSince returns the unified diff between the given commit and HEAD.
	DiffSince(since string) (string, error)
}
