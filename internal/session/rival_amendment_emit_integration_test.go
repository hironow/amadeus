// white-box-reason: Phase 1.2B (am side) integration test that drives
// the real corrective D-Mail body composer
// (composeCorrectiveBodyWithContract / writeContractAmendmentsSection)
// and commits the emitted body as the cross-tool source-of-truth fixture
// for sj's subsequent Phase 1.2B sj-side amendment-extract test. The
// composer is unexported, so this lives in package session (white-box)
// rather than in tests/integration/ (black-box) — see plan §"Phase 1.2B"
// at refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md.
package session

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

// updateGolden, when set via `go test ... -update`, rewrites the committed
// golden fixture from the live composer output. Maintenance toggle only;
// CI runs without the flag and asserts byte-equality.
var updateGolden = flag.Bool("update", false, "update golden fixture from live composer output (Phase 1.2B am side)")

// goldenPath is the cross-tool source-of-truth fixture path. sj's Phase
// 1.2B sj-side test will commit a byte-identical copy under
// sightjack/tests/integration/testdata/rival/cross-tool/, and a
// gap-check script enforces byte-identity.
const goldenPath = "testdata/rival/cross-tool/amadeus-emitted-correction.md"

// fixtureCurrentContract returns the deterministic Rival Contract v1
// projection used for the cross-tool amendment-emit fixture. Stable
// across runs by construction: pure value type, no clock, no IDs.
func fixtureCurrentContract() *domain.RivalContractContext {
	return &domain.RivalContractContext{
		ContractID: "auth-session-expiry",
		Revision:   2,
		Title:      "Add session expiry enforcement",
		Intent:     "Prevent expired sessions.",
		Decisions:  "Enforce expiry in middleware before handler execution.",
		Boundaries: "Do not add OAuth, refresh tokens, or background cleanup.",
		Evidence:   "test: just test",
	}
}

// fixtureAmendments returns the deterministic amendment set used for the
// cross-tool fixture. Two bullets exercise the grammar variants the
// sj-side parser must accept: with-rationale and without-rationale. Both
// carry an explicit canonical Section ("Boundaries"/"Evidence") to keep
// the bullet shape strict (no "(unspecified)" fallback).
func fixtureAmendments() []domain.RivalContractAmendment {
	return []domain.RivalContractAmendment{
		{
			Section:    "Boundaries",
			Suggestion: "Allow short-lived OAuth refresh tokens.",
			Rationale:  "SSO requires refresh.",
		},
		{
			Section:    "Evidence",
			Suggestion: "Add nfr.p95_latency_ms: <= 250 entry.",
		},
	}
}

// fixtureDetail is the corrective D-Mail prelude that precedes the
// "## Contract Amendments" section. Kept terse so the golden remains
// human-readable.
const fixtureDetail = "Boundaries section is no longer reflective of SSO requirements."

// composeFixtureBody runs the real corrective body composer with the
// fixed inputs that produce the cross-tool golden.
func composeFixtureBody() string {
	current := fixtureCurrentContract()
	amendments := fixtureAmendments()
	return composeCorrectiveBodyWithContract(
		fixtureDetail,
		domain.KindDesignFeedback,
		current,
		nil, // no violation citation: pure-amendments routing case.
		amendments,
	)
}

func TestRivalAmendmentEmit_AppendsContractAmendmentsSection_WritesGolden(t *testing.T) {
	// given the deterministic fixture contract, no violation citation, and
	// two proposed amendments (one with rationale, one without).

	// when the real corrective body composer is invoked.
	got := composeFixtureBody()

	// then the emitted body MUST byte-match the committed golden. The
	// golden is the source of truth for sj's Phase 1.2B sj-side test.
	abs, err := filepath.Abs(goldenPath)
	if err != nil {
		t.Fatalf("resolve golden path: %v", err)
	}

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden: %s", abs)
		return
	}

	want, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `go test -run TestRivalAmendmentEmit_AppendsContractAmendmentsSection_WritesGolden -update` to create)", abs, err)
	}
	if !bytes.Equal([]byte(got), want) {
		t.Errorf("emitted body does not match golden\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestRivalAmendmentEmit_BulletGrammarStable(t *testing.T) {
	// given the deterministic fixture inputs.
	// when the real corrective body composer is invoked.
	body := composeFixtureBody()

	// then the with-rationale bullet matches sj's parser grammar exactly:
	// `^- Amend <Section>: <Suggestion> \(rationale: <Rationale>\)$`.
	wantWithRationale := "- Amend Boundaries: Allow short-lived OAuth refresh tokens. (rationale: SSO requires refresh.)"
	if !strings.Contains(body, wantWithRationale) {
		t.Errorf("expected with-rationale bullet substring %q in body\n--- body ---\n%s", wantWithRationale, body)
	}

	// And the without-rationale bullet matches the no-parenthetical form:
	// `^- Amend <Section>: <Suggestion>$`. The body MUST NOT append
	// "(rationale: ...)" when Rationale is empty.
	wantWithoutRationale := "- Amend Evidence: Add nfr.p95_latency_ms: <= 250 entry."
	if !strings.Contains(body, wantWithoutRationale) {
		t.Errorf("expected without-rationale bullet substring %q in body\n--- body ---\n%s", wantWithoutRationale, body)
	}
	// Confirm the without-rationale bullet has no "(rationale: ...)" suffix
	// on its own line. We assert by checking that the line ending the
	// without-rationale bullet does NOT contain the rationale marker.
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "- Amend Evidence:") {
			if strings.Contains(line, "(rationale:") {
				t.Errorf("Evidence bullet must not carry a rationale parenthetical when Rationale is empty: %q", line)
			}
		}
	}

	// And the canonical section header is present so sj's parser can scope
	// bullet extraction to the amendments section.
	if !strings.Contains(body, "## Contract Amendments") {
		t.Errorf("expected '## Contract Amendments' section header in body\n--- body ---\n%s", body)
	}

	// And the leading "- Contract: <id> (revision N)" line is present so
	// sj can correlate the amendment proposal with the cited contract.
	wantContractLine := "- Contract: `auth-session-expiry` (revision 2)"
	if !strings.Contains(body, wantContractLine) {
		t.Errorf("expected contract citation line %q in body\n--- body ---\n%s", wantContractLine, body)
	}
}
