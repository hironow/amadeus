package session_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"gopkg.in/yaml.v3"
)

func TestInitGateDir_SkillFilesUpdatedWhenOutdated(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// given — first init creates SKILL.md
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// given — overwrite dmail-sendable SKILL.md with outdated content
	skillPath := filepath.Join(root, "skills", "dmail-sendable", "SKILL.md")
	outdated := []byte("---\nname: dmail-sendable\nmetadata:\n  produces:\n    - kind: old-kind\n---\nold\n")
	if err := os.WriteFile(skillPath, outdated, 0644); err != nil {
		t.Fatal(err)
	}

	// when — second init should overwrite with latest template
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("second InitGateDir: %v", err)
	}

	// then — SKILL.md should contain latest template content, not outdated
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if strings.Contains(string(content), "old-kind") {
		t.Error("outdated SKILL.md should be overwritten with latest template")
	}
	if !strings.Contains(string(content), "kind: design-feedback") {
		t.Error("updated SKILL.md should contain 'kind: design-feedback'")
	}
}

func TestInitGateDir_LogsWhenSkillUpdated(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// given — first init
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// given — overwrite with outdated content
	skillPath := filepath.Join(root, "skills", "dmail-sendable", "SKILL.md")
	os.WriteFile(skillPath, []byte("outdated"), 0644)

	// when — second init captures log
	var buf bytes.Buffer
	logCapture := platform.NewLogger(&buf, false)
	if _, err := session.InitGateDir(root, logCapture); err != nil {
		t.Fatalf("second InitGateDir: %v", err)
	}

	// then — log mentions updated skill
	output := buf.String()
	if !strings.Contains(output, "dmail-sendable") {
		t.Errorf("expected log to mention dmail-sendable, got: %q", output)
	}
}

func TestInitGateDir_NoLogWhenSkillUnchanged(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// given — first init
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// when — second init with no changes
	var buf bytes.Buffer
	logCapture := platform.NewLogger(&buf, false)
	if _, err := session.InitGateDir(root, logCapture); err != nil {
		t.Fatalf("second InitGateDir: %v", err)
	}

	// then — no SKILL.md update log
	output := buf.String()
	if strings.Contains(output, "SKILL.md") {
		t.Errorf("should not log when skills are unchanged, got: %q", output)
	}
}

func TestInitGateDir_GitignoreIncludesEvents(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// when
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("InitGateDir: %v", err)
	}

	// then
	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(content), "events/") {
		t.Errorf(".gitignore should contain events/, got: %q", string(content))
	}
}

func TestInitGateDir_AppendsEventsToExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	os.MkdirAll(root, 0755)
	logger := platform.NewLogger(io.Discard, false)

	// given — legacy .gitignore without events/
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".run/\noutbox/\ninbox/\n.otel.env\n"), 0644)

	// when
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("InitGateDir: %v", err)
	}

	// then
	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(content), "events/") {
		t.Errorf(".gitignore should contain events/ after upgrade, got: %q", string(content))
	}
}

func TestInitGateDir_ConfigCreatedWithDefaults(t *testing.T) {
	// given — fresh directory
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// when
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("InitGateDir: %v", err)
	}

	// then — config.yaml should exist with default values
	data, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}

	var cfg domain.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Lang != "ja" {
		t.Errorf("lang = %q, want %q", cfg.Lang, "ja")
	}
	if cfg.Convergence.WindowDays != 14 {
		t.Errorf("convergence.window_days = %d, want 14", cfg.Convergence.WindowDays)
	}
	if cfg.FullCheck.Interval != 10 {
		t.Errorf("full_check.interval = %d, want 10", cfg.FullCheck.Interval)
	}
}

func TestInitGateDir_ConfigMergesExisting(t *testing.T) {
	// given — first init creates config with defaults
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// given — user modifies lang to "en"
	configPath := filepath.Join(root, "config.yaml")
	data, _ := os.ReadFile(configPath)
	modified := strings.Replace(string(data), `lang: ja`, `lang: en`, 1)
	os.WriteFile(configPath, []byte(modified), 0644)

	// when — second init should merge (preserve user's lang)
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("second InitGateDir: %v", err)
	}

	// then — user's lang preserved, defaults still present
	result, _ := os.ReadFile(configPath)
	var cfg domain.Config
	if err := yaml.Unmarshal(result, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Lang != "en" {
		t.Errorf("lang = %q, want %q (user value should be preserved)", cfg.Lang, "en")
	}
	if cfg.Convergence.WindowDays != 14 {
		t.Errorf("convergence.window_days = %d, want 14 (default should persist)", cfg.Convergence.WindowDays)
	}
}

func TestInitGateDir_ConfigAddsNewFields(t *testing.T) {
	// given — existing config missing some fields (simulates upgrade)
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	os.MkdirAll(root, 0755)
	logger := platform.NewLogger(io.Discard, false)

	// Write a minimal config (missing convergence, full_check)
	minimal := []byte("lang: en\n")
	os.WriteFile(filepath.Join(root, "config.yaml"), minimal, 0644)

	// when
	if _, err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("InitGateDir: %v", err)
	}

	// then — new fields from defaults should appear
	result, _ := os.ReadFile(filepath.Join(root, "config.yaml"))
	var cfg domain.Config
	if err := yaml.Unmarshal(result, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Lang != "en" {
		t.Errorf("lang = %q, want %q (existing value)", cfg.Lang, "en")
	}
	if cfg.Convergence.WindowDays != 14 {
		t.Errorf("convergence.window_days = %d, want 14 (new default)", cfg.Convergence.WindowDays)
	}
	if cfg.FullCheck.Interval != 10 {
		t.Errorf("full_check.interval = %d, want 10 (new default)", cfg.FullCheck.Interval)
	}
}
