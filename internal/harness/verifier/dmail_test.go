package verifier_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/verifier"
)

func TestValidateDMail_Valid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "ADR violation detected",
		Severity:      domain.SeverityHigh,
		Body:          "Details.\n",
	}
	errs := verifier.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateDMail_AllKinds(t *testing.T) {
	for _, kind := range []domain.DMailKind{domain.KindDesignFeedback, domain.KindImplFeedback, domain.KindSpecification, domain.KindReport, domain.KindConvergence, domain.KindCIResult, domain.KindStallEscalation} {
		dmail := domain.DMail{
			SchemaVersion: domain.DMailSchemaVersion,
			Name:          "test-001",
			Kind:          kind,
			Description:   "test",
			Severity:      domain.SeverityLow,
			Body:          "Content.\n",
		}
		errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for invalid severity")
	}
}

func TestValidateDMail_MultipleErrors(t *testing.T) {
	dmail := domain.DMail{}
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
	if len(errs) == 0 {
		t.Error("expected error for unsupported dmail-schema-version")
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
	errs := verifier.ValidateDMail(dmail)

	// then
	if len(errs) != 0 {
		t.Errorf("expected no errors for ci-result kind, got %v", errs)
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
	errs := verifier.ValidateDMail(dmail)

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
	errs := verifier.ValidateDMail(dmail)

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
		errs := verifier.ValidateDMail(dmail)
		if len(errs) != 0 {
			t.Errorf("action %s: expected no errors, got %v", action, errs)
		}
	}
}

func TestValidateDMail_EmptyBody_IsInvalid(t *testing.T) {
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
	}
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
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
	errs := verifier.ValidateDMail(dmail)
	if len(errs) != 0 {
		t.Errorf("expected no errors for no targets, got %v", errs)
	}
}

func TestValidateDMail_DesignFeedbackKind(t *testing.T) {
	dmail := domain.DMail{SchemaVersion: "1", Name: "test", Kind: domain.KindDesignFeedback, Description: "test", Body: "Content.\n"}
	if errs := verifier.ValidateDMail(dmail); len(errs) > 0 {
		t.Errorf("expected valid, got: %v", errs)
	}
}

func TestValidateDMail_ImplFeedbackKind(t *testing.T) {
	dmail := domain.DMail{SchemaVersion: "1", Name: "test", Kind: domain.KindImplFeedback, Description: "test", Body: "Content.\n"}
	if errs := verifier.ValidateDMail(dmail); len(errs) > 0 {
		t.Errorf("expected valid, got: %v", errs)
	}
}

func TestValidateDMail_OldFeedbackKind_Invalid(t *testing.T) {
	dmail := domain.DMail{SchemaVersion: "1", Name: "test", Kind: "feedback", Description: "test"}
	if errs := verifier.ValidateDMail(dmail); len(errs) == 0 {
		t.Error("expected validation error for old feedback kind")
	}
}

