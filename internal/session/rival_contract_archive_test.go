// white-box-reason: tests session-level archive projection helper that wraps
// the policy layer's pure ProjectCurrentContracts with archive I/O.
package session

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// rivalContractValidBody is a minimal Rival Contract v1 body that satisfies
// ParseRivalContractBody (title + all six required ## sections).
const rivalContractValidBody = `# Contract: Add session expiry enforcement

## Intent
- Prevent expired sessions from authorizing API calls.

## Domain
- Command: validate session for request.

## Decisions
- Enforce expiry in middleware before handler execution.

## Steps
1. Add expiry check to auth middleware.

## Boundaries
- Do not add OAuth, refresh tokens, or background cleanup.

## Evidence
- test: just test
- nfr.p95_latency_ms: <= 200
`

// rivalContractAlternateBody differs from rivalContractValidBody so two
// candidates at the same revision are recognised as a real conflict.
const rivalContractAlternateBody = `# Contract: Add session expiry enforcement

## Intent
- Different intent text triggering same-revision conflict.

## Domain
- Command: validate session for request.

## Decisions
- Enforce expiry in middleware before handler execution.

## Steps
1. Add expiry check to auth middleware.

## Boundaries
- Do not add OAuth.

## Evidence
- test: just test
`

// makeRivalSpecDMail constructs a specification D-Mail carrying the canonical
// Rival Contract v1 metadata and the supplied body. It mirrors the helper
// shape used in the policy-level tests but lives here so the session-level
// tests stay self-contained.
func makeRivalSpecDMail(name, contractID, revision, supersedes, body string) domain.DMail {
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       contractID,
		"contract_revision": revision,
	}
	if supersedes != "" {
		meta["supersedes"] = supersedes
	}
	return domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          name,
		Kind:          domain.KindSpecification,
		Description:   "Add session expiry enforcement",
		Body:          body,
		Metadata:      meta,
	}
}

func TestReadRivalContractsFromArchive_SelectsCurrentRevision(t *testing.T) {
	// given two revisions of the same Rival Contract v1 contract — older is
	// revision 1, newer is revision 2 superseding the older D-Mail. The
	// reader must select the highest revision and emit no conflict.
	older := makeRivalSpecDMail("spec-auth-001_aaaaaaaa", "auth-x", "1", "", rivalContractValidBody)
	newer := makeRivalSpecDMail("spec-auth-002_bbbbbbbb", "auth-x", "2", "spec-auth-001_aaaaaaaa", rivalContractValidBody)

	// when the session-level reader runs against the in-memory archive.
	current, conflicts, err := ReadRivalContractsFromDMails([]domain.DMail{older, newer})

	// then the reader returns exactly one current contract at revision 2 and
	// no conflicts. The reader is a thin session adapter on top of the
	// pure policy projection — the contract identity comes from the
	// policy package via the harness facade.
	if err != nil {
		t.Fatalf("ReadRivalContractsFromDMails: unexpected error: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %+v", conflicts)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 current contract, got %d", len(current))
	}
	if current[0].Metadata.Revision != 2 {
		t.Errorf("expected current revision 2, got %d", current[0].Metadata.Revision)
	}
	if current[0].DMailName != "spec-auth-002_bbbbbbbb" {
		t.Errorf("expected current name spec-auth-002_bbbbbbbb, got %q", current[0].DMailName)
	}
	if !strings.Contains(current[0].Contract.Boundaries, "Do not add OAuth") {
		t.Errorf("expected parsed contract Boundaries to include OAuth guard, got %q", current[0].Contract.Boundaries)
	}
}

func TestReadRivalContractsFromArchive_ReportsConflict(t *testing.T) {
	// given two D-Mails claiming the same contract_id at the same revision
	// but with different bodies. The session reader must surface the
	// same-revision conflict instead of arbitrarily picking one.
	a := makeRivalSpecDMail("spec-auth-001_aaaaaaaa", "auth-x", "1", "", rivalContractValidBody)
	b := makeRivalSpecDMail("spec-auth-002_bbbbbbbb", "auth-x", "1", "", rivalContractAlternateBody)

	// when
	current, conflicts, err := ReadRivalContractsFromDMails([]domain.DMail{a, b})

	// then
	if err != nil {
		t.Fatalf("ReadRivalContractsFromDMails: unexpected error: %v", err)
	}
	if len(current) != 0 {
		t.Errorf("expected no current contract while in conflict, got %+v", current)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d (%+v)", len(conflicts), conflicts)
	}
	if conflicts[0].ContractID != "auth-x" {
		t.Errorf("ContractID: got %q, want auth-x", conflicts[0].ContractID)
	}
	if !strings.Contains(conflicts[0].Reason, "same-revision") {
		t.Errorf("Reason: expected to mention 'same-revision', got %q", conflicts[0].Reason)
	}
}
