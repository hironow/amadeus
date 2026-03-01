package amadeus_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus"
)

func TestParseDMail_Valid(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "ADR-003 violation detected"
issues:
  - MY-42
severity: high
metadata:
  created_at: "2026-02-20T12:00:00Z"
---

# ADR-003 Violation

The auth module violates the JWT requirement.
`
	dmail, err := amadeus.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != amadeus.KindFeedback {
		t.Errorf("expected kind feedback, got %s", dmail.Kind)
	}
	if dmail.Description != "ADR-003 violation detected" {
		t.Errorf("expected description, got %s", dmail.Description)
	}
	if len(dmail.Issues) != 1 || dmail.Issues[0] != "MY-42" {
		t.Errorf("expected issues [MY-42], got %v", dmail.Issues)
	}
	if dmail.Severity != amadeus.SeverityHigh {
		t.Errorf("expected severity high, got %s", dmail.Severity)
	}
	if dmail.Metadata["created_at"] != "2026-02-20T12:00:00Z" {
		t.Errorf("expected metadata created_at, got %v", dmail.Metadata)
	}
	if !strings.Contains(dmail.Body, "ADR-003 Violation") {
		t.Errorf("expected body to contain 'ADR-003 Violation', got %s", dmail.Body)
	}
}

func TestParseDMail_Minimal(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "minimal"
---
`
	dmail, err := amadeus.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != amadeus.KindFeedback {
		t.Errorf("expected kind feedback, got %s", dmail.Kind)
	}
	if len(dmail.Issues) != 0 {
		t.Errorf("expected empty issues, got %v", dmail.Issues)
	}
	if dmail.Severity != "" {
		t.Errorf("expected empty severity, got %s", dmail.Severity)
	}
}

func TestParseDMail_InvalidYAML(t *testing.T) {
	raw := `---
name: [invalid
---
`
	_, err := amadeus.ParseDMail([]byte(raw))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseDMail_MissingDelimiters(t *testing.T) {
	raw := `no frontmatter here`
	_, err := amadeus.ParseDMail([]byte(raw))
	if err == nil {
		t.Error("expected error for missing delimiters")
	}
}

func TestParseDMail_LegacyUppercaseSeverity(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "legacy uppercase severity"
severity: HIGH
---
`
	dmail, err := amadeus.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Severity != amadeus.SeverityHigh {
		t.Errorf("expected severity 'high', got %q", dmail.Severity)
	}
}

func TestParseDMail_LegacyMixedCaseSeverity(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "mixed case"
severity: Medium
---
`
	dmail, err := amadeus.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Severity != amadeus.SeverityMedium {
		t.Errorf("expected severity 'medium', got %q", dmail.Severity)
	}
}

func TestMarshalDMail_RoundTrip(t *testing.T) {
	original := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "ADR violation",
		Issues:      []string{"MY-42"},
		Severity:    amadeus.SeverityHigh,
		Metadata:    map[string]string{"created_at": "2026-02-20T12:00:00Z"},
		Body:        "# Details\n\nSome markdown content.\n",
	}

	data, err := amadeus.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	// then: raw content starts with ---
	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("expected data to start with '---', got: %s", string(data[:20]))
	}

	// round-trip
	parsed, err := amadeus.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail round-trip failed: %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("expected name %s, got %s", original.Name, parsed.Name)
	}
	if parsed.Kind != original.Kind {
		t.Errorf("expected kind %s, got %s", original.Kind, parsed.Kind)
	}
	if parsed.Description != original.Description {
		t.Errorf("expected description %s, got %s", original.Description, parsed.Description)
	}
	if len(parsed.Issues) != 1 || parsed.Issues[0] != "MY-42" {
		t.Errorf("expected issues [MY-42], got %v", parsed.Issues)
	}
	if parsed.Severity != original.Severity {
		t.Errorf("expected severity %s, got %s", original.Severity, parsed.Severity)
	}
	if !strings.Contains(parsed.Body, "Some markdown content.") {
		t.Errorf("expected body to contain 'Some markdown content.', got %s", parsed.Body)
	}
}

