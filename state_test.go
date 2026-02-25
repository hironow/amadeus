package amadeus

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitGateDir_CreatesStructure(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	err := InitGateDir(root)
	if err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}
	for _, sub := range []string{".run", "events", "outbox", "inbox", "archive"} {
		path := filepath.Join(root, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}
	configPath := filepath.Join(root, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config.yaml to exist: %v", err)
	}
	// .gitignore must exist and contain .run/
	gitignorePath := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("expected .gitignore to exist: %v", err)
	}
	if !strings.Contains(string(data), ".run/") {
		t.Errorf("expected .gitignore to contain '.run/', got: %s", string(data))
	}
}

func TestSaveAndLoadCheckResult(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	result := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 14, 30, 0, 0, time.UTC),
		Commit:     "a1b2c3d",
		Type:       CheckTypeDiff,
		Divergence: 0.145,
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 15, Details: "minor"},
			AxisDoD:        {Score: 20, Details: "edge case"},
			AxisDependency: {Score: 10, Details: "clean"},
			AxisImplicit:   {Score: 5, Details: "naming"},
		},
		PRsEvaluated: []string{"#120", "#122"},
	}
	store := NewProjectionStore(root)
	if err := store.SaveLatest(result); err != nil {
		t.Fatalf("SaveLatest failed: %v", err)
	}
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if loaded.Commit != "a1b2c3d" {
		t.Errorf("expected commit a1b2c3d, got %s", loaded.Commit)
	}
	if loaded.Divergence != 0.145 {
		t.Errorf("expected divergence 0.145, got %f", loaded.Divergence)
	}
}

func TestInitGateDir_MigratesLegacyState(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// given: legacy state/ directory with latest.json and baseline.json
	legacyDir := filepath.Join(root, "state")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	latestData := []byte(`{"commit":"legacy-abc","divergence":0.123}`)
	baselineData := []byte(`{"commit":"legacy-base","divergence":0.050}`)
	if err := os.WriteFile(filepath.Join(legacyDir, "latest.json"), latestData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "baseline.json"), baselineData, 0o644); err != nil {
		t.Fatal(err)
	}

	// when: InitGateDir is called
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: files should be migrated to .run/
	newLatest, err := os.ReadFile(filepath.Join(root, ".run", "latest.json"))
	if err != nil {
		t.Fatalf("expected .run/latest.json to exist: %v", err)
	}
	if string(newLatest) != string(latestData) {
		t.Errorf("expected migrated latest data to match, got: %s", string(newLatest))
	}
	newBaseline, err := os.ReadFile(filepath.Join(root, ".run", "baseline.json"))
	if err != nil {
		t.Fatalf("expected .run/baseline.json to exist: %v", err)
	}
	if string(newBaseline) != string(baselineData) {
		t.Errorf("expected migrated baseline data to match, got: %s", string(newBaseline))
	}

	// then: legacy state/ directory should be removed
	if _, err := os.Stat(legacyDir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected legacy state/ directory to be removed, but it still exists")
	}
}

func TestInitGateDir_SkipsMigrationWhenNoLegacy(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// given: no legacy state/ directory
	// when: InitGateDir is called
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: .run/ should exist but be empty (no migrated files)
	entries, err := os.ReadDir(filepath.Join(root, ".run"))
	if err != nil {
		t.Fatalf("expected .run/ to exist: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty .run/, got %d entries", len(entries))
	}
}

func TestInitGateDir_DoesNotOverwriteExistingRun(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// given: .run/ already has latest.json AND state/ also exists (edge case)
	runDir := filepath.Join(root, ".run")
	stateDir := filepath.Join(root, "state")
	os.MkdirAll(runDir, 0o755)
	os.MkdirAll(stateDir, 0o755)
	os.MkdirAll(filepath.Join(root, "history"), 0o755)
	os.MkdirAll(filepath.Join(root, "outbox"), 0o755)
	os.MkdirAll(filepath.Join(root, "inbox"), 0o755)
	os.MkdirAll(filepath.Join(root, "archive"), 0o755)

	existingData := []byte(`{"commit":"existing","divergence":0.999}`)
	legacyData := []byte(`{"commit":"legacy","divergence":0.001}`)
	os.WriteFile(filepath.Join(runDir, "latest.json"), existingData, 0o644)
	os.WriteFile(filepath.Join(stateDir, "latest.json"), legacyData, 0o644)

	// when: InitGateDir is called
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: .run/latest.json should NOT be overwritten by legacy data
	data, err := os.ReadFile(filepath.Join(runDir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(existingData) {
		t.Errorf("expected existing .run/latest.json to be preserved, got: %s", string(data))
	}
}

func TestInitGateDir_AppendsEntriesToExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	os.MkdirAll(root, 0o755)

	// given: existing .gitignore without required entries
	gitignorePath := filepath.Join(root, ".gitignore")
	os.WriteFile(gitignorePath, []byte("*.log\ntemp/\n"), 0o644)

	// when
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: .gitignore should contain all required entries
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, entry := range []string{".run/", "outbox/", "inbox/"} {
		if !strings.Contains(content, entry) {
			t.Errorf("expected .gitignore to contain %q, got: %s", entry, content)
		}
	}
	// then: original content should be preserved
	if !strings.Contains(content, "*.log") {
		t.Errorf("expected .gitignore to preserve original content, got: %s", content)
	}
}

func TestInitGateDir_SkipsGitignoreAppendIfAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	os.MkdirAll(root, 0o755)

	// given: existing .gitignore that already has all required entries
	gitignorePath := filepath.Join(root, ".gitignore")
	original := "*.log\n.run/\noutbox/\ninbox/\ntemp/\n"
	os.WriteFile(gitignorePath, []byte(original), 0o644)

	// when
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: .gitignore should be unchanged
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("expected .gitignore to be unchanged, got: %s", string(data))
	}
}

