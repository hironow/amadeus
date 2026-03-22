package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DMailKind represents the type of D-Mail message.
type DMailKind string

const (
	// KindDesignFeedback is produced by the verifier role for design-level issues.
	KindDesignFeedback DMailKind = "design-feedback"
	// KindImplFeedback is produced by the verifier role for implementation-level issues.
	KindImplFeedback DMailKind = "implementation-feedback"
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
// DECISION(MY-346): linear_issue_id field was removed in favor of Issues []string.
// Old D-Mail files with linear_issue_id silently drop that field on parse.
// This is a finalized non-backward-compatible change; no migration is provided.
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
	Context       *InsightContext   `yaml:"context,omitempty" json:"context,omitempty"`
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
	Context       *InsightContext   `yaml:"context,omitempty" json:"context,omitempty"`
	Body          string            `yaml:"-"`
}

// validKinds is the set of valid DMailKind values per schema v1.
var validKinds = map[DMailKind]bool{
	KindDesignFeedback: true,
	KindImplFeedback:   true,
	KindSpecification:  true,
	KindReport:         true,
	KindConvergence:    true,
	KindCIResult:       true,
}

// validSeverities is the set of valid Severity values per schema v1.
var validSeverities = map[Severity]bool{
	SeverityLow:    true,
	SeverityMedium: true,
	SeverityHigh:   true,
}

// DefaultDMailAction returns the default DMailAction for a given severity.
// Used when a D-Mail candidate does not specify an explicit action.
func DefaultDMailAction(severity Severity) DMailAction {
	switch severity {
	case SeverityHigh:
		return ActionEscalate
	case SeverityMedium:
		return ActionRetry
	default:
		return ActionResolve
	}
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
	if strings.TrimSpace(dmail.Body) == "" {
		errs = append(errs, "body is required")
	}
	errs = append(errs, validateTargets(dmail.Targets)...)
	return errs
}

// validateTargets checks D-Mail targets for path traversal and duplicates.
func validateTargets(targets []string) []string {
	var errs []string
	seen := make(map[string]bool)
	for _, target := range targets {
		if strings.TrimSpace(target) == "" {
			errs = append(errs, "target must not be empty")
			continue
		}
		if filepath.IsAbs(target) {
			errs = append(errs, fmt.Sprintf("target %q must be a relative path", target))
			continue
		}
		if containsDotDotElement(target) {
			errs = append(errs, fmt.Sprintf("target %q contains path traversal", target))
			continue
		}
		if seen[target] {
			errs = append(errs, fmt.Sprintf("duplicate target %q", target))
			continue
		}
		seen[target] = true
	}
	return errs
}

// containsDotDotElement reports whether the path contains ".." as a path element
// (e.g. "../foo" or "foo/../bar") rather than as a substring (e.g. "foo..bar").
func containsDotDotElement(path string) bool {
	for _, elem := range strings.Split(filepath.ToSlash(path), "/") {
		if elem == ".." {
			return true
		}
	}
	return false
}

// SanitizeTargets removes self-referencing targets from a D-Mail's target list.
// It filters out targets that match the sender identity or the kind string prefix,
// preventing routing loops where a D-Mail targets itself.
func SanitizeTargets(senderIdentity string, kind DMailKind, targets []string) []string {
	var result []string
	for _, target := range targets {
		if target == senderIdentity {
			continue
		}
		if target == string(kind) {
			continue
		}
		result = append(result, target)
	}
	return result
}

// splitFrontmatter splits raw D-Mail bytes into the YAML frontmatter bytes and Markdown body string.
// Returns an error if the opening or closing frontmatter delimiters are missing.
func splitFrontmatter(data []byte) (yamlPart []byte, bodyPart string, err error) {
	str := string(data)
	if !strings.HasPrefix(str, "---\n") {
		return nil, "", fmt.Errorf("missing opening frontmatter delimiter")
	}
	rest := str[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, "", fmt.Errorf("missing closing frontmatter delimiter")
	}
	return []byte(rest[:idx]), rest[idx+5:], nil // skip "\n---\n"
}

// fmToDMail converts a parsed dmailFrontmatter and body into a DMail.
func fmToDMail(fm dmailFrontmatter, bodyPart string) DMail {
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
		Context:       fm.Context,
		Body:          strings.TrimLeft(bodyPart, "\n"),
	}
}

