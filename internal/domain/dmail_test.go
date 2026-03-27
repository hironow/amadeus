package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestParseDMail_Valid(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
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
	dmail, err := domain.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != domain.KindDesignFeedback {
		t.Errorf("expected kind design-feedback, got %s", dmail.Kind)
	}
	if dmail.Description != "ADR-003 violation detected" {
		t.Errorf("expected description, got %s", dmail.Description)
	}
	if len(dmail.Issues) != 1 || dmail.Issues[0] != "MY-42" {
		t.Errorf("expected issues [MY-42], got %v", dmail.Issues)
	}
	if dmail.Severity != domain.SeverityHigh {
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
kind: design-feedback
description: "minimal"
---
`
	dmail, err := domain.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != domain.KindDesignFeedback {
		t.Errorf("expected kind design-feedback, got %s", dmail.Kind)
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
	_, err := domain.ParseDMail([]byte(raw))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseDMail_MissingDelimiters(t *testing.T) {
	raw := `no frontmatter here`
	_, err := domain.ParseDMail([]byte(raw))
	if err == nil {
		t.Error("expected error for missing delimiters")
	}
}

func TestParseDMail_LegacyUppercaseSeverity(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "legacy uppercase severity"
severity: HIGH
---
`
	dmail, err := domain.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Severity != domain.SeverityHigh {
		t.Errorf("expected severity 'high', got %q", dmail.Severity)
	}
}

func TestParseDMail_LegacyMixedCaseSeverity(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "mixed case"
severity: Medium
---
`
	dmail, err := domain.ParseDMail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if dmail.Severity != domain.SeverityMedium {
		t.Errorf("expected severity 'medium', got %q", dmail.Severity)
	}
}

func TestMarshalDMail_RoundTrip(t *testing.T) {
	original := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Issues:      []string{"MY-42"},
		Severity:    domain.SeverityHigh,
		Metadata:    map[string]string{"created_at": "2026-02-20T12:00:00Z"},
		Body:        "# Details\n\nSome markdown content.\n",
	}

	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	// then: raw content starts with ---
	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("expected data to start with '---', got: %s", string(data[:20]))
	}

	// round-trip
	parsed, err := domain.ParseDMail(data)
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
kind: design-feedback
description: "ADR violation"
targets:
  - auth/session.go
  - api/handler.go
---

Body text.
`
	dmail, err := domain.ParseDMail([]byte(raw))
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
	original := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "with targets",
		Targets:     []string{"auth/session.go", "api/handler.go"},
		Body:        "Details\n",
	}

	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
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
	original := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "no extras",
	}

	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "targets") {
		t.Errorf("expected no 'targets' in output, got:\n%s", content)
	}
}

func TestValidateDMail_Valid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "ADR violation detected",
		Severity:      domain.SeverityHigh,
		Body:          "Details.\n",
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateDMail_AllKinds(t *testing.T) {
	for _, kind := range []domain.DMailKind{domain.KindDesignFeedback, domain.KindImplFeedback, domain.KindSpecification, domain.KindReport, domain.KindConvergence, domain.KindCIResult} {
		dmail := domain.DMail{
			SchemaVersion: domain.DMailSchemaVersion,
			Name:          "test-001",
			Kind:          kind,
			Description:   "test",
			Severity:      domain.SeverityLow,
			Body:          "Content.\n",
		}
		errs := domain.ValidateDMail(dmail)
		if len(errs) != 0 {
			t.Errorf("kind %s: expected no errors, got %v", kind, errs)
		}
	}
}

func TestValidateDMail_MissingName(t *testing.T) {
	dmail := domain.DMail{
		Kind:        domain.KindDesignFeedback,
		Description: "test",
		Severity:    domain.SeverityHigh,
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing name")
	}
}

func TestValidateDMail_MissingKind(t *testing.T) {
	dmail := domain.DMail{
		Name:        "feedback-001",
		Description: "test",
		Severity:    domain.SeverityHigh,
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing kind")
	}
}

func TestValidateDMail_InvalidKind(t *testing.T) {
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.DMailKind("invalid"),
		Description: "test",
		Severity:    domain.SeverityHigh,
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for invalid kind")
	}
}

func TestValidateDMail_MissingDescription(t *testing.T) {
	dmail := domain.DMail{
		Name:     "feedback-001",
		Kind:     domain.KindDesignFeedback,
		Severity: domain.SeverityHigh,
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing description")
	}
}

