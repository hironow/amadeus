package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
)

// refs issue 0032 / decision D5(a): `amadeus init` materializes the
// entry skill into the target project's .claude/skills/ so a bare
// `claude` session auto-discovers /review-gate without plugin
// machinery, and `mcp-config generate` upserts the project-root
// .mcp.json so the server auto-attaches (conformance constraints
// C4/C5).

func TestInstallClaudeSkills_MaterializesEntrySkill(t *testing.T) {
	// given
	baseDir := t.TempDir()

	// when
	if err := session.InstallClaudeSkills(baseDir, platform.ClaudeSkillsFS, nil); err != nil {
		t.Fatalf("InstallClaudeSkills: %v", err)
	}

	// then
	skillPath := filepath.Join(baseDir, ".claude", "skills", "review-gate", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("entry skill not materialized: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "name: review-gate") {
		t.Errorf("materialized skill missing frontmatter name:\n%.200s", text)
	}
}

func TestInstallClaudeSkills_IsIdempotent(t *testing.T) {
	// given
	baseDir := t.TempDir()
	if err := session.InstallClaudeSkills(baseDir, platform.ClaudeSkillsFS, nil); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// when: second run must not error and must keep the file
	if err := session.InstallClaudeSkills(baseDir, platform.ClaudeSkillsFS, nil); err != nil {
		t.Fatalf("second install: %v", err)
	}

	// then
	if _, err := os.Stat(filepath.Join(baseDir, ".claude", "skills", "review-gate", "SKILL.md")); err != nil {
		t.Errorf("skill missing after idempotent re-run: %v", err)
	}
}

func TestUpsertRootMCPConfig_CreatesWhenAbsent(t *testing.T) {
	// given
	baseDir := t.TempDir()

	// when
	path, err := session.UpsertRootMCPConfig(baseDir)
	if err != nil {
		t.Fatalf("UpsertRootMCPConfig: %v", err)
	}

	// then
	if path != filepath.Join(baseDir, ".mcp.json") {
		t.Errorf("path = %s, want project-root .mcp.json", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read root .mcp.json: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	entry, _ := servers["amadeus"].(map[string]any)
	if entry == nil || entry["command"] != "amadeus" {
		t.Errorf("amadeus server entry missing: %v", cfg)
	}
}

func TestUpsertRootMCPConfig_PreservesOtherEntriesAndKeys(t *testing.T) {
	// given: a root .mcp.json with a foreign server and a foreign top-level key
	baseDir := t.TempDir()
	existing := `{
  "mcpServers": {
    "k6": {"command": "mcp-k6", "args": ["--transport", "stdio"], "timeout": 30000}
  },
  "somethingElse": {"keep": true}
}`
	if err := os.WriteFile(filepath.Join(baseDir, ".mcp.json"), []byte(existing), 0o644); err != nil {
		t.Fatalf("seed root config: %v", err)
	}

	// when
	if _, err := session.UpsertRootMCPConfig(baseDir); err != nil {
		t.Fatalf("UpsertRootMCPConfig: %v", err)
	}

	// then: k6 entry (with its extra fields), foreign key, and amadeus all present
	data, err := os.ReadFile(filepath.Join(baseDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := cfg["somethingElse"]; !ok {
		t.Error("foreign top-level key dropped by upsert")
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	k6, _ := servers["k6"].(map[string]any)
	if k6 == nil || k6["timeout"] != float64(30000) {
		t.Errorf("k6 entry mangled: %v", servers)
	}
	if _, ok := servers["amadeus"]; !ok {
		t.Errorf("amadeus entry missing after upsert: %v", servers)
	}
}

func TestUpsertRootMCPConfig_IsIdempotent(t *testing.T) {
	// given
	baseDir := t.TempDir()
	if _, err := session.UpsertRootMCPConfig(baseDir); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(baseDir, ".mcp.json"))

	// when
	if _, err := session.UpsertRootMCPConfig(baseDir); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(baseDir, ".mcp.json"))

	// then
	if string(first) != string(second) {
		t.Errorf("upsert not idempotent:\nfirst:  %s\nsecond: %s", first, second)
	}
}
