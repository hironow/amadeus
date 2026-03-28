package session

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

// NextDMailName returns the next sequential D-Mail name by scanning existing
// .md files in the archive/ directory.
func (s *ProjectionStore) NextDMailName(kind domain.DMailKind) (string, error) {
	archiveDir := filepath.Join(s.Root, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return "", err
	}
	// Scan both legacy (kind-NNN) and new (am-kind-NNN_uuid) formats
	legacyPrefix := string(kind) + "-"
	newPrefix := "am-" + string(kind) + "-"
	maxNum := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		// Strip UUID suffix for new format: am-kind-NNN_xxxxxxxx → am-kind-NNN
		if idx := strings.LastIndex(name, "_"); idx > 0 {
			name = name[:idx]
		}
		var num int
		if strings.HasPrefix(name, newPrefix) {
			if _, err := fmt.Sscanf(name, newPrefix+"%d", &num); err == nil && num > maxNum {
				maxNum = num
			}
		} else if strings.HasPrefix(name, legacyPrefix) {
			if _, err := fmt.Sscanf(name, legacyPrefix+"%d", &num); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}
	return fmt.Sprintf("am-%s-%03d_%s", kind, maxNum+1, shortDMailUUID()), nil
}

// SaveDMailToArchive writes a D-Mail to archive/ only, skipping outbox/.
// Used during rebuild to avoid re-queuing historical D-Mails for delivery.
func (s *ProjectionStore) SaveDMailToArchive(dmail domain.DMail) error {
	data, err := domain.MarshalDMail(dmail)
	if err != nil {
		return fmt.Errorf("marshal dmail: %w", err)
	}
	filename := dmail.Name + ".md"
	archivePath := filepath.Join(s.Root, "archive", filename)
	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	return nil
}

// LoadDMail reads a single D-Mail by name from the archive/ directory.
func (s *ProjectionStore) LoadDMail(name string) (domain.DMail, error) {
	path := filepath.Join(s.Root, "archive", name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.DMail{}, fmt.Errorf("load dmail %s: %w", name, err)
	}
	return domain.ParseDMail(data)
}

// LoadAllDMails reads all D-Mails from the archive/ directory, sorted by name ascending.
func (s *ProjectionStore) LoadAllDMails() ([]domain.DMail, error) {
	archiveDir := filepath.Join(s.Root, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil, err
	}
	var dmails []domain.DMail
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		dmail, err := s.LoadDMail(name)
		if err != nil {
			return nil, err
		}
		dmails = append(dmails, dmail)
	}
	sort.Slice(dmails, func(i, j int) bool {
		return dmails[i].Name < dmails[j].Name
	})
	return dmails, nil
}

// LoadConsumed reads all consumed records from .run/consumed.json.
func (s *ProjectionStore) LoadConsumed() ([]domain.ConsumedRecord, error) {
	path := filepath.Join(s.Root, ".run", "consumed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []domain.ConsumedRecord{}, nil
		}
		return nil, err
	}
	var records []domain.ConsumedRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// SaveConsumed appends consumed records to .run/consumed.json.
func (s *ProjectionStore) SaveConsumed(records []domain.ConsumedRecord) error {
	existing, err := s.LoadConsumed()
	if err != nil {
		return err
	}
	existing = append(existing, records...)
	return s.writeJSON(filepath.Join(s.Root, ".run", "consumed.json"), existing)
}

// ReceiveDMailFromInbox reads a single D-Mail file from inbox/, applies
// archive-based dedup, archives the file, and removes it from inbox/.
// Returns (nil, nil) if the file is already archived (dedup).
// Returns (nil, err) if the file cannot be read or parsed (left in inbox for retry).
func ReceiveDMailFromInbox(root, filename string) (*domain.DMail, error) {
	archivePath := filepath.Join(root, "archive", filename)
	inboxPath := filepath.Join(root, "inbox", filename)

	// Dedup: if already archived, clean up inbox (best-effort) and return nil.
	if _, err := os.Stat(archivePath); err == nil {
		_ = os.Remove(inboxPath) // best-effort cleanup
		return nil, nil
	}

	data, err := os.ReadFile(inboxPath)
	if err != nil {
		return nil, fmt.Errorf("read inbox file %s: %w", filename, err)
	}

	dmail, err := domain.ParseDMail(data)
	if err != nil {
		return nil, fmt.Errorf("parse inbox file %s: %w", filename, err)
	}

	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return nil, fmt.Errorf("archive %s: %w", filename, err)
	}

	if err := os.Remove(inboxPath); err != nil {
		return nil, fmt.Errorf("remove inbox %s: %w", filename, err)
	}

	return &dmail, nil
}

// ScanInbox reads all .md files from inbox/, parses them with ParseDMail,
// copies to archive/ (skip if already exists), and removes from inbox/.
// Returns the parsed D-Mails sorted by name.
//
// Used by RunCheck (one-shot check command). The Run daemon loop uses
// MonitorInbox (fsnotify-based, in inbox_watcher.go) for real-time D-Mail reception.
func (s *ProjectionStore) ScanInbox(ctx context.Context) ([]domain.DMail, error) {
	ctx, span := platform.Tracer.Start(ctx, "amadeus.dmail_io")
	defer span.End()

	inboxDir := filepath.Join(s.Root, "inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			span.SetAttributes(attribute.Int("inbox.scan.count", 0))
			return nil, nil
		}
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "amadeus.dmail_io"))
		return nil, fmt.Errorf("read inbox: %w", err)
	}

	var dmails []domain.DMail
	archiveCount := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		inboxPath := filepath.Join(inboxDir, e.Name())
		data, err := os.ReadFile(inboxPath)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.stage", "amadeus.dmail_io"))
			return nil, fmt.Errorf("read inbox file %s: %w", e.Name(), err)
		}
		dmail, err := domain.ParseDMail(data)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.stage", "amadeus.dmail_io"))
			return nil, fmt.Errorf("parse inbox file %s: %w", e.Name(), err)
		}

		// Copy to archive (skip if exists)
		archivePath := filepath.Join(s.Root, "archive", e.Name())
		if _, statErr := os.Stat(archivePath); errors.Is(statErr, fs.ErrNotExist) {
			if err := os.WriteFile(archivePath, data, 0o644); err != nil {
				span.RecordError(err)
				span.SetAttributes(attribute.String("error.stage", "amadeus.dmail_io"))
				return nil, fmt.Errorf("archive %s: %w", e.Name(), err)
			}
			archiveCount++
		}

		// Remove from inbox
		if err := os.Remove(inboxPath); err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.stage", "amadeus.dmail_io"))
			return nil, fmt.Errorf("remove inbox %s: %w", e.Name(), err)
		}

		dmails = append(dmails, dmail)
	}

	sort.Slice(dmails, func(i, j int) bool {
		return dmails[i].Name < dmails[j].Name
	})
	span.SetAttributes(
		attribute.Int("inbox.scan.count", len(dmails)),
		attribute.Int("archive.write.count", archiveCount),
	)
	return dmails, nil
}

func shortDMailUUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x", buf[:4])
}
