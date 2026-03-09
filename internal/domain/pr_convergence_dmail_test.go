package domain_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestBuildConvergenceDMailBody_singleChain(t *testing.T) {
	// given
	pr1 := mustPRState(t, "#1", "Base feature", "main", "feat-1", true, 0, nil)
	pr3 := mustPRState(t, "#3", "Extend feature", "#1", "feat-3", true, 2, nil)
	report := domain.PRConvergenceReport{
		IntegrationBranch: "main",
		Chains: []domain.PRChain{
			{ID: "chain-a", PRs: []domain.PRState{pr1, pr3}, HasConflict: false},
		},
		TotalOpenPRs: 2,
	}

	// when
	body := domain.BuildConvergenceDMailBody(report)

	// then — header present
	if !strings.Contains(body, "## PR Dependency Chain Analysis") {
		t.Error("expected header '## PR Dependency Chain Analysis' in body")
	}
	// chain structure visualization
	if !strings.Contains(body, "#1") || !strings.Contains(body, "#3") {
		t.Error("expected chain structure with PR numbers #1 and #3")
	}
	// issue table header
	if !strings.Contains(body, "| PR |") {
		t.Error("expected issue table header '| PR |' in body")
	}
}

func TestBuildConvergenceDMailBody_withConflict(t *testing.T) {
	// given
	pr1 := mustPRState(t, "#1", "Base", "main", "feat-1", true, 0, nil)
	pr3 := mustPRState(t, "#3", "Conflict", "#1", "feat-3", false, 5, []string{"api.go", "handler.go"})
	report := domain.PRConvergenceReport{
		IntegrationBranch: "main",
		Chains: []domain.PRChain{
			{ID: "chain-a", PRs: []domain.PRState{pr1, pr3}, HasConflict: true},
		},
		TotalOpenPRs: 2,
	}

	// when
	body := domain.BuildConvergenceDMailBody(report)

	// then — conflict details section present
	if !strings.Contains(body, "Conflict") {
		t.Error("expected conflict details section in body")
	}
	if !strings.Contains(body, "api.go") {
		t.Error("expected conflicting file 'api.go' listed in body")
	}
	if !strings.Contains(body, "handler.go") {
		t.Error("expected conflicting file 'handler.go' listed in body")
	}
}

func TestBuildConvergenceDMailBody_multipleChains(t *testing.T) {
	// given
	prA1 := mustPRState(t, "#1", "Chain A root", "main", "feat-a1", true, 0, nil)
	prA3 := mustPRState(t, "#3", "Chain A leaf", "#1", "feat-a3", true, 1, nil)
	prB2 := mustPRState(t, "#2", "Chain B root", "main", "feat-b2", true, 0, nil)
	prB5 := mustPRState(t, "#5", "Chain B leaf", "#2", "feat-b5", true, 0, nil)
	report := domain.PRConvergenceReport{
		IntegrationBranch: "main",
		Chains: []domain.PRChain{
			{ID: "chain-a", PRs: []domain.PRState{prA1, prA3}, HasConflict: false},
			{ID: "chain-b", PRs: []domain.PRState{prB2, prB5}, HasConflict: false},
		},
		TotalOpenPRs: 4,
	}

	// when
	body := domain.BuildConvergenceDMailBody(report)

	// then — both chains referenced
	if !strings.Contains(body, "chain-a") {
		t.Error("expected chain-a in body")
	}
	if !strings.Contains(body, "chain-b") {
		t.Error("expected chain-b in body")
	}
	// All PR numbers present
	for _, num := range []string{"#1", "#2", "#3", "#5"} {
		if !strings.Contains(body, num) {
			t.Errorf("expected PR %s in body", num)
		}
	}
}

func TestBuildConvergenceDMail_valid(t *testing.T) {
	// given
	pr1 := mustPRState(t, "#1", "Feature", "main", "feat-1", true, 0, nil)
	pr3 := mustPRState(t, "#3", "Extend", "#1", "feat-3", true, 2, nil)
	report := domain.PRConvergenceReport{
		IntegrationBranch: "main",
		Chains: []domain.PRChain{
			{ID: "chain-a", PRs: []domain.PRState{pr1, pr3}, HasConflict: false},
		},
		TotalOpenPRs: 2,
	}

	// when
	dmail := domain.BuildConvergenceDMail("test-convergence", report)

	// then — Kind
	if dmail.Kind != domain.KindImplFeedback {
		t.Errorf("expected Kind %q, got %q", domain.KindImplFeedback, dmail.Kind)
	}
	// SchemaVersion
	if dmail.SchemaVersion != domain.DMailSchemaVersion {
		t.Errorf("expected SchemaVersion %q, got %q", domain.DMailSchemaVersion, dmail.SchemaVersion)
	}
	// Metadata
	if dmail.Metadata["integration_branch"] != "main" {
		t.Errorf("expected metadata integration_branch=main, got %q", dmail.Metadata["integration_branch"])
	}
	if dmail.Metadata["chain_count"] != "1" {
		t.Errorf("expected metadata chain_count=1, got %q", dmail.Metadata["chain_count"])
	}
	if dmail.Metadata["conflict_prs"] != "" {
		t.Errorf("expected metadata conflict_prs empty, got %q", dmail.Metadata["conflict_prs"])
	}
	// Targets contain PR numbers
	if len(dmail.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(dmail.Targets))
	}
	// Body is non-empty
	if dmail.Body == "" {
		t.Error("expected non-empty body")
	}
	// Must pass ValidateDMail
	errs := domain.ValidateDMail(dmail)
	if len(errs) > 0 {
		t.Errorf("ValidateDMail failed: %v", errs)
	}
}

func TestBuildConvergenceDMail_severityFromWorstChain(t *testing.T) {
	// given — 2 chains, one with conflict
	prA := mustPRState(t, "#1", "Safe", "main", "feat-a", true, 0, nil)
	prB := mustPRState(t, "#2", "Root B", "main", "feat-b", true, 0, nil)
	prB2 := mustPRState(t, "#4", "Conflict B", "#2", "feat-b2", false, 3, []string{"x.go"})
	report := domain.PRConvergenceReport{
		IntegrationBranch: "main",
		Chains: []domain.PRChain{
			{ID: "chain-a", PRs: []domain.PRState{prA}, HasConflict: false},
			{ID: "chain-b", PRs: []domain.PRState{prB, prB2}, HasConflict: true},
		},
		TotalOpenPRs: 3,
	}

	// when
	dmail := domain.BuildConvergenceDMail("test-severity", report)

	// then — severity from worst chain (conflict => high)
	if dmail.Severity != domain.SeverityHigh {
		t.Errorf("expected severity %q, got %q", domain.SeverityHigh, dmail.Severity)
	}
}
