package session_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
)

func TestInitGateDir_SkillFilesUpdatedWhenOutdated(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// given — first init creates SKILL.md
	if err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// given — overwrite dmail-sendable SKILL.md with outdated content
	skillPath := filepath.Join(root, "skills", "dmail-sendable", "SKILL.md")
	outdated := []byte("---\nname: dmail-sendable\nmetadata:\n  produces:\n    - kind: old-kind\n---\nold\n")
	if err := os.WriteFile(skillPath, outdated, 0644); err != nil {
		t.Fatal(err)
	}

	// when — second init should overwrite with latest template
	if err := session.InitGateDir(root, logger); err != nil {
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
	if !strings.Contains(string(content), "kind: feedback") {
		t.Error("updated SKILL.md should contain 'kind: feedback'")
	}
}

func TestInitGateDir_LogsWhenSkillUpdated(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	logger := platform.NewLogger(io.Discard, false)

	// given — first init
	if err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// given — overwrite with outdated content
	skillPath := filepath.Join(root, "skills", "dmail-sendable", "SKILL.md")
	os.WriteFile(skillPath, []byte("outdated"), 0644)

	// when — second init captures log
	var buf bytes.Buffer
	logCapture := platform.NewLogger(&buf, false)
	if err := session.InitGateDir(root, logCapture); err != nil {
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
	if err := session.InitGateDir(root, logger); err != nil {
		t.Fatalf("first InitGateDir: %v", err)
	}

	// when — second init with no changes
	var buf bytes.Buffer
	logCapture := platform.NewLogger(&buf, false)
	if err := session.InitGateDir(root, logCapture); err != nil {
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
	if err := session.InitGateDir(root, logger); err != nil {
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
	if err := session.InitGateDir(root, logger); err != nil {
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
