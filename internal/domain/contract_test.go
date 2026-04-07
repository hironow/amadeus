//go:build contract

package domain

// white-box-reason: contract validation: tests unexported golden file enumeration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const contractGoldenDir = "testdata/contract"

func contractGoldenFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(contractGoldenDir)
	if err != nil {
		t.Fatalf("read contract golden dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		t.Fatal("no contract golden files found")
	}
	return files
}

func readContractGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractGoldenDir, name))
	if err != nil {
		t.Fatalf("read contract golden %s: %v", name, err)
	}
	return data
}

// TestContract_ParseDMail verifies that amadeus's ParseDMail can
// parse all cross-tool golden files. Amadeus is Postel-liberal at
// the parse level — unknown kinds and future schemas parse without error.
func TestContract_ParseDMail(t *testing.T) {
	for _, name := range contractGoldenFiles(t) {
		t.Run(name, func(t *testing.T) {
			data := readContractGolden(t, name)
			dm, err := ParseDMail(data)
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

// TestContract_ValidateDMailRejectsEdgeCases verifies that amadeus's
// strict validation rejects D-Mails with unknown kinds or future schemas.
func TestContract_ValidateDMailRejectsEdgeCases(t *testing.T) {
	// unknown-kind.md has kind "advisory" — should be rejected by ValidateKind
	data := readContractGolden(t, "unknown-kind.md")
	dm, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	if err := ValidateKind(dm.Kind); err == nil {
		t.Error("expected ValidateKind to reject unknown kind 'advisory', but it passed")
	}

	// future-schema.md has dmail-schema-version "2" — should differ from supported version
	data = readContractGolden(t, "future-schema.md")
	dm, err = ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	if dm.SchemaVersion == DMailSchemaVersion {
		t.Errorf("expected future schema %q to differ from supported %q", dm.SchemaVersion, DMailSchemaVersion)
	}
}

// TestContract_TargetsFieldPreserved verifies that amadeus's ParseDMail
// preserves the targets field which is amadeus-specific.
func TestContract_TargetsFieldPreserved(t *testing.T) {
	data := readContractGolden(t, "amadeus-convergence.md")
	dm, err := ParseDMail(data)
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

// TestContract_CorrectiveMetadataRoundTrip verifies that corrective-feedback.md
// golden file parses correctly and CorrectionMetadataFromMap extracts all fields.
func TestContract_CorrectiveMetadataRoundTrip(t *testing.T) {
	data := readContractGolden(t, "corrective-feedback.md")
	dm, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail error: %v", err)
	}
	meta := CorrectionMetadataFromMap(dm.Metadata)
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
}