// ParseDMail parses a D-Mail from raw bytes in YAML frontmatter + Markdown format.
func ParseDMail(data []byte) (DMail, error) {
	yamlPart, bodyPart, err := splitFrontmatter(data)
	if err != nil {
		return DMail{}, err
	}

	var fm dmailFrontmatter
	if err := yaml.Unmarshal(yamlPart, &fm); err != nil {
		return DMail{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	return fmToDMail(fm, bodyPart), nil
}

// ParseDMailStrict parses a D-Mail from raw bytes like ParseDMail, but rejects unknown frontmatter fields.
// Use this variant in trusted pipelines where strict schema conformance is required.
func ParseDMailStrict(data []byte) (DMail, error) {
	yamlPart, bodyPart, err := splitFrontmatter(data)
	if err != nil {
		return DMail{}, err
	}

	dec := yaml.NewDecoder(bytes.NewReader(yamlPart))
	dec.KnownFields(true)

	var fm dmailFrontmatter
	if err := dec.Decode(&fm); err != nil {
		return DMail{}, fmt.Errorf("parse frontmatter (strict): %w", err)
	}

	return fmToDMail(fm, bodyPart), nil
}

// DMailIdempotencyKey computes a SHA256 content-based idempotency key from
// the core fields of a DMail (name, kind, description, body, issues, severity).
func DMailIdempotencyKey(dmail DMail) string {
	h := sha256.New()
	h.Write([]byte(dmail.Name))
	h.Write([]byte{0})
	h.Write([]byte(string(dmail.Kind)))
	h.Write([]byte{0})
	h.Write([]byte(dmail.Description))
	h.Write([]byte{0})
	h.Write([]byte(dmail.Body))
	h.Write([]byte{0})
	// Include sorted issues to prevent collision when same content has different issues
	issuesCopy := make([]string, len(dmail.Issues))
	copy(issuesCopy, dmail.Issues)
	sort.Strings(issuesCopy)
	h.Write([]byte(strings.Join(issuesCopy, ",")))
	h.Write([]byte{0})
	h.Write([]byte(string(dmail.Severity)))
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
		Context:       dmail.Context,
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

// ErrIdempotencyMismatch is returned by VerifyIdempotencyKey when the stored
// idempotency_key in metadata does not match the recomputed key for the DMail.
var ErrIdempotencyMismatch = fmt.Errorf("idempotency key mismatch")

// VerifyIdempotencyKey checks that the idempotency_key stored in a DMail's
// metadata matches the key recomputed from its current content.
// Returns nil when no key is present (nil Metadata or empty string value).
// Returns ErrIdempotencyMismatch when the keys differ.
func VerifyIdempotencyKey(dmail DMail) error {
	if dmail.Metadata == nil {
		return nil
	}
	stored, ok := dmail.Metadata["idempotency_key"]
	if !ok || stored == "" {
		return nil
	}
	computed := DMailIdempotencyKey(dmail)
	prefixLen := min(16, len(stored))
	if stored[:prefixLen] != computed[:prefixLen] {
		return ErrIdempotencyMismatch
	}
	if stored != computed {
		return ErrIdempotencyMismatch
	}
	return nil
}

// ConsumedRecord tracks a processed inbox D-Mail.
type ConsumedRecord struct {
	Name       string    `json:"name"`
	Kind       DMailKind `json:"kind"`
	ConsumedAt time.Time `json:"consumed_at"`
	Source     string    `json:"source"`
}

// dmailTTL is the maximum age of a D-Mail before it is considered stale and excluded from
// convergence analysis. D-Mails older than this duration accumulate noise that degrades detection accuracy.
const dmailTTL = 7 * 24 * time.Hour

// DMailAge computes the age of a D-Mail from its Metadata["created_at"] field (RFC3339).
// Returns (age, true) when the timestamp is present and parseable, or (0, false) otherwise.
func DMailAge(dmail DMail, now time.Time) (time.Duration, bool) {
	if dmail.Metadata == nil {
		return 0, false
	}
	raw, ok := dmail.Metadata["created_at"]
	if !ok || raw == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, false
	}
	return now.Sub(t), true
}

// FilterByTTL excludes D-Mails older than the hardcoded TTL from the given slice.
// Entries with missing or unparseable created_at timestamps are conservatively included.
func FilterByTTL(dmails []DMail, now time.Time) []DMail {
	result := make([]DMail, 0, len(dmails))
	for _, d := range dmails {
		age, ok := DMailAge(d, now)
		if ok && age > dmailTTL {
			continue
		}
		result = append(result, d)
	}
	return result
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
