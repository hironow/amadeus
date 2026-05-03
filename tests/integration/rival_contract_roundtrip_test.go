// Package integration_test rival_contract_roundtrip_test.go: amadeus
// consumer-side round-trip tests for Rival Contract v1 / v1.1 D-Mails.
//
// These tests close the producer→consumer chain on the amadeus side.
// The producer (sightjack) writes the SoT golden at
// `<sj>/tests/integration/testdata/rival/produced/canonical-spec-v1.md`,
// which is committed byte-identically into amadeus at
// `tests/integration/testdata/rival/canonical-spec-v1.md`. Two companion
// fixtures exercise the optional v1.1 `domain_style` metadata key:
//
//	testdata/rival/legacy-spec.md       — full v1 contract, no v1.1 metadata
//	testdata/rival/event-sourced-v1.md  — full v1.1 contract, domain_style: event-sourced
//
// Each test reads its fixture from disk via `domain.ParseDMail`, then
// runs `policy.ParseRivalContractBody` and `policy.ParseRivalContractMetadata`
// against the parsed mail. Assertions check the canonical Go struct
// shape (Title, six section presence, schema, id, revision, DomainStyle)
// rather than re-asserting raw bytes — byte-level invariants are owned
// by sightjack's `rival_contract_produce_test.go` and the byte-identity
// of the consumer copy is guarded by `check_rival_canonical_fixture.sh`.
//
// Refs:
//   - refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md (Phase 1.2A)
//   - refs/plans/2026-05-03-rival-contract-v1.md (canonical body+metadata)
//   - refs/plans/2026-05-03-rival-contract-v1-1-extensions.md (domain_style)
package integration_test

import (
	"os"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/policy"
)

// loadRivalFixture reads a Rival Contract D-Mail fixture from disk and
// parses it via the canonical parsers. It returns the parsed mail, the
// parsed body, the parsed metadata, and the metadata-parser ok flag.
// Failures are reported via t.Fatalf with the fixture path attached.
func loadRivalFixture(t *testing.T, path string) (domain.DMail, policy.RivalContract, policy.RivalContractMetadata, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	mail, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail %s: %v", path, err)
	}

	body, bodyOK, bodyErr := policy.ParseRivalContractBody(mail.Body)
	if bodyErr != nil {
		t.Fatalf("ParseRivalContractBody %s: %v", path, bodyErr)
	}
	if !bodyOK {
		t.Fatalf("ParseRivalContractBody %s: ok=false (fixture is not a Rival Contract v1 body)", path)
	}

	meta, metaOK, metaErr := policy.ParseRivalContractMetadata(mail.Metadata)
	if metaErr != nil {
		t.Fatalf("ParseRivalContractMetadata %s: %v", path, metaErr)
	}
	return mail, body, meta, metaOK
}

// TestRivalContractRoundTrip_CanonicalSpecV1_ParsesIdentically asserts
// that the SoT canonical-spec-v1.md golden parses cleanly and exposes
// the canonical Go struct expectations. The same fixture content is
// committed (byte-identical) into sj/pt/dom; their equivalent tests
// must pass the same assertions.
func TestRivalContractRoundTrip_CanonicalSpecV1_ParsesIdentically(t *testing.T) {
	mail, body, meta, metaOK := loadRivalFixture(t, "testdata/rival/canonical-spec-v1.md")

	// then: D-Mail envelope is a v1 specification.
	if mail.Kind != domain.KindSpecification {
		t.Errorf("Kind: got %q, want %q", mail.Kind, domain.KindSpecification)
	}
	if mail.SchemaVersion != domain.DMailSchemaVersion {
		t.Errorf("SchemaVersion: got %q, want %q", mail.SchemaVersion, domain.DMailSchemaVersion)
	}

	// then: body parses into the canonical RivalContract struct.
	if body.Title != "Add session expiry enforcement" {
		t.Errorf("Title: got %q", body.Title)
	}
	for _, section := range []struct {
		name  string
		value string
	}{
		{"Intent", body.Intent},
		{"Domain", body.Domain},
		{"Decisions", body.Decisions},
		{"Steps", body.Steps},
		{"Boundaries", body.Boundaries},
		{"Evidence", body.Evidence},
	} {
		if strings.TrimSpace(section.value) == "" {
			t.Errorf("section %q must not be empty", section.name)
		}
	}
	// Steps must include the canonical implementation action.
	if !strings.Contains(body.Steps, "Enforce session expiry in middleware") {
		t.Errorf("Steps must contain implementation action, got %q", body.Steps)
	}
	// Boundaries must surface the issue-management action that sightjack
	// already applied.
	if !strings.Contains(body.Boundaries, "Document expiry enforcement in DoD") {
		t.Errorf("Boundaries must surface filtered issue-management action, got %q", body.Boundaries)
	}

	// then: metadata parses with full v1 semantics.
	if !metaOK {
		t.Fatal("ParseRivalContractMetadata: ok=false on canonical golden")
	}
	if meta.Schema != policy.SchemaRivalContractV1 {
		t.Errorf("Schema: got %q, want %q", meta.Schema, policy.SchemaRivalContractV1)
	}
	if meta.ID != "canonical-spec-v1" {
		t.Errorf("ID: got %q, want %q", meta.ID, "canonical-spec-v1")
	}
	if meta.Revision != 1 {
		t.Errorf("Revision: got %d, want 1", meta.Revision)
	}
	if meta.Supersedes != "" {
		t.Errorf("Supersedes: got %q, want empty (initial revision)", meta.Supersedes)
	}
	// then: legacy v1 producer does not emit domain_style — DomainStyle
	// MUST be the empty string per parser contract.
	if meta.DomainStyle != "" {
		t.Errorf("DomainStyle: got %q, want empty (legacy v1 producer omits domain_style)", meta.DomainStyle)
	}
}