func TestValidateDMail_MissingSeverity_IsValid(t *testing.T) {
	// severity is optional — inbox reports from external tools may omit it
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors for missing severity, got %v", errs)
	}
}

func TestValidateDMail_InvalidSeverity(t *testing.T) {
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "test",
		Severity:    domain.Severity("critical"),
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for invalid severity")
	}
}

func TestValidateDMail_MultipleErrors(t *testing.T) {
	dmail := domain.DMail{}
	errs := domain.ValidateDMail(dmail)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors for empty DMail, got %d: %v", len(errs), errs)
	}
}

func TestValidateDMail_MissingSchemaVersion(t *testing.T) {
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "test",
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for missing dmail-schema-version")
	}
}

func TestValidateDMail_UnsupportedSchemaVersion(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: "99",
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for unsupported dmail-schema-version")
	}
}

func TestDMailIdempotencyKey_Deterministic(t *testing.T) {
	// given: two identical D-Mails
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Body:        "Details here.\n",
	}

	// when
	key1 := domain.DMailIdempotencyKey(dmail)
	key2 := domain.DMailIdempotencyKey(dmail)

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
	dmail1 := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Body:        "Details v1.\n",
	}
	dmail2 := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Body:        "Details v2.\n",
	}

	// when
	key1 := domain.DMailIdempotencyKey(dmail1)
	key2 := domain.DMailIdempotencyKey(dmail2)

	// then: different content → different key
	if key1 == key2 {
		t.Error("different content should produce different keys")
	}
}

func TestMarshalDMail_IdempotencyKey(t *testing.T) {
	// given
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Body:        "Details here.\n",
	}

	// when
	data, err := domain.MarshalDMail(dmail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then: round-trip preserves idempotency_key in metadata
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	key, ok := parsed.Metadata["idempotency_key"]
	if !ok {
		t.Fatal("expected idempotency_key in metadata")
	}
	expected := domain.DMailIdempotencyKey(dmail)
	if key != expected {
		t.Errorf("idempotency_key: got %q, want %q", key, expected)
	}
}

func TestMarshalDMail_IdempotencyKey_PreservesExistingMetadata(t *testing.T) {
	// given: D-Mail with existing metadata
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Metadata:    map[string]string{"created_at": "2026-02-28T12:00:00Z"},
	}

	// when
	data, err := domain.MarshalDMail(dmail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then: both metadata keys present
	parsed, err := domain.ParseDMail(data)
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
	ids := domain.ExtractIssueIDs(text)

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
	ids := domain.ExtractIssueIDs(text)

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
	ids := domain.ExtractIssueIDs(text1, text2)

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
	ids := domain.ExtractIssueIDs(text)

	// then
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestExtractIssueIDs_EmptyInput(t *testing.T) {
	// when
	ids := domain.ExtractIssueIDs()

	// then
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestExtractIssueIDs_SortedOutput(t *testing.T) {
	// given
	text := "MY-305 then MY-241 then MY-302"

	// when
	ids := domain.ExtractIssueIDs(text)

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
	ids := domain.ExtractIssueIDs(titles...)

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
	ids := domain.ExtractIssueIDs(text)

	// then
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "AM-123" || ids[1] != "OPS-45" {
		t.Errorf("expected [AM-123 OPS-45], got %v", ids)
	}
}

func TestValidateDMail_CIResultKind(t *testing.T) {
	// given: D-Mail with ci-result kind
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "ci-result-pr42-run1",
		Kind:          domain.KindCIResult,
		Description:   "GitHub Actions CI run for PR #42",
		Body:          "CI results.\n",
	}

	// when
	errs := domain.ValidateDMail(dmail)

	// then
	if len(errs) != 0 {
		t.Errorf("expected no errors for ci-result kind, got %v", errs)
	}
}

func TestParseDMail_ActionField_RoundTrip(t *testing.T) {
	// given: D-Mail with action field
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-action-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "Evaluation with retry action",
		Action:        domain.ActionRetry,
		Body:          "Implementation needs revision.\n",
	}

	// when: marshal then parse
	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail round-trip failed: %v", err)
	}

	// then: action field preserved
	if parsed.Action != domain.ActionRetry {
		t.Errorf("expected action %q, got %q", domain.ActionRetry, parsed.Action)
	}
}

