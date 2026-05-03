package policy_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
)

func readRivalFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "rival", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

const legacySpecBody = `# Add session expiry enforcement

Some legacy specification body without the Contract title prefix.

## Intent

Prevent expired sessions.
`

const partialContractBody = `# Contract: Add session expiry enforcement

## Intent
- Prevent expired sessions.

## Domain
- Command: validate session.

## Decisions
- Enforce expiry in middleware.

## Steps
1. Add expiry check.

## Boundaries
- Do not add OAuth.
`

func TestParseRivalContractBody_ValidV1(t *testing.T) {
	// given
	body := readRivalFixture(t, "valid-v1.md")

	// when
	contract, ok, err := policy.ParseRivalContractBody(body)

	// then
	if err != nil {
		t.Fatalf("ParseRivalContractBody: unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("ParseRivalContractBody: expected ok=true for valid v1 body")
	}
	if contract.Title != "Add session expiry enforcement" {
		t.Errorf("Title: got %q", contract.Title)
	}
	if !strings.Contains(contract.Intent, "Prevent expired sessions") {
		t.Errorf("Intent missing expected text: %q", contract.Intent)
	}
	if !strings.Contains(contract.Domain, "validate session for request") {
		t.Errorf("Domain missing expected text: %q", contract.Domain)
	}
	if !strings.Contains(contract.Decisions, "Enforce expiry in middleware") {
		t.Errorf("Decisions missing expected text: %q", contract.Decisions)
	}
	if !strings.Contains(contract.Steps, "Add expiry check to auth middleware") {
		t.Errorf("Steps missing expected text: %q", contract.Steps)
	}
	if !strings.Contains(contract.Boundaries, "Do not add OAuth") {
		t.Errorf("Boundaries missing expected text: %q", contract.Boundaries)
	}
	if !strings.Contains(contract.Evidence, "test: just test") {
		t.Errorf("Evidence missing expected text: %q", contract.Evidence)
	}
}

func TestParseRivalContractBody_LegacyReturnsFalse(t *testing.T) {
	// given a legacy specification body without a `# Contract:` title
	body := legacySpecBody

	// when
	_, ok, err := policy.ParseRivalContractBody(body)

	// then
	if err != nil {
		t.Fatalf("ParseRivalContractBody on legacy body must not error: %v", err)
	}
	if ok {
		t.Fatal("ParseRivalContractBody: expected ok=false for legacy body without # Contract: heading")
	}
}

func TestParseRivalContractBody_PartialReturnsError(t *testing.T) {
	// given a body with the Contract title but missing the Evidence section
	body := partialContractBody

	// when
	_, ok, err := policy.ParseRivalContractBody(body)

	// then
	if err == nil {
		t.Fatal("ParseRivalContractBody: expected error for partial v1 body")
	}
	if ok {
		t.Errorf("ParseRivalContractBody: expected ok=false on error, got ok=true")
	}
	if !errors.Is(err, policy.ErrPartialContractBody) {
		t.Errorf("ParseRivalContractBody: expected ErrPartialContractBody, got %v", err)
	}
}

func TestParseEvidenceItems_ParsesSupportedKeys(t *testing.T) {
	// given
	evidence := strings.Join([]string{
		"- check: just check",
		"- test: just test",
		"- lint: just lint",
		"- semgrep: just semgrep",
		"- nfr.p95_latency_ms: <= 200",
		"- nfr.error_rate_percent: <= 1",
		"- nfr.success_rate_percent: >= 99",
		"- nfr.target_rps: >= 50",
	}, "\n")

	// when
	items := policy.ParseEvidenceItems(evidence)

	// then
	want := map[string]struct {
		Operator string
		Value    string
	}{
		"check":                    {"", "just check"},
		"test":                     {"", "just test"},
		"lint":                     {"", "just lint"},
		"semgrep":                  {"", "just semgrep"},
		"nfr.p95_latency_ms":       {"<=", "200"},
		"nfr.error_rate_percent":   {"<=", "1"},
		"nfr.success_rate_percent": {">=", "99"},
		"nfr.target_rps":           {">=", "50"},
	}
	if len(items) != len(want) {
		t.Fatalf("ParseEvidenceItems: got %d items, want %d (items=%+v)", len(items), len(want), items)
	}
	for _, item := range items {
		expected, found := want[item.Key]
		if !found {
			t.Errorf("unexpected key %q", item.Key)
			continue
		}
		if item.Operator != expected.Operator {
			t.Errorf("key %q: operator got %q want %q", item.Key, item.Operator, expected.Operator)
		}
		if item.Value != expected.Value {
			t.Errorf("key %q: value got %q want %q", item.Key, item.Value, expected.Value)
		}
	}
}

func TestParseEvidenceItems_IgnoresUnknownAndProse(t *testing.T) {
	// given
	evidence := strings.Join([]string{
		"- Add a regression test for expired sessions.",
		"- test: just test",
		"- unknown.key: 1",
		"- nfr.unknown_metric: <= 99",
		"Plain prose without bullet.",
		"- still prose without colon",
	}, "\n")

	// when
	items := policy.ParseEvidenceItems(evidence)

	// then
	if len(items) != 1 {
		t.Fatalf("ParseEvidenceItems: expected 1 item (only test), got %d (items=%+v)", len(items), items)
	}
	if items[0].Key != "test" {
		t.Errorf("expected only key 'test', got %q", items[0].Key)
	}
	if items[0].Value != "just test" {
		t.Errorf("expected value 'just test', got %q", items[0].Value)
	}
}

func TestDeriveContractID_PrefersWaveID(t *testing.T) {
	// when
	id, err := policy.DeriveContractID("auth-session-expiry", []string{"ISS-2", "ISS-1"}, "auth-cluster")

	// then
	if err != nil {
		t.Fatalf("DeriveContractID: unexpected error: %v", err)
	}
	if id != "auth-session-expiry" {
		t.Errorf("DeriveContractID: expected wave ID, got %q", id)
	}
}

func TestDeriveContractID_RejectsDMailNameFallback(t *testing.T) {
	// when no wave / issues / cluster is available
	id, err := policy.DeriveContractID("", nil, "")

	// then
	if err == nil {
		t.Fatalf("DeriveContractID: expected error when no stable input, got id=%q", id)
	}
	if !errors.Is(err, policy.ErrContractIDUnavailable) {
		t.Errorf("DeriveContractID: expected ErrContractIDUnavailable, got %v", err)
	}
}

// --- amadeus-specific: ProjectCurrentContracts -------------------------------

// rivalDMail builds a specification D-Mail with the canonical Rival Contract v1
// metadata and a parameterised body so test scenarios can be assembled inline.
func rivalDMail(name, contractID string, revision int, supersedes string, body string) domain.DMail {
	meta := map[string]string{
		"contract_schema":   policy.SchemaRivalContractV1,
		"contract_id":       contractID,
		"contract_revision": itoa(revision),
	}
	if supersedes != "" {
		meta["supersedes"] = supersedes
	}
	return domain.DMail{
		Name:     name,
		Kind:     domain.KindSpecification,
		Body:     body,
		Metadata: meta,
	}
}

// rivalDMailWithKey is like rivalDMail but additionally pins an
// idempotency_key so duplicate-delivery scenarios are unambiguous.
func rivalDMailWithKey(name, contractID string, revision int, supersedes, body, idempotencyKey string) domain.DMail {
	d := rivalDMail(name, contractID, revision, supersedes, body)
	d.Metadata["idempotency_key"] = idempotencyKey
	return d
}

// itoa avoids pulling strconv into the test surface for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func TestProjectCurrentContracts_HighestRevisionWins(t *testing.T) {
	// given two revisions of the same contract: revision 1 (older) and
	// revision 2 (newer). The newer revision must win.
	bodyV1 := readRivalFixture(t, "valid-v1.md")
	bodyV2 := readRivalFixture(t, "valid-v1.md") // body unchanged but revision bumped
	older := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", bodyV1)
	newer := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 2, "spec-auth_aaaaaaaa", bodyV2)

	// when
	current, conflicts := policy.ProjectCurrentContracts([]domain.DMail{older, newer})

	// then
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %+v", conflicts)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 current contract, got %d", len(current))
	}
	if current[0].Metadata.Revision != 2 {
		t.Errorf("expected winning revision 2, got %d", current[0].Metadata.Revision)
	}
	if current[0].DMailName != "spec-auth_bbbbbbbb" {
		t.Errorf("expected winner name 'spec-auth_bbbbbbbb', got %q", current[0].DMailName)
	}
}

