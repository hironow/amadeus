package session

import (
	"context"
	"io"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// Amadeus is the data-plane orchestrator for the MCP/sessions era. It exposes
// the event store, projection applier, and sequence allocator that the kept
// data-plane commands (log, sync, mark-commented) and the MCP server build on.
type Amadeus struct {
	Config      domain.Config
	Store       port.StateReader
	Events      port.EventStore          // nil skips event persistence (Projector still required for writes)
	Projector   port.ContextEventApplier // nil skips projection updates (Events still required for writes)
	Git         port.Git
	RepoDir     string // repository root directory
	Logger      domain.Logger
	DataOut     io.Writer                      // machine-readable output (stdout); Logger is for human progress (stderr)
	Approver    port.Approver                  // nil = no gate (auto-approve)
	Notifier    port.Notifier                  // nil = no notifications
	Metrics     port.PolicyMetrics             // nil = no policy metrics
	PRReader    port.GitHubPRReader            // nil = skip PR convergence
	PRWriter    port.GitHubPRWriter            // nil = skip PR label writes
	IssueWriter port.GitHubIssueWriter         // nil = skip issue close
	Emitter     port.CheckEventEmitter         // event production + persistence + dispatch (injected by usecase layer)
	State       port.CheckStateProvider        // aggregate state read/write (injected by usecase layer)
	SeqAlloc    port.SeqAllocator              // global SeqNr (ADR S0040)
	Insights    *InsightWriter                 // nil = skip insight generation
	Collector   *ImprovementCollector          // nil = skip external improvement signal ingestion
	Policy      domain.RoutingPolicy           // corrective routing policy (loaded from YAML, fallback = default)
	Dispatcher  port.ImprovementTaskDispatcher // never nil — use NopImprovementTaskDispatcher for dry-run/tests
}

// EventStore returns the event persistence store.
func (a *Amadeus) EventStore() port.EventStore {
	return a.Events
}

// EventApplier returns the projection applier (ctx-aware port interface).
func (a *Amadeus) EventApplier() port.ContextEventApplier {
	return a.Projector
}

func (a *Amadeus) SeqAllocator() port.SeqAllocator {
	return a.SeqAlloc
}

// MarkCommented records that a D-Mail x Issue pair has been posted as a comment.
func (a *Amadeus) MarkCommented(ctx context.Context, dmailName, issueID string) error {
	return a.Emitter.EmitDMailCommented(dmailName, issueID, time.Now().UTC())
}