func TestValidateDMail_InvalidAction(t *testing.T) {
	// given: D-Mail with invalid action
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Action:        domain.DMailAction("invalid-action"),
	}

	// when
	errs := domain.ValidateDMail(dmail)

	// then
	if len(errs) == 0 {
		t.Error("expected error for invalid action")
	}
	found := false
	for _, e := range errs {
		if e == `invalid action "invalid-action"` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid action error message, got %v", errs)
	}
}

func TestValidateDMail_EmptyAction_IsValid(t *testing.T) {
	// given: D-Mail without action (action is optional)
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
	}

	// when
	errs := domain.ValidateDMail(dmail)

	// then
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty action, got %v", errs)
	}
}

func TestValidateDMail_AllActions(t *testing.T) {
	for _, action := range []domain.DMailAction{domain.ActionRetry, domain.ActionEscalate, domain.ActionResolve} {
		dmail := domain.DMail{
			SchemaVersion: domain.DMailSchemaVersion,
			Name:          "test-001",
			Kind:          domain.KindDesignFeedback,
			Description:   "test",
			Action:        action,
			Body:          "Content.\n",
		}
		errs := domain.ValidateDMail(dmail)
		if len(errs) != 0 {
			t.Errorf("action %s: expected no errors, got %v", action, errs)
		}
	}
}

func TestParseDMail_PriorityField_RoundTrip(t *testing.T) {
	// given: D-Mail with priority field
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "spec-priority-001",
		Kind:          domain.KindSpecification,
		Description:   "High priority specification",
		Priority:      2,
		Body:          "Implement authentication module.\n",
	}

	// when: marshal then parse
	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail round-trip failed: %v", err)
	}

	// then: priority field preserved
	if parsed.Priority != 2 {
		t.Errorf("expected priority 2, got %d", parsed.Priority)
	}
}

func TestParseDMail_ZeroPriority_OmittedInMarshal(t *testing.T) {
	// given: D-Mail without priority (zero value)
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test without prio field",
	}

	// when
	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	// then: "priority:" YAML key not present in output
	content := string(data)
	if strings.Contains(content, "priority:") {
		t.Errorf("expected no 'priority:' key in output for zero value, got:\n%s", content)
	}
}

// MY-346: legacy linear_issue_id field is silently dropped on parse.
// This is a finalized non-backward-compatible change: no migration is provided.
func TestParseDMail_LegacyLinearIssueID_SilentDrop(t *testing.T) {
	// given: a D-Mail with the removed linear_issue_id field
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "legacy format with linear_issue_id"
linear_issue_id: "MY-42"
---

Body text.
`
	// when
	dmail, err := domain.ParseDMail([]byte(raw))

	// then: parse succeeds, linear_issue_id is silently dropped
	if err != nil {
		t.Fatalf("ParseDMail should not error on legacy linear_issue_id: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	// Issues should be empty (linear_issue_id is not migrated to Issues)
	if len(dmail.Issues) != 0 {
		t.Errorf("expected empty issues (linear_issue_id should be dropped), got %v", dmail.Issues)
	}
}

// MY-346: new Issues[] field coexists with legacy linear_issue_id gracefully.
// If both are present, only Issues[] is used.
func TestParseDMail_LegacyLinearIssueID_WithNewIssues(t *testing.T) {
	// given: a D-Mail with both old and new fields
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "both old and new fields"
linear_issue_id: "MY-42"
issues:
  - MY-303
---

Body text.
`
	// when
	dmail, err := domain.ParseDMail([]byte(raw))

	// then: parse succeeds, only Issues[] is populated
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}
	if len(dmail.Issues) != 1 || dmail.Issues[0] != "MY-303" {
		t.Errorf("expected issues [MY-303], got %v", dmail.Issues)
	}
}

func TestExtractIssueIDs_MixedPrefixes(t *testing.T) {
	// given
	titles := []string{
		"feat: implement MY-302",
		"fix: resolve AM-99 conflict",
	}

	// when
	ids := domain.ExtractIssueIDs(titles...)

	// then
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "AM-99" || ids[1] != "MY-302" {
		t.Errorf("expected [AM-99 MY-302], got %v", ids)
	}
}

