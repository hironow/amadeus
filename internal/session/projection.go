package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	amadeus "github.com/hironow/amadeus"
)

// Compile-time check that Projector implements amadeus.EventApplier.
var _ amadeus.EventApplier = (*Projector)(nil)

// Projector applies domain events to update materialized projection files.
type Projector struct {
	Store       *ProjectionStore
	OutboxStore amadeus.OutboxStore // transactional outbox for D-Mail delivery
	rebuilding  bool                // true during Rebuild to skip outbox writes
}

// Apply processes a single event and updates the relevant projections.
func (p *Projector) Apply(event amadeus.Event) error {
	switch event.Type {
	case amadeus.EventCheckCompleted:
		return p.applyCheckCompleted(event)
	case amadeus.EventBaselineUpdated:
		return p.applyBaselineUpdated(event)
	case amadeus.EventForceFullNextSet:
		return p.applyForceFullNextSet(event)
	case amadeus.EventDMailGenerated:
		return p.applyDMailGenerated(event)
	case amadeus.EventInboxConsumed:
		return p.applyInboxConsumed(event)
	case amadeus.EventDMailCommented:
		return p.applyDMailCommented(event)
	case amadeus.EventArchivePruned:
		return p.applyArchivePruned(event)
	case amadeus.EventConvergenceDetected:
		return nil // informational only, no projection needed
	default:
		return nil // unknown events are ignored for forward compatibility
	}
}

// Rebuild replays all events to regenerate projections from scratch.
// All projection directories (.run/, archive/, outbox/) are cleared before replay
// so that rebuilt state exactly reflects the event stream.
// NOTE: Inbox-sourced D-Mails (consumed via ScanInbox) are NOT reconstructed
// because inbox.consumed events contain only metadata, not the full D-Mail content.
func (p *Projector) Rebuild(events []amadeus.Event) error {
	// Clear all projection directories
	for _, sub := range []string{".run", "archive", "outbox"} {
		dir := filepath.Join(p.Store.Root, sub)
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear %s: %w", sub, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", sub, err)
		}
	}

	p.rebuilding = true
	defer func() { p.rebuilding = false }()

	for _, ev := range events {
		if err := p.Apply(ev); err != nil {
			return fmt.Errorf("rebuild event %s (%s): %w", ev.ID, ev.Type, err)
		}
	}
	return nil
}

func (p *Projector) applyCheckCompleted(event amadeus.Event) error {
	var data amadeus.CheckCompletedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal CheckCompletedData: %w", err)
	}
	return p.Store.SaveLatest(data.Result)
}

func (p *Projector) applyBaselineUpdated(event amadeus.Event) error {
	var data amadeus.BaselineUpdatedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal BaselineUpdatedData: %w", err)
	}
	// Load current latest and save it as baseline with the updated commit/divergence
	latest, err := p.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load latest for baseline: %w", err)
	}
	latest.Commit = data.Commit
	latest.Divergence = data.Divergence
	return p.Store.SaveBaseline(latest)
}

func (p *Projector) applyForceFullNextSet(event amadeus.Event) error {
	latest, err := p.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load latest for ForceFullNext: %w", err)
	}
	latest.ForceFullNext = true
	return p.Store.SaveLatest(latest)
}

func (p *Projector) applyDMailGenerated(event amadeus.Event) error {
	var data amadeus.DMailGeneratedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal DMailGeneratedData: %w", err)
	}
	// During rebuild, only write to archive/ (permanent record).
	// Skip outbox/ to avoid re-queuing historical D-Mails for delivery.
	if p.rebuilding {
		return p.Store.SaveDMailToArchive(data.DMail)
	}
	// Normal mode: transactional outbox (Stage → Flush to archive/ + outbox/).
	marshaledData, err := amadeus.MarshalDMail(data.DMail)
	if err != nil {
		return fmt.Errorf("marshal dmail: %w", err)
	}
	filename := data.DMail.Name + ".md"
	if err := p.OutboxStore.Stage(filename, marshaledData); err != nil {
		return fmt.Errorf("stage dmail: %w", err)
	}
	n, err := p.OutboxStore.Flush()
	if err != nil {
		return fmt.Errorf("flush dmail: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("flush dmail: item not delivered (write failure, will retry)")
	}
	return nil
}

// applyInboxConsumed records that an inbox D-Mail was consumed.
// NOTE: This only updates consumed.json, not archive/. Inbox D-Mails are copied
// to archive/ at scan time (ScanInbox), not via event replay. This is intentional:
// the event payload contains only metadata, not the full D-Mail content.
func (p *Projector) applyInboxConsumed(event amadeus.Event) error {
	var data amadeus.InboxConsumedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal InboxConsumedData: %w", err)
	}
	record := amadeus.ConsumedRecord{
		Name:       data.Name,
		Kind:       data.Kind,
		ConsumedAt: event.Timestamp,
		Source:     data.Source,
	}
	return p.Store.SaveConsumed([]amadeus.ConsumedRecord{record})
}

func (p *Projector) applyDMailCommented(event amadeus.Event) error {
	var data amadeus.DMailCommentedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal DMailCommentedData: %w", err)
	}
	state, err := p.Store.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	key := data.DMail + ":" + data.IssueID
	state.CommentedDMails[key] = amadeus.CommentRecord{
		DMail:       data.DMail,
		IssueID:     data.IssueID,
		CommentedAt: event.Timestamp,
	}
	return p.Store.SaveSyncState(state)
}

func (p *Projector) applyArchivePruned(event amadeus.Event) error {
	var data amadeus.ArchivePrunedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal ArchivePrunedData: %w", err)
	}
	archiveDir := filepath.Join(p.Store.Root, "archive")
	for _, name := range data.Paths {
		// Reject path traversal: only allow plain filenames
		if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == ".." {
			continue
		}
		// Only delete archive/ files. Event files (.jsonl) are the source of
		// truth and must never be deleted by a projection handler — the CLI
		// archive-prune command handles event file deletion directly.
		if strings.HasSuffix(name, ".jsonl") {
			continue
		}
		os.Remove(filepath.Join(archiveDir, name))
	}
	return nil
}
