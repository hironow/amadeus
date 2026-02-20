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
)

// DMailStatus represents the lifecycle status of a D-Mail.
type DMailStatus string

const (
	// DMailPending indicates a D-Mail awaiting human approval.
	DMailPending DMailStatus = "pending"
	// DMailSent indicates a D-Mail that has been auto-sent.
	DMailSent DMailStatus = "sent"
	// DMailApproved indicates a D-Mail approved by a human.
	DMailApproved DMailStatus = "approved"
	// DMailRejected indicates a D-Mail rejected by a human.
	DMailRejected DMailStatus = "rejected"
)

// dmailFrontmatter is the YAML frontmatter of a D-Mail file.
type dmailFrontmatter struct {
	Name        string            `yaml:"name"`
	Kind        DMailKind         `yaml:"kind"`
	Description string            `yaml:"description"`
	Issues      []string          `yaml:"issues,omitempty"`
	Severity    Severity          `yaml:"severity,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// DMail is the correction routing message using YAML frontmatter + Markdown body.
type DMail struct {
	Name        string            `yaml:"name"`
	Kind        DMailKind         `yaml:"kind"`
	Description string            `yaml:"description"`
	Issues      []string          `yaml:"issues,omitempty"`
	Severity    Severity          `yaml:"severity,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
	Body        string            `yaml:"-"`
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

// RouteDMail applies severity-based status mapping.
// HIGH severity requires human approval (pending); all others are auto-sent.
func RouteDMail(severity Severity) DMailStatus {
	if severity == SeverityHigh {
		return DMailPending
	}
	return DMailSent
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

// SaveDMail writes a D-Mail to both outbox/ and archive/ (dual-write pattern).
func (s *StateStore) SaveDMail(dmail DMail) error {
	data, err := MarshalDMail(dmail)
	if err != nil {
		return fmt.Errorf("marshal dmail: %w", err)
	}
	filename := dmail.Name + ".md"
	outboxPath := filepath.Join(s.Root, "outbox", filename)
	archivePath := filepath.Join(s.Root, "archive", filename)
	if err := os.WriteFile(outboxPath, data, 0o644); err != nil {
		return fmt.Errorf("write outbox: %w", err)
	}
	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return fmt.Errorf("write archive: %w", err)
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

// ErrNoResolution is returned when no resolution exists for a given D-Mail name.
var ErrNoResolution = errors.New("no resolution found")

// Resolution tracks the approval state of a D-Mail, stored as a sidecar file
// in .run/resolutions.json. The D-Mail .md file itself is immutable.
type Resolution struct {
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Action     string     `json:"action,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// LoadResolutions reads all resolutions from .run/resolutions.json.
func (s *StateStore) LoadResolutions() (map[string]Resolution, error) {
	path := filepath.Join(s.Root, ".run", "resolutions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return make(map[string]Resolution), nil
		}
		return nil, err
	}
	var resolutions map[string]Resolution
	if err := json.Unmarshal(data, &resolutions); err != nil {
		return nil, err
	}
	return resolutions, nil
}

// LoadResolution reads a single resolution by D-Mail name.
func (s *StateStore) LoadResolution(name string) (Resolution, error) {
	resolutions, err := s.LoadResolutions()
	if err != nil {
		return Resolution{}, err
	}
	res, ok := resolutions[name]
	if !ok {
		return Resolution{}, fmt.Errorf("%w: %s", ErrNoResolution, name)
	}
	return res, nil
}

// SaveResolution writes or updates a resolution in .run/resolutions.json.
func (s *StateStore) SaveResolution(res Resolution) error {
	resolutions, err := s.LoadResolutions()
	if err != nil {
		return err
	}
	resolutions[res.Name] = res
	return s.writeJSON(filepath.Join(s.Root, ".run", "resolutions.json"), resolutions)
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