func TestMarshalDMail_ContextRoundTrip(t *testing.T) {
	// given
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-context-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "ADR violation with insight context",
		Context: &domain.InsightContext{
			Insights: []domain.InsightSummary{
				{Source: "amadeus", Summary: "Divergence score exceeds threshold"},
				{Source: "sightjack", Summary: "Shibito count reduced to 3"},
			},
		},
		Body: "# Details\n\nSome markdown content.\n",
	}

	// when
	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail round-trip failed: %v", err)
	}

	// then
	if parsed.Context == nil {
		t.Fatal("expected non-nil Context after round-trip")
	}
	if len(parsed.Context.Insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(parsed.Context.Insights))
	}
	if parsed.Context.Insights[0].Source != "amadeus" {
		t.Errorf("insight[0].Source = %q, want %q", parsed.Context.Insights[0].Source, "amadeus")
	}
	if parsed.Context.Insights[0].Summary != "Divergence score exceeds threshold" {
		t.Errorf("insight[0].Summary = %q, want %q", parsed.Context.Insights[0].Summary, "Divergence score exceeds threshold")
	}
	if parsed.Context.Insights[1].Source != "sightjack" {
		t.Errorf("insight[1].Source = %q, want %q", parsed.Context.Insights[1].Source, "sightjack")
	}
}

func TestMarshalDMail_NilContextOmitted(t *testing.T) {
	// given — DMail with nil Context
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-no-context",
		Kind:          domain.KindDesignFeedback,
		Description:   "no context",
	}

	// when
	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	// then — context should not appear in output
	if strings.Contains(string(data), "context:") {
		t.Error("nil Context should be omitted from marshalled output")
	}
}

func TestValidateDMail_EmptyBody_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
	}
	errs := domain.ValidateDMail(dmail)
	found := false
	for _, e := range errs {
		if e == "body is required" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'body is required' error, got %v", errs)
	}
}

func TestValidateDMail_WhitespaceOnlyBody_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "   \n\t  ",
	}
	errs := domain.ValidateDMail(dmail)
	found := false
	for _, e := range errs {
		if e == "body is required" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'body is required' error for whitespace-only body, got %v", errs)
	}
}

func TestValidateDMail_NonEmptyBody_IsValid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "# Details\n\nSome content.\n",
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors for non-empty body, got %v", errs)
	}
}

func TestValidateDMail_PathTraversal_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
		Targets:       []string{"../../etc/passwd"},
	}
	errs := domain.ValidateDMail(dmail)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "path traversal") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected path traversal error, got %v", errs)
	}
}

func TestValidateDMail_AbsoluteTarget_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
		Targets:       []string{"/etc/passwd"},
	}
	errs := domain.ValidateDMail(dmail)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "relative path") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected relative path error, got %v", errs)
	}
}

func TestValidateDMail_DuplicateTargets_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
		Targets:       []string{"auth/session.go", "auth/session.go"},
	}
	errs := domain.ValidateDMail(dmail)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicate target") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate target error, got %v", errs)
	}
}

func TestValidateDMail_EmptyTarget_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
		Targets:       []string{""},
	}
	errs := domain.ValidateDMail(dmail)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "target must not be empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected empty target error, got %v", errs)
	}
}

func TestValidateDMail_ValidTargets(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
		Targets:       []string{"auth/session.go", "api/handler.go"},
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid targets, got %v", errs)
	}
}

func TestValidateDMail_NoTargets_IsValid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Body:          "Content.\n",
	}
	errs := domain.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors for no targets, got %v", errs)
	}
}

func TestDMailIdempotencyKey_DifferentIssues_DifferentKey(t *testing.T) {
	dmail1 := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "same body.\n",
		Issues: []string{"MY-42"},
	}
	dmail2 := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "same body.\n",
		Issues: []string{"MY-99"},
	}
	key1 := domain.DMailIdempotencyKey(dmail1)
	key2 := domain.DMailIdempotencyKey(dmail2)
	if key1 == key2 {
		t.Error("different issues should produce different keys")
	}
}

func TestDMailIdempotencyKey_DifferentSeverity_DifferentKey(t *testing.T) {
	dmail1 := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "same body.\n",
		Severity: domain.SeverityLow,
	}
	dmail2 := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "same body.\n",
		Severity: domain.SeverityHigh,
	}
	key1 := domain.DMailIdempotencyKey(dmail1)
	key2 := domain.DMailIdempotencyKey(dmail2)
	if key1 == key2 {
		t.Error("different severity should produce different keys")
	}
}

