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
	for _, sub := range []string{".run", "history", "outbox", "inbox", "archive"} {
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
	store := NewStateStore(root)
	if err := store.SaveLatest(result); err != nil {
		t.Fatalf("SaveLatest failed: %v", err)
	}
	if err := store.SaveHistory(result); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
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

func TestSaveHistory_SecondPrecision(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: two results 30 seconds apart within the same minute
	r1 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 14, 30, 0, 0, time.UTC),
		Commit:     "aaa",
		Divergence: 0.10,
	}
	r2 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 14, 30, 30, 0, time.UTC),
		Commit:     "bbb",
		Divergence: 0.20,
	}

	// when
	if err := store.SaveHistory(r1); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveHistory(r2); err != nil {
		t.Fatal(err)
	}

	// then: both files must exist (not overwritten)
	entries, err := os.ReadDir(filepath.Join(root, "history"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected 2 history files, got %d: %v", len(entries), names)
	}
}

func TestSaveHistory_SameSecond_NoClobber(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: two results at the exact same second
	ts := time.Date(2026, 2, 19, 14, 30, 0, 0, time.UTC)
	r1 := CheckResult{CheckedAt: ts, Commit: "aaa", Divergence: 0.10}
	r2 := CheckResult{CheckedAt: ts, Commit: "bbb", Divergence: 0.20}

	// when
	if err := store.SaveHistory(r1); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveHistory(r2); err != nil {
		t.Fatal(err)
	}

	// then: both files must exist (not overwritten)
	entries, err := os.ReadDir(filepath.Join(root, "history"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected 2 history files, got %d: %v", len(entries), names)
	}
}

func TestSaveHistory_UnreadableDir_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: history directory with no read/execute permission
	histDir := filepath.Join(root, "history")
	if err := os.Chmod(histDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(histDir, 0o755) })

	// when: SaveHistory is called
	r := CheckResult{
		CheckedAt: time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC),
		Commit:    "aaa",
	}
	err := store.SaveHistory(r)

	// then: should return an error, not loop forever
	if err == nil {
		t.Error("expected error when history directory is unreadable")
	}
}

func TestLoadHistory_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: three history entries at different times
	r1 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		Commit:     "aaa",
		Type:       CheckTypeFull,
		Divergence: 0.10,
	}
	r2 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
		Commit:     "bbb",
		Type:       CheckTypeDiff,
		Divergence: 0.13,
	}
	r3 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Commit:     "ccc",
		Type:       CheckTypeDiff,
		Divergence: 0.15,
	}
	for _, r := range []CheckResult{r1, r2, r3} {
		if err := store.SaveHistory(r); err != nil {
			t.Fatal(err)
		}
	}

	// when
	history, err := store.LoadHistory()

	// then
	if err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}
	// sorted newest first (descending)
	if history[0].Commit != "ccc" {
		t.Errorf("expected newest first (ccc), got %s", history[0].Commit)
	}
	if history[2].Commit != "aaa" {
		t.Errorf("expected oldest last (aaa), got %s", history[2].Commit)
	}
}

func TestLoadHistory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	history, err := store.LoadHistory()

	// then
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 entries, got %d", len(history))
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
	original := "*.log\n.run/\noutbox/\ninbox/\npending/\nrejected/\ntemp/\n"
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
	store := NewStateStore(root)
	result, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Commit != "" {
		t.Errorf("expected empty commit, got %s", result.Commit)
	}
}
