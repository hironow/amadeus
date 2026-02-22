package amadeus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DMailKind represents the type of D-Mail message.
type DMailKind string

const (
	// KindFeedback is produced by the verifier role.
	KindFeedback DMailKind = "feedback"
	// KindSpecification is produced by the designer role.
	KindSpecification DMailKind = "specification"
	// KindReport is produced by the implementer role.
	KindReport DMailKind = "report"
	// KindConvergence is generated when multiple D-Mails converge on the same target.
	KindConvergence DMailKind = "convergence"
)

// DMailStatus represents the lifecycle status of a D-Mail.
type DMailStatus string

const (
	// DMailSent indicates a D-Mail that has been auto-sent.
	DMailSent DMailStatus = "sent"
)

// dmailFrontmatter is the YAML frontmatter of a D-Mail file.
// NOTE(MY-346): linear_issue_id was intentionally removed without migration.
// Existing D-Mail files with linear_issue_id will lose that field on parse.
// This is acceptable because amadeus is pre-release and no production .gate/ state exists.
type dmailFrontmatter struct {
	Name        string            `yaml:"name"`
	Kind        DMailKind         `yaml:"kind"`
	Description string            `yaml:"description"`
	Issues      []string          `yaml:"issues,omitempty"`
	Severity    Severity          `yaml:"severity,omitempty"`
	Targets     []string          `yaml:"targets,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// DMail is the correction routing message using YAML frontmatter + Markdown body.
type DMail struct {
	Name        string            `yaml:"name"`
	Kind        DMailKind         `yaml:"kind"`
	Description string            `yaml:"description"`
	Issues      []string          `yaml:"issues,omitempty"`
	Severity    Severity          `yaml:"severity,omitempty"`
	Targets     []string          `yaml:"targets,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
	Body        string            `yaml:"-"`
}

// validKinds is the set of valid DMailKind values per schema v1.
var validKinds = map[DMailKind]bool{
	KindFeedback:      true,
	KindSpecification: true,
	KindReport:        true,
	KindConvergence:   true,
}

// validSeverities is the set of valid Severity values per schema v1.
var validSeverities = map[Severity]bool{
	SeverityLow:    true,
	SeverityMedium: true,
	SeverityHigh:   true,
}

// ValidateDMail checks that a DMail conforms to D-Mail schema v1.
// Returns a list of validation errors (empty if valid).
func ValidateDMail(dmail DMail) []string {
	var errs []string
	if dmail.Name == "" {
		errs = append(errs, "name is required")
	}
	if dmail.Kind == "" {
		errs = append(errs, "kind is required")
	} else if !validKinds[dmail.Kind] {
		errs = append(errs, fmt.Sprintf("invalid kind: %q", dmail.Kind))
	}
	if dmail.Description == "" {
		errs = append(errs, "description is required")
	}
	if dmail.Severity != "" && !validSeverities[dmail.Severity] {
		errs = append(errs, fmt.Sprintf("invalid severity: %q", dmail.Severity))
	}
	return errs
}

// ParseDMail parses a D-Mail from raw bytes in YAML frontmatter + Markdown format.
func ParseDMail(data []byte) (DMail, error) {
	str := string(data)
	if !strings.HasPrefix(str, "---\n") {
		return DMail{}, fmt.Errorf("missing opening frontmatter delimiter")
	}
	rest := str[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return DMail{}, fmt.Errorf("missing closing frontmatter delimiter")
	}
	yamlPart := rest[:idx]
	bodyPart := rest[idx+5:] // skip "\n---\n"

	var fm dmailFrontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return DMail{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	return DMail{
		Name:        fm.Name,
		Kind:        fm.Kind,
		Description: fm.Description,
		Issues:      fm.Issues,
		Severity:    NormalizeSeverity(fm.Severity),
		Targets:     fm.Targets,
		Metadata:    fm.Metadata,
		Body:        strings.TrimLeft(bodyPart, "\n"),
	}, nil
}

// MarshalDMail serializes a DMail to YAML frontmatter + Markdown format.
func MarshalDMail(dmail DMail) ([]byte, error) {
	fm := dmailFrontmatter{
		Name:        dmail.Name,
		Kind:        dmail.Kind,
		Description: dmail.Description,
		Issues:      dmail.Issues,
		Severity:    dmail.Severity,
		Targets:     dmail.Targets,
		Metadata:    dmail.Metadata,
	}
	yamlData, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlData)
	buf.WriteString("---\n")
	if dmail.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(dmail.Body)
		if !strings.HasSuffix(dmail.Body, "\n") {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}

// NextDMailName returns the next sequential D-Mail name by scanning existing
// .md files in the archive/ directory.
func (s *StateStore) NextDMailName(kind DMailKind) (string, error) {
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

// SaveDMail writes a D-Mail to archive/ and outbox/.
// Archive is always written first so the permanent record exists even if
// the second write fails. All D-Mails go directly to outbox/ (MY-359).
func (s *StateStore) SaveDMail(dmail DMail) error {
	data, err := MarshalDMail(dmail)
	if err != nil {
		return fmt.Errorf("marshal dmail: %w", err)
	}
	filename := dmail.Name + ".md"
	archivePath := filepath.Join(s.Root, "archive", filename)
	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	outboxPath := filepath.Join(s.Root, "outbox", filename)
	if err := os.WriteFile(outboxPath, data, 0o644); err != nil {
		return fmt.Errorf("write outbox: %w", err)
	}
	return nil
}

// LoadDMail reads a single D-Mail by name from the archive/ directory.
func (s *StateStore) LoadDMail(name string) (DMail, error) {
	path := filepath.Join(s.Root, "archive", name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return DMail{}, fmt.Errorf("load dmail %s: %w", name, err)
	}
	return ParseDMail(data)
}

// LoadAllDMails reads all D-Mails from the archive/ directory, sorted by name ascending.
func (s *StateStore) LoadAllDMails() ([]DMail, error) {
	archiveDir := filepath.Join(s.Root, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil, err
	}
	var dmails []DMail
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

// ConsumedRecord tracks a processed inbox D-Mail.
type ConsumedRecord struct {
	Name       string    `json:"name"`
	Kind       DMailKind `json:"kind"`
	ConsumedAt time.Time `json:"consumed_at"`
	Source     string    `json:"source"`
}

// LoadConsumed reads all consumed records from .run/consumed.json.
func (s *StateStore) LoadConsumed() ([]ConsumedRecord, error) {
	path := filepath.Join(s.Root, ".run", "consumed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []ConsumedRecord{}, nil
		}
		return nil, err
	}
	var records []ConsumedRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// SaveConsumed appends consumed records to .run/consumed.json.
func (s *StateStore) SaveConsumed(records []ConsumedRecord) error {
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
func (s *StateStore) ScanInbox() ([]DMail, error) {
	inboxDir := filepath.Join(s.Root, "inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read inbox: %w", err)
	}

	var dmails []DMail
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		inboxPath := filepath.Join(inboxDir, e.Name())
		data, err := os.ReadFile(inboxPath)
		if err != nil {
			return nil, fmt.Errorf("read inbox file %s: %w", e.Name(), err)
		}
		dmail, err := ParseDMail(data)
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