func TestDMailIdempotencyKey_IssueOrderIndependent(t *testing.T) {
	dmail1 := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "same.\n",
		Issues: []string{"MY-42", "MY-99"},
	}
	dmail2 := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "same.\n",
		Issues: []string{"MY-99", "MY-42"},
	}
	key1 := domain.DMailIdempotencyKey(dmail1)
	key2 := domain.DMailIdempotencyKey(dmail2)
	if key1 != key2 {
		t.Error("same issues in different order should produce same key")
	}
}

func TestDMailIdempotencyKey_DoesNotMutateIssues(t *testing.T) {
	issues := []string{"MY-99", "MY-42"}
	dmail := domain.DMail{
		Name: "feedback-001", Kind: domain.KindDesignFeedback,
		Description: "same", Body: "body.\n",
		Issues: issues,
	}
	domain.DMailIdempotencyKey(dmail)
	if issues[0] != "MY-99" || issues[1] != "MY-42" {
		t.Error("original issues slice was mutated")
	}
}

func TestSanitizeTargets_RemovesSelfReference(t *testing.T) {
	targets := domain.SanitizeTargets("amadeus", domain.KindDesignFeedback, []string{"auth/session.go", "amadeus", "api/handler.go"})
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "auth/session.go" || targets[1] != "api/handler.go" {
		t.Errorf("unexpected targets: %v", targets)
	}
}

func TestSanitizeTargets_RemovesKindPrefix(t *testing.T) {
	targets := domain.SanitizeTargets("amadeus", domain.KindDesignFeedback, []string{"design-feedback", "auth/session.go"})
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %v", len(targets), targets)
	}
	if targets[0] != "auth/session.go" {
		t.Errorf("unexpected target: %v", targets)
	}
}

func TestSanitizeTargets_NothingToRemove(t *testing.T) {
	targets := domain.SanitizeTargets("amadeus", domain.KindDesignFeedback, []string{"auth/session.go", "api/handler.go"})
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
}

func TestSanitizeTargets_EmptyTargets(t *testing.T) {
	targets := domain.SanitizeTargets("amadeus", domain.KindDesignFeedback, nil)
	if targets != nil {
		t.Errorf("expected nil, got %v", targets)
	}
}

func TestValidateDMail_DesignFeedbackKind(t *testing.T) {
	dmail := domain.DMail{SchemaVersion: "1", Name: "test", Kind: domain.KindDesignFeedback, Description: "test", Body: "Content.\n"}
	if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
		t.Errorf("expected valid, got: %v", errs)
	}
}

func TestValidateDMail_ImplFeedbackKind(t *testing.T) {
	dmail := domain.DMail{SchemaVersion: "1", Name: "test", Kind: domain.KindImplFeedback, Description: "test", Body: "Content.\n"}
	if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
		t.Errorf("expected valid, got: %v", errs)
	}
}

func TestValidateDMail_OldFeedbackKind_Invalid(t *testing.T) {
	dmail := domain.DMail{SchemaVersion: "1", Name: "test", Kind: "feedback", Description: "test"}
	if errs := domain.ValidateDMail(dmail); len(errs) == 0 {
		t.Error("expected validation error for old feedback kind")
	}
}

func TestParseDMailStrict_ValidInput(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "ADR-003 violation detected"
severity: high
---

# ADR-003 Violation

Body text.
`
	// given: valid frontmatter with known fields only
	// when
	dmail, err := domain.ParseDMailStrict([]byte(raw))
	// then
	if err != nil {
		t.Fatalf("ParseDMailStrict failed on valid input: %v", err)
	}
	if dmail.Name != "feedback-001" {
		t.Errorf("expected name feedback-001, got %s", dmail.Name)
	}
	if dmail.Kind != domain.KindDesignFeedback {
		t.Errorf("expected kind design-feedback, got %s", dmail.Kind)
	}
	if dmail.Severity != domain.SeverityHigh {
		t.Errorf("expected severity high, got %s", dmail.Severity)
	}
}

func TestParseDMailStrict_RejectsUnknownField(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "strict test"
unknown_field: "this should be rejected"
---

Body text.
`
	// given: frontmatter with an unknown field
	// when
	_, err := domain.ParseDMailStrict([]byte(raw))
	// then: strict parser must return an error
	if err == nil {
		t.Error("ParseDMailStrict expected error for unknown frontmatter field, got nil")
	}
}

