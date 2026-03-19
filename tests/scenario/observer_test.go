//go:build scenario

package scenario_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Observer provides high-level assertion helpers for scenario tests.
// It wraps a Workspace and testing.T to verify mailbox state, D-Mail
// content, and closed-loop completion.
type Observer struct {
	ws *Workspace
	t  *testing.T
}

// NewObserver creates an Observer for the given workspace.
func NewObserver(ws *Workspace, t *testing.T) *Observer {
	return &Observer{ws: ws, t: t}
}

// AssertMailboxState verifies file counts in mailbox directories.
// Keys are relative paths like ".siren/inbox", ".expedition/archive".
func (o *Observer) AssertMailboxState(expectations map[string]int) {
	o.t.Helper()
	for relPath, want := range expectations {
		dir := filepath.Join(o.ws.RepoPath, relPath)
		got := o.ws.CountFiles(o.t, dir)
		if got != want {
			o.t.Errorf("mailbox %s: got %d files, want %d", relPath, got, want)
		}
	}
}

// AssertAllOutboxEmpty verifies that all tool outboxes contain no .md files.
func (o *Observer) AssertAllOutboxEmpty() {
	o.t.Helper()
	tools := []string{".siren", ".expedition", ".gate"}
	for _, tool := range tools {
		dir := filepath.Join(o.ws.RepoPath, tool, "outbox")
		files := o.ws.ListFiles(o.t, dir)
		var mdFiles []string
		for _, f := range files {
			if strings.HasSuffix(f, ".md") {
				mdFiles = append(mdFiles, f)
			}
		}
		if len(mdFiles) > 0 {
			o.t.Errorf("%s/outbox not empty: %v", tool, mdFiles)
		}
	}
}

// AssertArchiveContains verifies that a tool's archive directory contains
// D-Mail files with the expected kinds in their frontmatter.
func (o *Observer) AssertArchiveContains(toolDir string, kinds []string) {
	o.t.Helper()
	dir := filepath.Join(o.ws.RepoPath, toolDir, "archive")
	files := o.ws.ListFiles(o.t, dir)
	if len(files) == 0 && len(kinds) > 0 {
		o.t.Errorf("%s/archive: expected D-Mails with kinds %v, but archive is empty", toolDir, kinds)
		return
	}

	// Collect all kinds found in archive
	foundKinds := make(map[string]bool)
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		path := filepath.Join(dir, f)
		fm, _ := o.ws.ReadDMail(o.t, path)
		if kind, ok := fm["kind"].(string); ok {
			foundKinds[kind] = true
		}
	}

	for _, want := range kinds {
		if !foundKinds[want] {
			o.t.Errorf("%s/archive: missing D-Mail with kind %q (found kinds: %v)", toolDir, want, foundKinds)
		}
	}
}

// AssertDMailKind verifies that a D-Mail file has the expected kind.
func (o *Observer) AssertDMailKind(path, expectedKind string) {
	o.t.Helper()
	fm, _ := o.ws.ReadDMail(o.t, path)
	kind, ok := fm["kind"].(string)
	if !ok {
		o.t.Errorf("D-Mail %s: missing kind field in frontmatter", path)
		return
	}
	if kind != expectedKind {
		o.t.Errorf("D-Mail %s: got kind %q, want %q", path, kind, expectedKind)
	}
}

// WaitForClosedLoop waits for a complete closed loop (report -> feedback).
// It polls all 3 delivery points:
//  1. feedback in .expedition/inbox (delivered by phonewave from .gate/outbox)
//  2. report in .gate/archive (amadeus consumed from .gate/inbox and archived)
//  3. feedback in .siren/inbox (delivered by phonewave from .gate/outbox)
//
// NOTE: amadeus checks .gate/archive (not .gate/inbox) because amadeus
// consumes reports from inbox during check, archiving them in the process.
func (o *Observer) WaitForClosedLoop(timeout time.Duration) {
	o.t.Helper()
	stepTimeout := timeout / 3
	if stepTimeout < 10*time.Second {
		stepTimeout = 10 * time.Second
	}

	o.ws.WaitForDMail(o.t, ".expedition", "inbox", stepTimeout)
	o.ws.WaitForDMail(o.t, ".gate", "archive", stepTimeout)
	o.ws.WaitForDMail(o.t, ".siren", "inbox", stepTimeout)
}

