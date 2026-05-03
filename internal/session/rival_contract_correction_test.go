// white-box-reason: tests session-level corrective body composer that
// appends Rival Contract v1 citation sections (Violated Contract /
// Contract Amendments) to a corrective D-Mail body before emission.
package session

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestCorrectionDMail_IncludesViolatedContractSection(t *testing.T) {
	// given an implementation-feedback corrective D-Mail body and a
	// current Rival Contract v1 projection. The corrective composer is
	// asked to emit a "Violated Contract" citation because the diff
	// violates a contract Boundary.
	current := &domain.RivalContractContext{
		ContractID: "auth-session-expiry",
		Revision:   2,
		Title:      "Add session expiry enforcement",
		Intent:     "Prevent expired sessions.",
		Decisions:  "Enforce expiry in middleware before handler execution.",
		Boundaries: "Do not add OAuth, refresh tokens, or background cleanup.",
		Evidence:   "test: just test",
	}
	citation := domain.RivalContractCitation{
		ContractID: current.ContractID,
		Revision:   current.Revision,
		Reason:     "Diff adds an OAuth refresh-token path which the contract Boundaries forbid.",
	}

	// when the implementation-feedback body is composed with a violation
	// citation and no amendments.
	body := composeCorrectiveBodyWithContract(
		"OAuth scope creep detected in diff hunk #3.",
		domain.KindImplFeedback,
		current,
		&citation,
		nil,
	)

	// then the body contains the canonical "## Violated Contract" header
	// with the contract id, revision, and the cited reason.
	if !strings.Contains(body, "## Violated Contract") {
		t.Error("expected '## Violated Contract' section header")
	}
	if !strings.Contains(body, "auth-session-expiry") {
		t.Error("expected contract id in cited section")
	}
	if !strings.Contains(body, "revision 2") {
		t.Error("expected contract revision in cited section")
	}
	if !strings.Contains(body, "OAuth refresh-token") {
		t.Error("expected violation reason text in cited section")
	}
	// And the body MUST NOT silently emit a Contract Amendments section
	// for a pure violation — that would conflate the two routing cases.
	if strings.Contains(body, "## Contract Amendments") {
		t.Error("expected NO '## Contract Amendments' section when only a violation citation is provided")
	}
}

func TestCorrectionDMail_IncludesContractAmendmentsSection(t *testing.T) {
	// given a design-feedback corrective D-Mail body and a current Rival
	// Contract v1 projection. The corrective composer is asked to emit a
	// "Contract Amendments" section because reality diverged from the
	// contract — the contract should be amended, not the implementation.
	current := &domain.RivalContractContext{
		ContractID: "auth-session-expiry",
		Revision:   2,
		Title:      "Add session expiry enforcement",
		Intent:     "Prevent expired sessions.",
		Decisions:  "Enforce expiry in middleware before handler execution.",
		Boundaries: "Do not add OAuth.",
		Evidence:   "test: just test",
	}
	amendments := []domain.RivalContractAmendment{
		{
			Section:    "Boundaries",
			Suggestion: "Allow short-lived OAuth refresh tokens for first-party clients.",
			Rationale:  "Implementation now requires SSO; original boundary is obsolete.",
		},
		{
			Section:    "Evidence",
			Suggestion: "Add nfr.p95_latency_ms: <= 250 to reflect SSO overhead.",
		},
	}

	// when the design-feedback body is composed with no violation citation
	// and one or more amendments.
	body := composeCorrectiveBodyWithContract(
		"Boundaries section is no longer reflective of SSO requirements.",
		domain.KindDesignFeedback,
		current,
		nil,
		amendments,
	)

	// then the body contains the canonical "## Contract Amendments"
	// header and one bullet per amendment with the targeted section.
	if !strings.Contains(body, "## Contract Amendments") {
		t.Error("expected '## Contract Amendments' section header")
	}
	if !strings.Contains(body, "auth-session-expiry") {
		t.Error("expected contract id in amendments section")
	}
	if !strings.Contains(body, "Boundaries") {
		t.Error("expected amended section name 'Boundaries'")
	}
	if !strings.Contains(body, "OAuth refresh tokens") {
		t.Error("expected amendment suggestion text in body")
	}
	if !strings.Contains(body, "nfr.p95_latency_ms") {
		t.Error("expected second amendment suggestion text in body")
	}
	// And without a violation citation, no Violated Contract section is
	// emitted — keeps the two routing cases distinguishable for amadeus
	// downstream consumers (Phase 5 amendment loop).
	if strings.Contains(body, "## Violated Contract") {
		t.Error("expected NO '## Violated Contract' section in pure-amendments mode")
	}
}

func TestCorrectionDMail_NoCurrentContract_PreservesOriginalDetail(t *testing.T) {
	// given no current contract (legacy path, graceful degradation): the
	// composer must return the original detail verbatim, with no contract
	// citations appended.
	body := composeCorrectiveBodyWithContract(
		"Diff exceeds wave step 1 boundaries.",
		domain.KindImplFeedback,
		nil,
		nil,
		nil,
	)

	// then
	if body != "Diff exceeds wave step 1 boundaries." {
		t.Errorf("expected verbatim body when no contract, got %q", body)
	}
}
