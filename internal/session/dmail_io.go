package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	amadeus "github.com/hironow/amadeus"
)

// NextDMailName returns the next sequential D-Mail name by scanning existing
// .md files in the archive/ directory.
func (s *ProjectionStore) NextDMailName(kind amadeus.DMailKind) (string, error) {
	archiveDir := filepath.Join(s.Root, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return "", err
	}
	prefix := string(kind) + "-"
	maxNum := 0
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		if strings.HasPrefix(name, prefix) {
			var num int
			if _, err := fmt.Sscanf(name, prefix+"%d", &num); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}
	return fmt.Sprintf("%s-%03d", kind, maxNum+1), nil
}

// SaveDMailToArchive writes a D-Mail to archive/ only, skipping outbox/.
// Used during rebuild to avoid re-queuing historical D-Mails for delivery.
func (s *ProjectionStore) SaveDMailToArchive(dmail amadeus.DMail) error {
	data, err := amadeus.MarshalDMail(dmail)
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
func (s *ProjectionStore) LoadDMail(name string) (amadeus.DMail, error) {
	path := filepath.Join(s.Root, "archive", name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return amadeus.DMail{}, fmt.Errorf("load dmail %s: %w", name, err)
	}
	return amadeus.ParseDMail(data)
}

// LoadAllDMails reads all D-Mails from the archive/ directory, sorted by name ascending.
func (s *ProjectionStore) LoadAllDMails() ([]amadeus.DMail, error) {
	archiveDir := filepath.Join(s.Root, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil, err
	}
	var dmails []amadeus.DMail
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
func (s *ProjectionStore) LoadConsumed() ([]amadeus.ConsumedRecord, error) {
	path := filepath.Join(s.Root, ".run", "consumed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []amadeus.ConsumedRecord{}, nil
		}
		return nil, err
	}
	var records []amadeus.ConsumedRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// SaveConsumed appends consumed records to .run/consumed.json.
func (s *ProjectionStore) SaveConsumed(records []amadeus.ConsumedRecord) error {
	existing, err := s.LoadConsumed()
	if err != nil {
		return err
	}
	existing = append(existing, records...)
	return s.writeJSON(filepath.Join(s.Root, ".run", "consumed.json"), existing)
}

// ScanInbox reads all .md files from inbox/, parses them with ParseDMail,
// copies to archive/ (skip if already exists), and removes from inbox/.
// Returns the parsed D-Mails sorted by name.
//
// NOTE: All D-Mail I/O (inbox, outbox, archive) uses synchronous
// os.ReadDir/ReadFile/WriteFile/Rename — no file-system watcher such as
// github.com/fsnotify/fsnotify is involved. amadeus is a one-shot CLI
// invoked by cron or git hooks, so polling at invocation time is sufficient.
// A watcher would only be warranted if amadeus were daemonised for
// real-time inbox delivery.
func (s *ProjectionStore) ScanInbox() ([]amadeus.DMail, error) {
	inboxDir := filepath.Join(s.Root, "inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read inbox: %w", err)
	}

	var dmails []amadeus.DMail
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		inboxPath := filepath.Join(inboxDir, e.Name())
		data, err := os.ReadFile(inboxPath)
		if err != nil {
			return nil, fmt.Errorf("read inbox file %s: %w", e.Name(), err)
		}
		dmail, err := amadeus.ParseDMail(data)
		if err != nil {
			return nil, fmt.Errorf("parse inbox file %s: %w", e.Name(), err)
		}

		// Copy to archive (skip if exists)
		archivePath := filepath.Join(s.Root, "archive", e.Name())
		if _, statErr := os.Stat(archivePath); errors.Is(statErr, fs.ErrNotExist) {
			if err := os.WriteFile(archivePath, data, 0o644); err != nil {
				return nil, fmt.Errorf("archive %s: %w", e.Name(), err)
			}
		}

		// Remove from inbox
		if err := os.Remove(inboxPath); err != nil {
			return nil, fmt.Errorf("remove inbox %s: %w", e.Name(), err)
		}

		dmails = append(dmails, dmail)
	}

	sort.Slice(dmails, func(i, j int) bool {
		return dmails[i].Name < dmails[j].Name
	})
	return dmails, nil
}
