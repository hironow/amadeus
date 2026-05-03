package policy_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/harness/policy"
)

// Phase 1.1A — DomainStyle metadata tests (Rival Contract v1.1).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1A"
//
// amadeus is a CONSUMER of the Rival Contract v1.1 DomainStyle field. The
// canonical source is sightjack/internal/harness/filter/rival_contract.go
// (sj is the v1.1 producer). amadeus copy-syncs the parser-side surface so
// the divergence prompt path can branch on `metadata.domain_style` without
// importing across tool boundaries.
//
// The parser MUST NOT infer domain_style from environment, ADRs, or any
// other side-channel: missing => DomainStyle == "" (treated as "generic"
// by consumers). Only producers may set the value.

func TestParseRivalContractMetadata_DomainStyleAccepted_AllValues(t *testing.T) {
	// given the three accepted enum values
	cases := []string{
		policy.DomainStyleEventSourced,
		policy.DomainStyleGeneric,
		policy.DomainStyleMixed,
	}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			meta := map[string]string{
				"contract_schema":   "rival-contract-v1",
				"contract_id":       "auth-x",
				"contract_revision": "1",
				"domain_style":      value,
			}

			// when
			parsed, ok, err := policy.ParseRivalContractMetadata(meta)

			// then
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Fatal("expected ok=true for valid metadata")
			}
			if parsed.DomainStyle != value {
				t.Errorf("DomainStyle: got %q, want %q", parsed.DomainStyle, value)
			}
		})
	}
}

func TestParseRivalContractMetadata_DomainStyleRejected_UnknownValue(t *testing.T) {
	// given an unknown enum value
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       "auth-x",
		"contract_revision": "1",
		"domain_style":      "foo",
	}

	// when
	_, ok, err := policy.ParseRivalContractMetadata(meta)

	// then
	if err == nil {
		t.Fatal("expected error for unknown domain_style value 'foo'")
	}
	if ok {
		t.Errorf("expected ok=false on error, got ok=true")
	}
	if !strings.Contains(err.Error(), "domain_style") {
		t.Errorf("error should mention domain_style, got: %v", err)
	}
}

func TestParseRivalContractMetadata_DomainStyleOmitted_LegacyOK(t *testing.T) {
	// given metadata with no domain_style key (v1 legacy shape)
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       "auth-x",
		"contract_revision": "1",
	}

	// when
	parsed, ok, err := policy.ParseRivalContractMetadata(meta)

	// then
	if err != nil {
		t.Fatalf("legacy v1 metadata must parse cleanly, got %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for legacy v1 metadata")
	}
	if parsed.DomainStyle != "" {
		t.Errorf("DomainStyle: expected \"\" for legacy v1, got %q", parsed.DomainStyle)
	}
}

// TestParseRivalContractMetadata_DomainStyleOmitted_NoInference asserts that
// the parser does not consult environment variables, ADR files, or any other
// side channel when domain_style is missing. The parser is pure: missing =>
// "". Inference only happens at producer time.
func TestParseRivalContractMetadata_DomainStyleOmitted_NoInference(t *testing.T) {
	// given metadata with no domain_style key, plus adversarial decoy keys
	// that COULD be misread as a hint if the parser were inferring.
	meta := map[string]string{
		"contract_schema":   "rival-contract-v1",
		"contract_id":       "auth-x",
		"contract_revision": "1",
		// Adversarial decoy keys that MUST NOT influence the parser.
		"adr_event_sourcing": "docs/adr/0001-event-sourcing.md",
		"event_sourced_hint": "true",
	}

	// when
	parsed, ok, err := policy.ParseRivalContractMetadata(meta)

	// then
	if err != nil {
		t.Fatalf("parser must ignore decoy keys, got %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for legacy-shape metadata")
	}
	if parsed.DomainStyle != "" {
		t.Errorf("parser inferred DomainStyle=%q from non-domain_style keys; must be \"\"", parsed.DomainStyle)
	}
}