func TestInitGateDir_CreatesSkillDirectories(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// when
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: skills directories should exist
	for _, sub := range []string{
		filepath.Join("skills", "dmail-sendable"),
		filepath.Join("skills", "dmail-readable"),
	} {
		path := filepath.Join(root, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}
}

func TestInitGateDir_CreatesSkillMDFiles(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// when
	if err := InitGateDir(root); err != nil {
		t.Fatalf("InitGateDir failed: %v", err)
	}

	// then: dmail-sendable/SKILL.md should exist with metadata format
	sendablePath := filepath.Join(root, "skills", "dmail-sendable", "SKILL.md")
	sendableData, err := os.ReadFile(sendablePath)
	if err != nil {
		t.Fatalf("expected dmail-sendable/SKILL.md to exist: %v", err)
	}
	sendableContent := string(sendableData)
	if !strings.Contains(sendableContent, "name: dmail-sendable") {
		t.Errorf("expected SKILL.md to contain 'name: dmail-sendable', got:\n%s", sendableContent)
	}
	if !strings.Contains(sendableContent, "license: Apache-2.0") {
		t.Errorf("expected SKILL.md to contain 'license: Apache-2.0', got:\n%s", sendableContent)
	}
	if !strings.Contains(sendableContent, "dmail-schema-version:") {
		t.Errorf("expected SKILL.md to contain 'dmail-schema-version:', got:\n%s", sendableContent)
	}
	if !strings.Contains(sendableContent, "produces:") {
		t.Errorf("expected SKILL.md to contain 'produces:', got:\n%s", sendableContent)
	}
	if !strings.Contains(sendableContent, "kind: feedback") {
		t.Errorf("expected SKILL.md to contain 'kind: feedback', got:\n%s", sendableContent)
	}
	if !strings.Contains(sendableContent, "kind: convergence") {
		t.Errorf("expected SKILL.md to contain 'kind: convergence', got:\n%s", sendableContent)
	}

	// then: dmail-readable/SKILL.md should exist with metadata format
	readablePath := filepath.Join(root, "skills", "dmail-readable", "SKILL.md")
	readableData, err := os.ReadFile(readablePath)
	if err != nil {
		t.Fatalf("expected dmail-readable/SKILL.md to exist: %v", err)
	}
	readableContent := string(readableData)
	if !strings.Contains(readableContent, "name: dmail-readable") {
		t.Errorf("expected SKILL.md to contain 'name: dmail-readable', got:\n%s", readableContent)
	}
	if !strings.Contains(readableContent, "license: Apache-2.0") {
		t.Errorf("expected SKILL.md to contain 'license: Apache-2.0', got:\n%s", readableContent)
	}
	if !strings.Contains(readableContent, "consumes:") {
		t.Errorf("expected SKILL.md to contain 'consumes:', got:\n%s", readableContent)
	}
	if !strings.Contains(readableContent, "kind: report") {
		t.Errorf("expected SKILL.md to contain 'kind: report', got:\n%s", readableContent)
	}
}

func TestInitGateDir_DoesNotOverwriteExistingSkillMD(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// given: first init creates SKILL.md files
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	// given: modify the SKILL.md to simulate user customization
	customPath := filepath.Join(root, "skills", "dmail-sendable", "SKILL.md")
	customContent := []byte("---\nname: dmail-sendable\ndescription: custom\n---\n")
	if err := os.WriteFile(customPath, customContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// when: re-init
	if err := InitGateDir(root); err != nil {
		t.Fatalf("second InitGateDir failed: %v", err)
	}

	// then: custom content should be preserved
	data, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(customContent) {
		t.Errorf("expected custom SKILL.md to be preserved, got:\n%s", string(data))
	}
}

func TestLoadLatest_NoFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	result, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Commit != "" {
		t.Errorf("expected empty commit, got %s", result.Commit)
	}
}
