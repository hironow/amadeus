package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
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
	// KindStallEscalation is produced when a wave is stalled due to repeated structural errors.
	KindStallEscalation DMailKind = "stall-escalation"
)

// ValidDMailKinds is the canonical set of valid D-Mail kinds (schema v1).
// Used by verifiers to avoid maintaining a separate copy.
var ValidDMailKinds = map[DMailKind]bool{
	KindDesignFeedback:  true,
	KindImplFeedback:    true,
	KindSpecification:   true,
	KindReport:          true,
	KindConvergence:     true,
	KindCIResult:        true,
	KindStallEscalation: true,
}

// IsValidDMailKind returns true if the kind is in the canonical set.
func IsValidDMailKind(kind DMailKind) bool {
	return ValidDMailKinds[kind]
}

// ErrDMailKindInvalid is returned when a D-Mail kind is not in the canonical set.
var ErrDMailKindInvalid = errors.New("dmail: invalid kind")

// ParseKindString parses a raw string into a DMailKind, returning the typed value or an error.
func ParseKindString(s string) (DMailKind, error) {
	kind := DMailKind(s)
	if !IsValidDMailKind(kind) {
		return "", fmt.Errorf("invalid D-Mail kind %q: %w", s, ErrDMailKindInvalid)
	}
	return kind, nil
}

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
	Wave          *WaveReference    `yaml:"wave,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	Context       *InsightContext   `yaml:"context,omitempty" json:"context,omitempty"`
}

// WaveStepDef defines a single step within a wave specification.
type WaveStepDef struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go,structure.multiple-exported-structs-go — YAML/JSON wire struct for wave spec; Targets/Prerequisites are spec list fields, not managed collections; D-Mail family (WaveStepDef/WaveReference/DMail) is a cohesive D-Mail message schema [permanent]
	ID            string   `yaml:"id" json:"id"`
	Title         string   `yaml:"title" json:"title"`
	Description   string   `yaml:"description,omitempty" json:"description,omitempty"`
	Targets       []string `yaml:"targets,omitempty" json:"targets,omitempty"`
	Acceptance    string   `yaml:"acceptance,omitempty" json:"acceptance,omitempty"`
	Prerequisites []string `yaml:"prerequisites,omitempty" json:"prerequisites,omitempty"`
}

// WaveReference links a D-Mail to a wave and optionally a specific step.
type WaveReference struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go,structure.multiple-exported-structs-go — YAML/JSON wire struct; Steps is a deserialized list from wave spec, not a managed collection; D-Mail family cohesive set; see WaveStepDef [permanent]
	ID    string        `yaml:"id" json:"id"`
	Step  string        `yaml:"step,omitempty" json:"step,omitempty"`
	Steps []WaveStepDef `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// DMail is the correction routing message using YAML frontmatter + Markdown body.
type DMail struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go,structure.multiple-exported-structs-go — YAML/JSON wire format for D-Mail message; Issues/Targets/Steps are serialized list fields from message spec; D-Mail family cohesive set; see WaveStepDef [permanent]
	SchemaVersion string            `yaml:"dmail-schema-version,omitempty"`
	Name          string            `yaml:"name"`
	Kind          DMailKind         `yaml:"kind"`
	Description   string            `yaml:"description"`
	Issues        []string          `yaml:"issues,omitempty"`
	Severity      Severity          `yaml:"severity,omitempty"`
	Action        DMailAction       `yaml:"action,omitempty"`
	Priority      int               `yaml:"priority,omitempty"`
	Targets       []string          `yaml:"targets,omitempty"`
	Wave          *WaveReference    `yaml:"wave,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	Context       *InsightContext   `yaml:"context,omitempty" json:"context,omitempty"`
	Body          string            `yaml:"-"`
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

// RequiredTargets returns the mandatory delivery targets for a given D-Mail kind.
// design-feedback MUST reach sightjack; implementation-feedback MUST reach paintress.
// This enforces the feedback loop wiring regardless of AI-generated target fields.
func RequiredTargets(kind DMailKind) []string {
	switch kind {
	case KindDesignFeedback:
		return []string{"sightjack"}
	case KindImplFeedback:
		return []string{"paintress"}
	default:
		return nil
	}
}

// MaxFeedbackRounds is the maximum number of feedback loop iterations before
// amadeus generates a convergence D-Mail instead of another feedback round.
const MaxFeedbackRounds = 3

// FeedbackRound extracts the feedback_round counter from D-Mail metadata.
// Returns 0 when absent (first generation).
func FeedbackRound(d DMail) int {
	if d.Metadata == nil {
		return 0
	}
	n, err := strconv.Atoi(d.Metadata["feedback_round"])
	if err != nil {
		return 0
	}
	return n
}

// WithFeedbackRound returns a copy of the D-Mail metadata with feedback_round set.
// Does not mutate the original.
func WithFeedbackRound(d DMail, round int) DMail {
	meta := make(map[string]string, len(d.Metadata)+1)
	for k, v := range d.Metadata {
		meta[k] = v
	}
	meta["feedback_round"] = strconv.Itoa(round)
	d.Metadata = meta
	return d
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
		Wave:          fm.Wave,
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
	h.Write([]byte{0})
	// Include wave reference to distinguish D-Mails targeting different waves
	if dmail.Wave != nil {
		h.Write([]byte(dmail.Wave.ID))
		h.Write([]byte{0})
		h.Write([]byte(dmail.Wave.Step))
		for _, step := range dmail.Wave.Steps {
			h.Write([]byte{0})
			h.Write([]byte(step.ID))
			h.Write([]byte{0})
			h.Write([]byte(step.Title))
			h.Write([]byte{0})
			h.Write([]byte(step.Acceptance))
		}
	}
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
		Wave:          dmail.Wave,
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