func TestParseDMailStrict_ExistingParseDMailAcceptsUnknownField(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "strict test"
unknown_field: "silently ignored"
---

Body text.
`
	// given: the lenient parser should still accept unknown fields (backward compat)
	// when
	_, err := domain.ParseDMail([]byte(raw))
	// then
	if err != nil {
		t.Errorf("ParseDMail should accept unknown fields, got error: %v", err)
	}
}

func TestParseDMailStrict_NestedContextValidation(t *testing.T) {
	raw := `---
name: "feedback-001"
kind: design-feedback
description: "context validation"
context:
  unknown_nested: "should fail"
---

Body text.
`
	// given: frontmatter with unknown nested field in context
	// when
	_, err := domain.ParseDMailStrict([]byte(raw))
	// then: strict parser must reject unknown nested fields
	if err == nil {
		t.Error("ParseDMailStrict expected error for unknown nested field in context, got nil")
	}
}

func TestVerifyIdempotencyKey_NilMetadata(t *testing.T) {
	// given: a DMail with no metadata (nil map)
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Body:        "Details.\n",
		Metadata:    nil,
	}

	// when
	err := domain.VerifyIdempotencyKey(dmail)

	// then: lenient - no key present means no mismatch
	if err != nil {
		t.Errorf("expected nil error for nil metadata, got: %v", err)
	}
}

func TestVerifyIdempotencyKey_EmptyKey(t *testing.T) {
	// given: a DMail with metadata but empty idempotency_key
	dmail := domain.DMail{
		Name:        "feedback-001",
		Kind:        domain.KindDesignFeedback,
		Description: "ADR violation",
		Body:        "Details.\n",
		Metadata:    map[string]string{"idempotency_key": ""},
	}

	// when
	err := domain.VerifyIdempotencyKey(dmail)

	// then: lenient - empty string means no key present
	if err != nil {
		t.Errorf("expected nil error for empty idempotency_key, got: %v", err)
	}
}

func TestVerifyIdempotencyKey_RoundTrip(t *testing.T) {
	// given: a DMail marshaled (which injects idempotency_key) then parsed back
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-roundtrip",
		Kind:          domain.KindImplFeedback,
		Description:   "roundtrip check",
		Issues:        []string{"MY-545"},
		Severity:      domain.SeverityMedium,
		Body:          "# Roundtrip\n\nContent here.\n",
	}

	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}

	// when
	err = domain.VerifyIdempotencyKey(parsed)

	// then: key in metadata must match recomputed key
	if err != nil {
		t.Errorf("expected nil error on valid roundtrip, got: %v", err)
	}
}

func TestVerifyIdempotencyKey_TamperedBody(t *testing.T) {
	// given: a DMail marshaled then body tampered before parse
	original := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-tampered",
		Kind:          domain.KindDesignFeedback,
		Description:   "tamper check",
		Body:          "# Original\n\nOriginal content.\n",
	}

	data, err := domain.MarshalDMail(original)
	if err != nil {
		t.Fatalf("MarshalDMail failed: %v", err)
	}

	// tamper: replace body content after the closing frontmatter delimiter
	raw := string(data)
	tampered := raw + "EXTRA TAMPERED CONTENT\n"

	parsed, err := domain.ParseDMail([]byte(tampered))
	if err != nil {
		t.Fatalf("ParseDMail failed: %v", err)
	}

	// when
	err = domain.VerifyIdempotencyKey(parsed)

	// then: key mismatch must be returned
	if err == nil {
		t.Error("expected ErrIdempotencyMismatch for tampered body, got nil")
	}
	if err != domain.ErrIdempotencyMismatch {
		t.Errorf("expected ErrIdempotencyMismatch, got: %v", err)
	}
}

func TestDMailAge_WithValidTimestamp(t *testing.T) {
	// given: a D-Mail created 3 days ago
	now := time.Now().UTC()
	createdAt := now.Add(-3 * 24 * time.Hour)
	dmail := domain.DMail{
		Metadata: map[string]string{
			"created_at": createdAt.Format(time.RFC3339),
		},
	}

	// when
	age, ok := domain.DMailAge(dmail, now)

	// then
	if !ok {
		t.Fatal("expected ok=true for valid created_at timestamp")
	}
	if age < 2*24*time.Hour || age > 4*24*time.Hour {
		t.Errorf("expected age ~72h, got %v", age)
	}
}

func TestDMailAge_WithMissingTimestamp(t *testing.T) {
	// given: a D-Mail with no metadata
	dmail := domain.DMail{}

	// when
	_, ok := domain.DMailAge(dmail, time.Now().UTC())

	// then: missing timestamp returns ok=false
	if ok {
		t.Error("expected ok=false for missing created_at timestamp")
	}
}

func TestDMailAge_WithUnparsableTimestamp(t *testing.T) {
	// given: a D-Mail with a bad timestamp
	dmail := domain.DMail{
		Metadata: map[string]string{
			"created_at": "not-a-timestamp",
		},
	}

	// when
	_, ok := domain.DMailAge(dmail, time.Now().UTC())

	// then: unparsable timestamp returns ok=false
	if ok {
		t.Error("expected ok=false for unparsable created_at timestamp")
	}
}

func TestFilterByTTL_ExcludesStale(t *testing.T) {
	// given: a D-Mail created 8 days ago (beyond 7-day TTL)
	now := time.Now().UTC()
	stale := domain.DMail{
		Name: "stale-dmail",
		Metadata: map[string]string{
			"created_at": now.Add(-8 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	// when
	result := domain.FilterByTTL([]domain.DMail{stale}, now)

	// then: stale D-Mail is excluded
	if len(result) != 0 {
		t.Errorf("expected 0 D-Mails after TTL filter, got %d", len(result))
	}
}

func TestFilterByTTL_IncludesFresh(t *testing.T) {
	// given: a D-Mail created 2 days ago (within 7-day TTL)
	now := time.Now().UTC()
	fresh := domain.DMail{
		Name: "fresh-dmail",
		Metadata: map[string]string{
			"created_at": now.Add(-2 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	// when
	result := domain.FilterByTTL([]domain.DMail{fresh}, now)

	// then: fresh D-Mail is included
	if len(result) != 1 {
		t.Errorf("expected 1 D-Mail after TTL filter, got %d", len(result))
	}
}

func TestFilterByTTL_IncludesMissingTimestamp(t *testing.T) {
	// given: a D-Mail with no created_at (conservatively include)
	dmail := domain.DMail{
		Name: "no-timestamp-dmail",
	}

	// when
	result := domain.FilterByTTL([]domain.DMail{dmail}, time.Now().UTC())

	// then: missing timestamp D-Mail is conservatively included
	if len(result) != 1 {
		t.Errorf("expected 1 D-Mail (conservative include) after TTL filter, got %d", len(result))
	}
}

// --- S02: Feedback Loop ---

func TestRequiredTargets_DesignFeedback(t *testing.T) {
	got := domain.RequiredTargets(domain.KindDesignFeedback)
	if len(got) != 1 || got[0] != "sightjack" {
		t.Errorf("RequiredTargets(design-feedback) = %v, want [sightjack]", got)
	}
}

func TestRequiredTargets_ImplFeedback(t *testing.T) {
	got := domain.RequiredTargets(domain.KindImplFeedback)
	if len(got) != 1 || got[0] != "paintress" {
		t.Errorf("RequiredTargets(impl-feedback) = %v, want [paintress]", got)
	}
}

func TestRequiredTargets_Other_ReturnsNil(t *testing.T) {
	got := domain.RequiredTargets(domain.KindReport)
	if got != nil {
		t.Errorf("RequiredTargets(report) = %v, want nil", got)
	}
}

func TestFeedbackRound_Absent(t *testing.T) {
	d := domain.DMail{Metadata: nil}
	if got := domain.FeedbackRound(d); got != 0 {
		t.Errorf("FeedbackRound(nil metadata) = %d, want 0", got)
	}
}

func TestFeedbackRound_Present(t *testing.T) {
	d := domain.DMail{Metadata: map[string]string{"feedback_round": "2"}}
	if got := domain.FeedbackRound(d); got != 2 {
		t.Errorf("FeedbackRound = %d, want 2", got)
	}
}

func TestWithFeedbackRound(t *testing.T) {
	d := domain.DMail{Metadata: map[string]string{"existing": "val"}}
	d2 := domain.WithFeedbackRound(d, 3)
	if d2.Metadata["feedback_round"] != "3" {
		t.Errorf("feedback_round = %q, want 3", d2.Metadata["feedback_round"])
	}
	if d2.Metadata["existing"] != "val" {
		t.Error("existing metadata lost")
	}
	// Original should not be mutated
	if _, ok := d.Metadata["feedback_round"]; ok {
		t.Error("original DMail metadata was mutated")
	}
}
