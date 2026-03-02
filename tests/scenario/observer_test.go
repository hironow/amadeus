//go:build scenario

package scenario_test

import (
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

// WaitForClosedLoop waits for a complete closed loop (specification -> report -> feedback).
// It polls all 3 delivery points:
//  1. specification in .expedition/inbox
//  2. report in .gate/inbox
//  3. feedback in .siren/inbox AND .expedition/inbox
func (o *Observer) WaitForClosedLoop(timeout time.Duration) {
	o.t.Helper()
	stepTimeout := timeout / 3
	if stepTimeout < 10*time.Second {
		stepTimeout = 10 * time.Second
	}

	o.ws.WaitForDMail(o.t, ".expedition", "inbox", stepTimeout)
	o.ws.WaitForDMail(o.t, ".gate", "inbox", stepTimeout)
	o.ws.WaitForDMail(o.t, ".siren", "inbox", stepTimeout)
}
