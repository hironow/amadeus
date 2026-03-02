package amadeus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
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
	// KindCIResult is produced by CI/CD pipeline integrations.
	KindCIResult DMailKind = "ci-result"
)

// DMailAction represents a recommended follow-up action for a D-Mail.
type DMailAction string

const (
	// ActionRetry indicates the operation should be retried.
	ActionRetry DMailAction = "retry"
	// ActionEscalate indicates the issue should be escalated.
	ActionEscalate DMailAction = "escalate"
	// ActionResolve indicates the issue can be resolved.
	ActionResolve DMailAction = "resolve"
)

// validActions is the set of valid DMailAction values per schema v1.
var validActions = map[DMailAction]bool{
	ActionRetry:    true,
	ActionEscalate: true,
	ActionResolve:  true,
}

// DMailStatus represents the lifecycle status of a D-Mail.
type DMailStatus string

const (
	// DMailSent indicates a D-Mail that has been auto-sent.
	DMailSent DMailStatus = "sent"
)

// DMailSchemaVersion is the current D-Mail protocol schema version.
const DMailSchemaVersion = "1"

// dmailFrontmatter is the YAML frontmatter of a D-Mail file.
// NOTE(MY-346): linear_issue_id was intentionally removed without migration.
// Existing D-Mail files with linear_issue_id will lose that field on parse.
// This is acceptable because amadeus is pre-release and no production .gate/ state exists.
type dmailFrontmatter struct {
	SchemaVersion string            `yaml:"dmail-schema-version"`
	Name          string            `yaml:"name"`
	Kind          DMailKind         `yaml:"kind"`
	Description   string            `yaml:"description"`
	Issues        []string          `yaml:"issues,omitempty"`
	Severity      Severity          `yaml:"severity,omitempty"`
	Action        DMailAction       `yaml:"action,omitempty"`
	Priority      int               `yaml:"priority,omitempty"`
	Targets       []string          `yaml:"targets,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
}

// DMail is the correction routing message using YAML frontmatter + Markdown body.
type DMail struct {
	SchemaVersion string            `yaml:"dmail-schema-version,omitempty"`
	Name          string            `yaml:"name"`
	Kind          DMailKind         `yaml:"kind"`
	Description   string            `yaml:"description"`
	Issues        []string          `yaml:"issues,omitempty"`
	Severity      Severity          `yaml:"severity,omitempty"`
	Action        DMailAction       `yaml:"action,omitempty"`
	Priority      int               `yaml:"priority,omitempty"`
	Targets       []string          `yaml:"targets,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	Body          string            `yaml:"-"`
}

// validKinds is the set of valid DMailKind values per schema v1.
var validKinds = map[DMailKind]bool{
	KindFeedback:      true,
	KindSpecification: true,
	KindReport:        true,
	KindConvergence:   true,
	KindCIResult:      true,
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
	if dmail.SchemaVersion == "" {
		errs = append(errs, "dmail-schema-version is required")
	} else if dmail.SchemaVersion != DMailSchemaVersion {
		errs = append(errs, fmt.Sprintf("unsupported dmail-schema-version: %q (want %q)", dmail.SchemaVersion, DMailSchemaVersion))
	}
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
	if dmail.Action != "" && !validActions[dmail.Action] {
		errs = append(errs, fmt.Sprintf("invalid action %q", dmail.Action))
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
		SchemaVersion: fm.SchemaVersion,
		Name:          fm.Name,
		Kind:          fm.Kind,
		Description:   fm.Description,
		Issues:        fm.Issues,
		Severity:      NormalizeSeverity(fm.Severity),
		Action:        fm.Action,
		Priority:      fm.Priority,
		Targets:       fm.Targets,
		Metadata:      fm.Metadata,
		Body:          strings.TrimLeft(bodyPart, "\n"),
	}, nil
}

// DMailIdempotencyKey computes a SHA256 content-based idempotency key from
// the core fields of a DMail (name, kind, description, body).
func DMailIdempotencyKey(dmail DMail) string {
	h := sha256.New()
	h.Write([]byte(dmail.Name))
	h.Write([]byte{0})
	h.Write([]byte(string(dmail.Kind)))
	h.Write([]byte{0})
	h.Write([]byte(dmail.Description))
	h.Write([]byte{0})
	h.Write([]byte(dmail.Body))
	return hex.EncodeToString(h.Sum(nil))
}

// MarshalDMail serializes a DMail to YAML frontmatter + Markdown format.
// Automatically injects an idempotency_key into metadata based on content hash.
func MarshalDMail(dmail DMail) ([]byte, error) {
	meta := make(map[string]string, len(dmail.Metadata)+1)
	for k, v := range dmail.Metadata {
		meta[k] = v
	}
	meta["idempotency_key"] = DMailIdempotencyKey(dmail)

	fm := dmailFrontmatter{
		SchemaVersion: dmail.SchemaVersion,
		Name:          dmail.Name,
		Kind:          dmail.Kind,
		Description:   dmail.Description,
		Issues:        dmail.Issues,
		Severity:      dmail.Severity,
		Action:        dmail.Action,
		Priority:      dmail.Priority,
		Targets:       dmail.Targets,
		Metadata:      meta,
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

// ConsumedRecord tracks a processed inbox D-Mail.
type ConsumedRecord struct {
	Name       string    `json:"name"`
	Kind       DMailKind `json:"kind"`
	ConsumedAt time.Time `json:"consumed_at"`
	Source     string    `json:"source"`
}

var issueIDPattern = regexp.MustCompile(`[A-Z]+-\d+`)

// ExtractIssueIDs scans texts for Linear Issue IDs (e.g. "MY-302", "AM-123")
// and returns a unique, sorted list.
func ExtractIssueIDs(texts ...string) []string {
	seen := make(map[string]bool)
	for _, text := range texts {
		for _, id := range issueIDPattern.FindAllString(text, -1) {
			seen[id] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
