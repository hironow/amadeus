package amadeus

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitDivergenceDir_CreatesStructure(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	err := InitDivergenceDir(root)
	if err != nil {
		t.Fatalf("InitDivergenceDir failed: %v", err)
	}
	for _, sub := range []string{"state", "history", "dmails"} {
		path := filepath.Join(root, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", sub, err)
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}
	configPath := filepath.Join(root, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config.yaml to exist: %v", err)
	}
}

func TestSaveAndLoadCheckResult(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
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
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
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
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
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
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
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

func TestLoadLatest_NoFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
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
