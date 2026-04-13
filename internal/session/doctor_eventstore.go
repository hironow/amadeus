package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
)

// CheckDeadLetters reports outbox items that have exceeded max retry count.
func CheckDeadLetters(ctx context.Context, repoRoot string) domain.DoctorCheck {
	// Check DB file exists before opening (avoid creating dirs/DB as side effect)
	dbPath := filepath.Join(repoRoot, domain.StateDir, ".run", "outbox.db")
	if _, err := os.Stat(dbPath); err != nil {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckSkip,
			Message: "no outbox DB",
		}
	}
	store, err := NewOutboxStoreForDir(repoRoot)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckSkip,
			Message: "outbox store unavailable",
		}
	}
	defer store.Close()

	count, err := store.DeadLetterCount(ctx)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckSkip,
			Message: fmt.Sprintf("dead letter count: %v", err),
		}
	}
	if count > 0 {
		return domain.DoctorCheck{
			Name:    "dead-letters",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("%d dead-lettered outbox item(s)", count),
			Hint:    "run 'amadeus dead-letters purge --execute' to remove",
		}
	}
	return domain.DoctorCheck{
		Name:    "dead-letters",
		Status:  domain.CheckOK,
		Message: "no dead-lettered items",
	}
}

// CheckEventStore verifies events/ directory exists and all JSONL files are parseable.
func CheckEventStore(gateRoot string) domain.DoctorCheck {
	eventsDir := filepath.Join(gateRoot, "events")
	if _, err := os.Stat(eventsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DoctorCheck{
				Name:    "Event Store",
				Status:  domain.CheckSkip,
				Message: "no events directory — run 'amadeus init'",
			}
		}
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("stat events: %v", err),
			Hint:    `run "amadeus init" to create the events directory`,
		}
	}
	count, corruptCount, err := countEventStoreEntries(eventsDir)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("event store read error: %v", err),
			Hint:    "check event files in .gate/events/ for corruption",
		}
	}
	if corruptCount > 0 {
		return domain.DoctorCheck{
			Name:    "Event Store",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("%d event(s) loaded, %d corrupt line(s) skipped", count, corruptCount),
			Hint:    "corrupt lines are skipped during replay — review JSONL files in .gate/events/",
		}
	}
	return domain.DoctorCheck{
		Name:    "Event Store",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d event(s) loaded", count),
	}
}

// countEventStoreEntries reads all .jsonl files in the events directory // nosemgrep: layer-session-no-event-persistence [permanent] read-only health check, not event persistence
// and counts valid event entries. Returns an error if any line fails to parse.
func countEventStoreEntries(eventsDir string) (count int, corruptCount int, err error) {
	entries, readErr := os.ReadDir(eventsDir)
	if readErr != nil {
		return 0, 0, fmt.Errorf("read events dir: %w", readErr)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl") { // nosemgrep: layer-session-no-event-persistence [permanent] file extension check for read-only doctor scan
			continue
		}
		data, fileErr := os.ReadFile(filepath.Join(eventsDir, e.Name()))
		if fileErr != nil {
			return 0, 0, fmt.Errorf("read %s: %w", e.Name(), fileErr)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var ev domain.Event
			if parseErr := json.Unmarshal([]byte(line), &ev); parseErr != nil {
				corruptCount++
				continue
			}
			count++
		}
	}
	return count, corruptCount, nil
}

// CheckDMailSchema validates all D-Mails in archive/ conform to schema v1.
func CheckDMailSchema(gateRoot string) domain.DoctorCheck {
	archiveDir := filepath.Join(gateRoot, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DoctorCheck{
				Name:    "D-Mail Schema",
				Status:  domain.CheckSkip,
				Message: "no archive directory",
			}
		}
		return domain.DoctorCheck{
			Name:    "D-Mail Schema",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("read archive: %v", err),
			Hint:    "check file permissions on the .gate/archive/ directory",
		}
	}

	var mdFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	if len(mdFiles) == 0 {
		return domain.DoctorCheck{
			Name:    "D-Mail Schema",
			Status:  domain.CheckSkip,
			Message: "no D-Mails in archive",
		}
	}

	var invalid []string
	for _, name := range mdFiles {
		data, readErr := os.ReadFile(filepath.Join(archiveDir, name))
		if readErr != nil {
			invalid = append(invalid, fmt.Sprintf("%s: read error", name))
			continue
		}
		dmail, parseErr := domain.ParseDMail(data)
		if parseErr != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", name, parseErr))
			continue
		}
		if errs := harness.ValidateDMail(dmail); len(errs) > 0 {
			invalid = append(invalid, fmt.Sprintf("%s: %s", name, strings.Join(errs, "; ")))
		}
	}

	if len(invalid) > 0 {
		return domain.DoctorCheck{
			Name:    "D-Mail Schema",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%d/%d invalid: %s", len(invalid), len(mdFiles), strings.Join(invalid, ", ")),
			Hint:    "re-send affected D-Mails or manually fix the frontmatter",
		}
	}
	return domain.DoctorCheck{
		Name:    "D-Mail Schema",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d D-Mail(s) valid", len(mdFiles)),
	}
}
