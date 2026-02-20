package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocs_GeneratesMarkdown(t *testing.T) {
	// given
	info := BuildInfo{Version: "test", Commit: "none", Date: "unknown"}
	root := NewRootCommand(info)
	outDir := t.TempDir()
	root.SetArgs([]string{"docs", "--output", outDir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// root doc should exist
	rootDoc := filepath.Join(outDir, "amadeus.md")
	if _, err := os.Stat(rootDoc); os.IsNotExist(err) {
		t.Error("expected amadeus.md to be generated")
	}

	// at least one subcommand doc should exist
	versionDoc := filepath.Join(outDir, "amadeus_version.md")
	if _, err := os.Stat(versionDoc); os.IsNotExist(err) {
		t.Error("expected amadeus_version.md to be generated")
	}
}

func TestDocs_CreatesOutputDirectory(t *testing.T) {
	// given — output directory does not exist yet
	info := BuildInfo{Version: "test", Commit: "none", Date: "unknown"}
	root := NewRootCommand(info)
	outDir := filepath.Join(t.TempDir(), "nested", "cli-docs")
	root.SetArgs([]string{"docs", "--output", outDir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rootDoc := filepath.Join(outDir, "amadeus.md")
	if _, err := os.Stat(rootDoc); os.IsNotExist(err) {
		t.Error("expected amadeus.md to be generated in auto-created directory")
	}
}

func TestDocs_OutputFlagRequired(t *testing.T) {
	// given
	info := BuildInfo{Version: "test"}
	root := NewRootCommand(info)

	// then
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "docs" {
			found = true
			if f := sub.Flags().Lookup("output"); f == nil {
				t.Error("expected --output flag on docs command")
			}
		}
	}
	if !found {
		t.Fatal("expected docs subcommand to exist")
	}
}