// --- Prompt assertion helpers (009, 001, 013) ---

// AssertPromptCount verifies that fake-claude was called exactly wantCount times
// by counting log files in PromptLogDir. A count of 0 suggests the real API
// may have been called instead of fake-claude.
func (o *Observer) AssertPromptCount(wantCount int) {
	o.t.Helper()
	entries, err := os.ReadDir(o.ws.PromptLogDir())
	if err != nil {
		o.t.Fatalf("read prompt-log dir %s: %v", o.ws.PromptLogDir(), err)
	}
	got := len(entries)
	if got != wantCount {
		if got == 0 {
			o.t.Errorf("prompt count: got 0, want %d — fake-claude may not have been invoked (real API called?)", wantCount)
		} else {
			o.t.Errorf("prompt count: got %d, want %d", got, wantCount)
		}
	}
}

// AssertPromptContains verifies that at least one logged prompt contains
// all of the specified substrings. This ensures the LLM Judge received
// the expected input (e.g., ADR file references, DoD content).
func (o *Observer) AssertPromptContains(wantSubstrings []string) {
	o.t.Helper()
	entries, err := os.ReadDir(o.ws.PromptLogDir())
	if err != nil {
		o.t.Fatalf("read prompt-log dir: %v", err)
	}
	if len(entries) == 0 {
		o.t.Fatal("no prompt logs found — cannot verify prompt content")
	}

	for _, substr := range wantSubstrings {
		found := false
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(o.ws.PromptLogDir(), entry.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(string(data), substr) {
				found = true
				break
			}
		}
		if !found {
			o.t.Errorf("prompt content: no logged prompt contains %q", substr)
		}
	}
}

// AssertPromptQuality is a composite guard that verifies both call count
// and prompt content in a single call. Combines AssertPromptCount and
// AssertPromptContains for convenience.
func (o *Observer) AssertPromptQuality(wantCount int, wantContains []string) {
	o.t.Helper()
	o.AssertPromptCount(wantCount)
	if len(wantContains) > 0 {
		o.AssertPromptContains(wantContains)
	}
}

// --- D-Mail field assertion helpers (004) ---

// AssertDMailSeverity verifies that a D-Mail file has the expected severity
// field in its frontmatter. This tests the CalcDivergence → DetermineSeverity
// → D-Mail frontmatter pipeline end-to-end.
func (o *Observer) AssertDMailSeverity(path, expectedSeverity string) {
	o.t.Helper()
	fm, _ := o.ws.ReadDMail(o.t, path)
	severity, ok := fm["severity"].(string)
	if !ok {
		o.t.Errorf("D-Mail %s: missing severity field in frontmatter", path)
		return
	}
	if severity != expectedSeverity {
		o.t.Errorf("D-Mail %s: got severity %q, want %q", path, severity, expectedSeverity)
	}
}

// AssertDMailAction verifies that a D-Mail file has the expected action
// field in its frontmatter (e.g., "escalate", "monitor", "none").
func (o *Observer) AssertDMailAction(path, expectedAction string) {
	o.t.Helper()
	fm, _ := o.ws.ReadDMail(o.t, path)
	action, ok := fm["action"].(string)
	if !ok {
		o.t.Errorf("D-Mail %s: missing action field in frontmatter", path)
		return
	}
	if action != expectedAction {
		o.t.Errorf("D-Mail %s: got action %q, want %q", path, action, expectedAction)
	}
}

// --- Diff check assertion helpers (proposal 025) ---

// AssertDMailCount verifies the number of D-Mail .md files in a mailbox directory.
func (o *Observer) AssertDMailCount(toolDir, mailbox string, wantCount int) {
	o.t.Helper()
	dir := filepath.Join(o.ws.RepoPath, toolDir, mailbox)
	files := o.ws.ListFiles(o.t, dir)
	var mdCount int
	for _, f := range files {
		if strings.HasSuffix(f, ".md") {
			mdCount++
		}
	}
	if mdCount != wantCount {
		o.t.Errorf("%s/%s: got %d D-Mails, want %d", toolDir, mailbox, mdCount, wantCount)
	}
}
