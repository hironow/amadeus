package amadeus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Projector applies domain events to update materialized projection files.
type Projector struct {
	Store *ProjectionStore
}

// Apply processes a single event and updates the relevant projections.
func (p *Projector) Apply(event Event) error {
	switch event.Type {
	case EventCheckCompleted:
		return p.applyCheckCompleted(event)
	case EventBaselineUpdated:
		return p.applyBaselineUpdated(event)
	case EventForceFullNextSet:
		return p.applyForceFullNextSet(event)
	case EventDMailGenerated:
		return p.applyDMailGenerated(event)
	case EventInboxConsumed:
		return p.applyInboxConsumed(event)
	case EventDMailCommented:
		return p.applyDMailCommented(event)
	case EventArchivePruned:
		return p.applyArchivePruned(event)
	case EventConvergenceDetected:
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
func (p *Projector) Rebuild(events []Event) error {
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

	for _, ev := range events {
		if err := p.Apply(ev); err != nil {
			return fmt.Errorf("rebuild event %s (%s): %w", ev.ID, ev.Type, err)
		}
	}
	return nil
}

func (p *Projector) applyCheckCompleted(event Event) error {
	var data CheckCompletedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal CheckCompletedData: %w", err)
	}
	return p.Store.SaveLatest(data.Result)
}

func (p *Projector) applyBaselineUpdated(event Event) error {
	var data BaselineUpdatedData
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

func (p *Projector) applyForceFullNextSet(event Event) error {
	latest, err := p.Store.LoadLatest()
	if err != nil {
		return fmt.Errorf("load latest for ForceFullNext: %w", err)
	}
	latest.ForceFullNext = true
	return p.Store.SaveLatest(latest)
}

func (p *Projector) applyDMailGenerated(event Event) error {
	var data DMailGeneratedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal DMailGeneratedData: %w", err)
	}
	return p.Store.SaveDMail(data.DMail)
}

// applyInboxConsumed records that an inbox D-Mail was consumed.
// NOTE: This only updates consumed.json, not archive/. Inbox D-Mails are copied
// to archive/ at scan time (ScanInbox), not via event replay. This is intentional:
// the event payload contains only metadata, not the full D-Mail content.
func (p *Projector) applyInboxConsumed(event Event) error {
	var data InboxConsumedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal InboxConsumedData: %w", err)
	}
	record := ConsumedRecord{
		Name:       data.Name,
		Kind:       data.Kind,
		ConsumedAt: event.Timestamp,
		Source:     data.Source,
	}
	return p.Store.SaveConsumed([]ConsumedRecord{record})
}

func (p *Projector) applyDMailCommented(event Event) error {
	var data DMailCommentedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("unmarshal DMailCommentedData: %w", err)
	}
	state, err := p.Store.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	key := data.DMail + ":" + data.IssueID
	state.CommentedDMails[key] = CommentRecord{
		DMail:       data.DMail,
		IssueID:     data.IssueID,
		CommentedAt: event.Timestamp,
	}
	return p.Store.SaveSyncState(state)
}

func (p *Projector) applyArchivePruned(event Event) error {
	var data ArchivePrunedData
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