func TestProjectCurrentContracts_DuplicateDeliveryTolerated(t *testing.T) {
	// given the same logical contract delivered twice (e.g. through phonewave
	// at-least-once delivery). The two D-Mails share the same idempotency_key
	// and the same body. The projection must collapse them to one winner
	// without emitting a conflict.
	body := readRivalFixture(t, "valid-v1.md")
	first := rivalDMailWithKey("spec-auth_aaaaaaaa", "auth-x", 1, "", body, "key-shared")
	second := rivalDMailWithKey("spec-auth_bbbbbbbb", "auth-x", 1, "", body, "key-shared")

	// when
	current, conflicts := policy.ProjectCurrentContracts([]domain.DMail{first, second})

	// then
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts for duplicate delivery, got %+v", conflicts)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 current contract, got %d", len(current))
	}
	// Deterministic tie-break: lexicographically smallest D-Mail name wins.
	if current[0].DMailName != "spec-auth_aaaaaaaa" {
		t.Errorf("expected deterministic winner 'spec-auth_aaaaaaaa', got %q", current[0].DMailName)
	}
}

func TestProjectCurrentContracts_SameRevisionConflict(t *testing.T) {
	// given two D-Mails with the same contract_id and revision but different
	// bodies. The projection must emit a same-revision conflict.
	bodyA := readRivalFixture(t, "conflicting-revision-a.md")
	bodyB := readRivalFixture(t, "conflicting-revision-b.md")
	a := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", bodyA)
	b := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 1, "", bodyB)

	// when
	current, conflicts := policy.ProjectCurrentContracts([]domain.DMail{a, b})

	// then
	if len(current) != 0 {
		t.Errorf("expected no current contract while in conflict, got %+v", current)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d (%+v)", len(conflicts), conflicts)
	}
	c := conflicts[0]
	if c.ContractID != "auth-x" {
		t.Errorf("ContractID: got %q", c.ContractID)
	}
	if !strings.Contains(c.Reason, "same-revision") {
		t.Errorf("Reason: expected 'same-revision', got %q", c.Reason)
	}
	if len(c.Names) != 2 {
		t.Errorf("Names: expected both D-Mail names, got %v", c.Names)
	}
}

