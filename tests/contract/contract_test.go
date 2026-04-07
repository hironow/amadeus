//go:build contract

package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

const goldenDir = "testdata/golden"

func goldenFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		t.Fatal("no golden files found")
	}
	return files
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(goldenDir, name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return data
}

// TestContract_ParseDMail verifies that amadeus's ParseDMail can
// parse all cross-tool golden files.
func TestContract_ParseDMail(t *testing.T) {
	for _, name := range goldenFiles(t) {
		t.Run(name, func(t *testing.T) {
			data := readGolden(t, name)
			dm, err := domain.ParseDMail(data)
			if err != nil {
				t.Fatalf("ParseDMail error: %v", err)
			}
			if dm.Name == "" {
				t.Error("parsed name is empty")
			}
			if dm.Kind == "" {
				t.Error("parsed kind is empty")
			}
			if dm.Description == "" {
				t.Error("parsed description is empty")
			}
			if dm.SchemaVersion == "" {
				t.Error("parsed schema version is empty")
			}
		})
	}
}

// TestContract_ValidateDMailRejectsEdgeCases verifies strict validation
// rejects D-Mails with unknown kinds or future schemas.
func TestContract_ValidateDMailRejectsEdgeCases(t *testing.T) {
	data := readGolden(t, "unknown-kind.md")
	dm, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	if err := domain.ValidateKind(dm.Kind); err == nil {
		t.Error("expected ValidateKind to reject unknown kind 'advisory', but it passed")
	}

	data = readGolden(t, "future-schema.md")
	dm, err = domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	if dm.SchemaVersion == domain.DMailSchemaVersion {
		t.Errorf("expected future schema %q to differ from supported %q", dm.SchemaVersion, domain.DMailSchemaVersion)
	}
}

// TestContract_TargetsFieldPreserved verifies that amadeus's ParseDMail
// preserves the targets field.
func TestContract_TargetsFieldPreserved(t *testing.T) {
	data := readGolden(t, "amadeus-convergence.md")
	dm, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	if len(dm.Targets) != 2 {
		t.Fatalf("Targets len = %d, want 2", len(dm.Targets))
	}
	if dm.Targets[0] != "authentication" {
		t.Errorf("Targets[0] = %q, want %q", dm.Targets[0], "authentication")
	}
	if dm.Targets[1] != "session-management" {
		t.Errorf("Targets[1] = %q, want %q", dm.Targets[1], "session-management")
	}
}

// TestContract_CorrectiveMetadataRoundTrip verifies corrective-feedback.md
// golden file parses correctly and CorrectionMetadataFromMap extracts all fields.
func TestContract_CorrectiveMetadataRoundTrip(t *testing.T) {
	data := readGolden(t, "corrective-feedback.md")
	dm, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	meta := domain.CorrectionMetadataFromMap(dm.Metadata)
	if !meta.IsImprovement() {
		t.Fatal("expected IsImprovement() = true for corrective-feedback.md")
	}
	checks := map[string]string{
		"routing_mode":   string(meta.RoutingMode),
		"target_agent":   meta.TargetAgent,
		"provider_state": string(meta.ProviderState),
		"correlation_id": meta.CorrelationID,
		"trace_id":       meta.TraceID,
		"failure_type":   string(meta.FailureType),
	}
	expected := map[string]string{
		"routing_mode":   "escalate",
		"target_agent":   "sightjack",
		"provider_state": "active",
		"correlation_id": "corr-abc-123",
		"trace_id":       "trace-xyz-789",
		"failure_type":   "scope_violation",
	}
	for key, want := range expected {
		got := checks[key]
		if got != want {
			t.Errorf("metadata[%q] = %q, want %q", key, got, want)
		}
	}

	// Semantic history check: raw metadata string must use canonical ">" delimiter
	// and match domain semantics (owner=agents, routing=modes).
	rawOwner := dm.Metadata["owner_history"]
	if rawOwner != "amadeus>sightjack" {
		t.Errorf("owner_history raw = %q, want %q", rawOwner, "amadeus>sightjack")
	}
	rawRouting := dm.Metadata["routing_history"]
	if rawRouting != "retry>escalate" {
		t.Errorf("routing_history raw = %q, want %q", rawRouting, "retry>escalate")
	}
	// Parse roundtrip must produce same list
	parsedOwner := domain.ParseImprovementHistory(rawOwner)
	if len(parsedOwner) != 2 || parsedOwner[0] != "amadeus" || parsedOwner[1] != "sightjack" {
		t.Errorf("ParseImprovementHistory(owner) = %v, want [amadeus sightjack]", parsedOwner)
	}
	parsedRouting := domain.ParseImprovementHistory(rawRouting)
	if len(parsedRouting) != 2 || parsedRouting[0] != "retry" || parsedRouting[1] != "escalate" {
		t.Errorf("ParseImprovementHistory(routing) = %v, want [retry escalate]", parsedRouting)
	}
}
