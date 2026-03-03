//go:build contract

package domain

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
// amadeus ValidateDMail returns []string (error list), not error.
func TestContract_ValidateDMailRejectsEdgeCases(t *testing.T) {
	cases := []struct {
		file   string
		reason string
	}{
		{"unknown-kind.md", "kind 'advisory' not in valid kinds"},
		{"future-schema.md", "dmail-schema-version '2' not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data := readContractGolden(t, tc.file)
			dm, err := ParseDMail(data)
			if err != nil {
				t.Fatalf("ParseDMail error: %v", err)
			}
			errs := ValidateDMail(dm)
			if len(errs) == 0 {
				t.Errorf("expected ValidateDMail to fail (%s), but it passed", tc.reason)
			}
		})
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