func TestProjectCurrentContracts_InvalidSupersedesConflict(t *testing.T) {
	// given a winning revision that points at a supersedes name which does
	// not exist in the group. The projection must emit an invalid-supersedes
	// conflict and refuse to publish a current contract for that id.
	body := readRivalFixture(t, "valid-v1.md")
	older := rivalDMail("spec-auth_aaaaaaaa", "auth-x", 1, "", body)
	newer := rivalDMail("spec-auth_bbbbbbbb", "auth-x", 2, "spec-auth_does-not-exist", body)

	// when
	current, conflicts := policy.ProjectCurrentContracts([]domain.DMail{older, newer})

	// then
	if len(current) != 0 {
		t.Errorf("expected no current contract when supersedes lineage is invalid, got %+v", current)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d (%+v)", len(conflicts), conflicts)
	}
	if conflicts[0].ContractID != "auth-x" {
		t.Errorf("ContractID: got %q", conflicts[0].ContractID)
	}
	if !strings.Contains(conflicts[0].Reason, "supersedes") {
		t.Errorf("Reason: expected 'supersedes', got %q", conflicts[0].Reason)
	}
}

func TestProjectCurrentContracts_RejectsDMailNameContractID(t *testing.T) {
	// given a D-Mail whose metadata.contract_id matches the D-Mail name
	// pattern. The metadata parser rejects it, so the projection must skip
	// the D-Mail entirely and produce neither a current contract nor a
	// conflict.
	body := readRivalFixture(t, "valid-v1.md")
	bad := domain.DMail{
		Name: "spec-auth_aaaaaaaa",
		Kind: domain.KindSpecification,
		Body: body,
		Metadata: map[string]string{
			"contract_schema":   policy.SchemaRivalContractV1,
			"contract_id":       "spec-auth-session_a3f2b7c4",
			"contract_revision": "1",
		},
	}

	// when
	current, conflicts := policy.ProjectCurrentContracts([]domain.DMail{bad})

	// then
	if len(current) != 0 {
		t.Errorf("expected no current contract for D-Mail-name-as-id, got %+v", current)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflict for invalid metadata, got %+v", conflicts)
	}
}
