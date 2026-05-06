package policy_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/harness/policy"
)

// Phase 1.2 (multiplex readiness) — unknown metadata keys regression test.
//
// Plan: refs/docs/issues/0014-tap-multiplex-readiness-audit.md §"軸 2"
// Related: refs/docs/issues/0005-refs-dmail-metadata-project-id.md
//
// Rival Contract metadata is an open map[string]string sourced from D-Mail
// YAML frontmatter. Multiplex-mode producers (project_id / worktree_id /
// notify_slack and any future v1.2+ keys) MUST be safely ignored by the
// parser so that downstream tools that have not yet learned those keys keep
// working. This test locks that contract.
//
// Sameform note: this test is byte-identical (modulo package name) across
// sightjack / paintress / amadeus / dominator. Do not introduce
// tool-specific helpers or imports.

// TestParseRivalContractMetadata_MultiplexKeysIgnored asserts that the
// parser silently ignores v1.2 multiplex keys (project_id, worktree_id,
// notify_slack) and arbitrary unknown keys, while still parsing the v1
// required fields and the v1.1 optional domain_style field correctly.
//
// Why each unknown key must be ignored:
//   - project_id / worktree_id / notify_slack are v1.2 multiplex-mode
//     producer hints; tools that predate v1.2 must still parse the
//     metadata cleanly so multi-CWD operation does not break older
//     consumers (Issue 0014 軸 2, Issue 0005).
//   - decoy_random confirms the parser does not infer behavior from any
//     unknown key — only the documented keys are honored, mirroring the
//     v1.1 DomainStyleOmitted_NoInference contract.
func TestParseRivalContractMetadata_MultiplexKeysIgnored(t *testing.T) {
	// given metadata containing v1 required keys + v1.1 optional
	// domain_style + v1.2 multiplex keys + an adversarial decoy key.
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       "test-multiplex-keys",
		"contract_revision": "1",
		"supersedes":        "",
		"domain_style":      "event-sourced",
		// v1.2 multiplex keys (unknown to the v1/v1.1 parser, must be ignored)
		"project_id":   "foo",
		"worktree_id":  "feat-baz",
		"notify_slack": "false",
		// adversarial decoy key to confirm no inference from arbitrary keys
		"decoy_random": "should-be-ignored",
	}

	// when
	parsed, ok, err := policy.ParseRivalContractMetadata(meta)

	// then — parser must accept the metadata cleanly.
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true, got false")
	}

	// v1 required + v1.1 optional fields must parse correctly.
	if parsed.Schema != policy.SchemaRivalContractV1 {
		t.Errorf("Schema: got %q, want %q", parsed.Schema, policy.SchemaRivalContractV1)
	}
	if parsed.ID != "test-multiplex-keys" {
		t.Errorf("ID: got %q, want %q", parsed.ID, "test-multiplex-keys")
	}
	if parsed.Revision != 1 {
		t.Errorf("Revision: got %d, want %d", parsed.Revision, 1)
	}
	if parsed.Supersedes != "" {
		t.Errorf("Supersedes: got %q, want \"\"", parsed.Supersedes)
	}
	if parsed.DomainStyle != policy.DomainStyleEventSourced {
		t.Errorf("DomainStyle: got %q, want %q", parsed.DomainStyle, policy.DomainStyleEventSourced)
	}

	// Multiplex keys must NOT appear on the parsed struct. The
	// RivalContractMetadata type definition has no field for project_id,
	// worktree_id, or notify_slack, so the assertion is implicit at the
	// type level: if a future change added such fields, this test would
	// need to be updated alongside the type, which is the point of locking
	// the contract here.
	//
	// We additionally guard against accidental escape into the existing
	// fields by re-checking the canonical fields above remain exactly the
	// values supplied (no smearing from the unknown keys).
}