// TestRivalContractRoundTrip_LegacyV1_GracefulFallback asserts that a
// full v1 contract whose metadata predates v1.1 parses cleanly: the
// metadata parser returns ok=true with a nil error and DomainStyle as
// the empty string. Producers MUST treat the empty string as the v1
// default semantically equivalent to `generic`.
func TestRivalContractRoundTrip_LegacyV1_GracefulFallback(t *testing.T) {
	mail, body, meta, metaOK := loadRivalFixture(t, "testdata/rival/legacy-spec.md")

	if mail.Kind != domain.KindSpecification {
		t.Errorf("Kind: got %q, want %q", mail.Kind, domain.KindSpecification)
	}
	if body.Title == "" {
		t.Error("Title must not be empty")
	}
	if !metaOK {
		t.Fatal("ParseRivalContractMetadata: ok=false on legacy v1 fixture")
	}
	if meta.Schema != policy.SchemaRivalContractV1 {
		t.Errorf("Schema: got %q, want %q", meta.Schema, policy.SchemaRivalContractV1)
	}
	if meta.Revision != 1 {
		t.Errorf("Revision: got %d, want 1", meta.Revision)
	}
	if meta.DomainStyle != "" {
		t.Errorf("DomainStyle: got %q, want empty (fixture has no domain_style key)", meta.DomainStyle)
	}
}

// TestRivalContractRoundTrip_EventSourcedV1_DomainStyleAccepted asserts
// that a v1.1 fixture carrying `domain_style: event-sourced` round-trips
// through the metadata parser and the canonical enum value is preserved.
func TestRivalContractRoundTrip_EventSourcedV1_DomainStyleAccepted(t *testing.T) {
	mail, body, meta, metaOK := loadRivalFixture(t, "testdata/rival/event-sourced-v1.md")

	if mail.Kind != domain.KindSpecification {
		t.Errorf("Kind: got %q, want %q", mail.Kind, domain.KindSpecification)
	}
	if body.Title == "" {
		t.Error("Title must not be empty")
	}
	if !metaOK {
		t.Fatal("ParseRivalContractMetadata: ok=false on event-sourced v1.1 fixture")
	}
	if meta.Schema != policy.SchemaRivalContractV1 {
		t.Errorf("Schema: got %q, want %q", meta.Schema, policy.SchemaRivalContractV1)
	}
	if meta.DomainStyle != policy.DomainStyleEventSourced {
		t.Errorf("DomainStyle: got %q, want %q", meta.DomainStyle, policy.DomainStyleEventSourced)
	}
	// And the body's Domain section reflects event-sourced grammar.
	if !strings.Contains(body.Domain, "Event:") || !strings.Contains(body.Domain, "Aggregate:") {
		t.Errorf("Domain section should carry event-sourced vocabulary (Event:/Aggregate:), got %q", body.Domain)
	}
}