func TestParseDMail_WithTargets(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: feedback
description: "ADR violation"
targets:
  - auth/session.go
  - api/handler.go
---

Body text.
`
	dmail, err := amadeus.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if len(dmail.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(dmail.Targets))
	}
	if dmail.Targets[0] != "auth/session.go" {
		t.Errorf("expected first target 'auth/session.go', got %s", dmail.Targets[0])
	}
}

func TestMarshalDMail_Targets_RoundTrip(t *testing.T) {
	original := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "with targets",
		Targets:     []string{"auth/session.go", "api/handler.go"},
		Body:        "Details\n",
	}

	data, err := amadeus.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	parsed, err := amadeus.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail round-trip failed: %v", err)
	}
	if len(parsed.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(parsed.Targets))
	}
	if parsed.Targets[1] != "api/handler.go" {
		t.Errorf("expected second target 'api/handler.go', got %s", parsed.Targets[1])
	}
}

func TestMarshalDMail_OmitsEmptyTargets(t *testing.T) {
	original := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "no extras",
	}

	data, err := amadeus.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "targets") {
		t.Errorf("expected no 'targets' in output, got:\n%s", content)
	}
}

func TestValidateDMail_Valid(t *testing.T) {
	dmail := amadeus.DMail{
		SchemaVersion: amadeus.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          amadeus.KindFeedback,
		Description:   "ADR violation detected",
		Severity:      amadeus.SeverityHigh,
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateDMail_AllKinds(t *testing.T) {
	for _, kind := range []amadeus.DMailKind{amadeus.KindFeedback, amadeus.KindSpecification, amadeus.KindReport, amadeus.KindConvergence} {
		dmail := amadeus.DMail{
			SchemaVersion: amadeus.DMailSchemaVersion,
			Name:          "test-001",
			Kind:          kind,
			Description:   "test",
			Severity:      amadeus.SeverityLow,
		}
		errs := amadeus.ValidateDMail(dmail)
		if len(errs) != 0 {
			t.Errorf("kind %s: expected no errors, got %v", kind, errs)
		}
	}
}

func TestValidateDMail_MissingName(t *testing.T) {
	dmail := amadeus.DMail{
		Kind:        amadeus.KindFeedback,
		Description: "test",
		Severity:    amadeus.SeverityHigh,
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing name")
	}
}

func TestValidateDMail_MissingKind(t *testing.T) {
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Description: "test",
		Severity:    amadeus.SeverityHigh,
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing kind")
	}
}

func TestValidateDMail_InvalidKind(t *testing.T) {
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.DMailKind("invalid"),
		Description: "test",
		Severity:    amadeus.SeverityHigh,
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for invalid kind")
	}
}

func TestValidateDMail_MissingDescription(t *testing.T) {
	dmail := amadeus.DMail{
		Name:     "feedback-001",
		Kind:     amadeus.KindFeedback,
		Severity: amadeus.SeverityHigh,
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing description")
	}
}

func TestValidateDMail_MissingSeverity_IsValid(t *testing.T) {
	// severity is optional — inbox reports from external tools may omit it
	dmail := amadeus.DMail{
		SchemaVersion: amadeus.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          amadeus.KindFeedback,
		Description:   "test",
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors for missing severity, got %v", errs)
	}
}

func TestValidateDMail_InvalidSeverity(t *testing.T) {
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "test",
		Severity:    amadeus.Severity("critical"),
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for invalid severity")
	}
}

func TestValidateDMail_MultipleErrors(t *testing.T) {
	dmail := amadeus.DMail{}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors for empty DMail, got %d: %v", len(errs), errs)
	}
}

func TestValidateDMail_MissingSchemaVersion(t *testing.T) {
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "test",
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing dmail-schema-version")
	}
}

func TestValidateDMail_UnsupportedSchemaVersion(t *testing.T) {
	dmail := amadeus.DMail{
		SchemaVersion: "99",
		Name:          "feedback-001",
		Kind:          amadeus.KindFeedback,
		Description:   "test",
	}
	errs := amadeus.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for unsupported dmail-schema-version")
	}
}

func TestDMailIdempotencyKey_Deterministic(t *testing.T) {
	// given: two identical D-Mails
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "ADR violation",
		Body:        "Details here.\n",
	}

	// when
	key1 := amadeus.DMailIdempotencyKey(dmail)
	key2 := amadeus.DMailIdempotencyKey(dmail)

	// then: same input → same key
	if key1 != key2 {
		t.Errorf("idempotency key not deterministic: %q != %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("expected 64-char hex SHA256, got %d chars: %q", len(key1), key1)
	}
}

func TestDMailIdempotencyKey_DifferentContent(t *testing.T) {
	// given: two D-Mails with different bodies
	dmail1 := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "ADR violation",
		Body:        "Details v1.\n",
	}
	dmail2 := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "ADR violation",
		Body:        "Details v2.\n",
	}

	// when
	key1 := amadeus.DMailIdempotencyKey(dmail1)
	key2 := amadeus.DMailIdempotencyKey(dmail2)

	// then: different content → different key
	if key1 == key2 {
		t.Error("different content should produce different keys")
	}
}

func TestMarshalDMail_IdempotencyKey(t *testing.T) {
	// given
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "ADR violation",
		Body:        "Details here.\n",
	}

	// when
	data, err := amadeus.MarshalDMail(dmail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then: round-trip preserves idempotency_key in metadata
	parsed, err := amadeus.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	key, ok := parsed.Metadata["idempotency_key"]
	if !ok {
		t.Fatal("expected idempotency_key in metadata")
	}
	expected := amadeus.DMailIdempotencyKey(dmail)
	if key != expected {
		t.Errorf("idempotency_key: got %q, want %q", key, expected)
	}
}

func TestMarshalDMail_IdempotencyKey_PreservesExistingMetadata(t *testing.T) {
	// given: D-Mail with existing metadata
	dmail := amadeus.DMail{
		Name:        "feedback-001",
		Kind:        amadeus.KindFeedback,
		Description: "ADR violation",
		Metadata:    map[string]string{"created_at": "2026-02-28T12:00:00Z"},
	}

	// when
	data, err := amadeus.MarshalDMail(dmail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then: both metadata keys present
	parsed, err := amadeus.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	if parsed.Metadata["created_at"] != "2026-02-28T12:00:00Z" {
		t.Errorf("existing metadata lost: %v", parsed.Metadata)
	}
	if _, ok := parsed.Metadata["idempotency_key"]; !ok {
		t.Fatal("expected idempotency_key in metadata")
	}
}

func TestExtractIssueIDs_SingleID(t *testing.T) {
	// given
	text := "feat: add CollectADRs for reading ADR markdown files (MY-302)"

	// when
	ids := amadeus.ExtractIssueIDs(text)

	// then
	if len(ids) != 1 {
		t.Fatalf("expected 1 ID, got %d: %v", len(ids), ids)
	}
	if ids[0] != "MY-302" {
		t.Errorf("expected MY-302, got %s", ids[0])
	}
}

func TestExtractIssueIDs_MultipleIDsInOneText(t *testing.T) {
	// given
	text := "fix: resolve MY-241 and MY-302 conflicts"

	// when
	ids := amadeus.ExtractIssueIDs(text)

	// then
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "MY-241" || ids[1] != "MY-302" {
		t.Errorf("expected [MY-241 MY-302], got %v", ids)
	}
}

func TestExtractIssueIDs_DeduplicatesAcrossTexts(t *testing.T) {
	// given
	text1 := "feat: implement MY-302"
	text2 := "test: verify MY-302 behavior"

	// when
	ids := amadeus.ExtractIssueIDs(text1, text2)

	// then
	if len(ids) != 1 {
		t.Fatalf("expected 1 unique ID, got %d: %v", len(ids), ids)
	}
	if ids[0] != "MY-302" {
		t.Errorf("expected MY-302, got %s", ids[0])
	}
}

func TestExtractIssueIDs_NoIDs(t *testing.T) {
	// given
	text := "refactor: clean up code style"

	// when
	ids := amadeus.ExtractIssueIDs(text)

	// then
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestExtractIssueIDs_EmptyInput(t *testing.T) {
	// when
	ids := amadeus.ExtractIssueIDs()

	// then
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestExtractIssueIDs_SortedOutput(t *testing.T) {
	// given
	text := "MY-305 then MY-241 then MY-302"

	// when
	ids := amadeus.ExtractIssueIDs(text)

	// then
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d: %v", len(ids), ids)
	}
	sorted := strings.Join(ids, ",")
	if sorted != "MY-241,MY-302,MY-305" {
		t.Errorf("expected sorted order, got %s", sorted)
	}
}

func TestExtractIssueIDs_MultipleTexts(t *testing.T) {
	// given
	titles := []string{
		"80254a3 feat: CLI improvements (#5)",
		"514dd3e feat: inject ADR/DoD/DepMap (MY-302)",
		"abc1234 fix: resolve issue (MY-241)",
	}

	// when
	ids := amadeus.ExtractIssueIDs(titles...)

	// then
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "MY-241" || ids[1] != "MY-302" {
		t.Errorf("expected [MY-241 MY-302], got %v", ids)
	}
}

func TestExtractIssueIDs_NonMyPrefix(t *testing.T) {
	// given: other Linear project keys
	text := "fix: resolve AM-123 and OPS-45 issues"

	// when
	ids := amadeus.ExtractIssueIDs(text)

	// then
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "AM-123" || ids[1] != "OPS-45" {
		t.Errorf("expected [AM-123 OPS-45], got %v", ids)
	}
}

func TestExtractIssueIDs_MixedPrefixes(t *testing.T) {
	// given
	titles := []string{
		"feat: implement MY-302",
		"fix: resolve AM-99 conflict",
	}

	// when
	ids := amadeus.ExtractIssueIDs(titles...)

	// then
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "AM-99" || ids[1] != "MY-302" {
		t.Errorf("expected [AM-99 MY-302], got %v", ids)
	}
}
