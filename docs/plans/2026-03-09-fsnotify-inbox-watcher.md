# amadeus fsnotify Inbox Watcher Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace amadeus's polling-based ScanInbox with fsnotify-based MonitorInbox (channel pattern), unifying D-Mail reception with sightjack and paintress.

**Architecture:** Add `MonitorInbox` function (session layer, not a Store method) that returns `<-chan domain.DMail`. It does a two-phase startup: initial drain of existing inbox files, then fsnotify watch for new arrivals. Each file is archived + removed atomically (dedup by archive presence). The `Run` daemon loop replaces `default: ScanInbox + sleep` with `case mail := <-ch:` in its select. The `ScanInbox` method on ProjectionStore is preserved for `RunCheck` (one-shot) but its comment is updated. The `PollInterval` field and `DefaultPollInterval` constant are removed.

**Tech Stack:** Go 1.26, github.com/fsnotify/fsnotify v1.9.0, existing OTel tracing

---

### Task 1: Add fsnotify dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add fsnotify**

```bash
cd /Users/nino/tap/amadeus && go get github.com/fsnotify/fsnotify@v1.9.0
```

**Step 2: Tidy**

```bash
cd /Users/nino/tap/amadeus && go mod tidy
```

**Step 3: Verify**

```bash
cd /Users/nino/tap/amadeus && grep fsnotify go.mod
```

Expected: `github.com/fsnotify/fsnotify v1.9.0`

**Step 4: Commit**

```bash
cd /Users/nino/tap/amadeus && git add go.mod go.sum && git commit -m "deps: add fsnotify v1.9.0 for inbox watcher"
```

---

### Task 2: Add receiveDMail helper (archive + remove single file)

**Files:**
- Modify: `internal/session/dmail_io.go`
- Create: `internal/session/dmail_io_test.go`

**Step 1: Write failing test**

Create `internal/session/dmail_io_test.go`:

```go
package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestReceiveDMail_ArchivesAndRemoves(t *testing.T) {
	// given: inbox file exists, archive dir exists
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	archiveDir := filepath.Join(root, "archive")
	os.MkdirAll(inboxDir, 0o755)
	os.MkdirAll(archiveDir, 0o755)

	content := []byte("---\nschema_version: \"1\"\nname: report-001\nkind: report\ndescription: test\n---\nbody\n")
	os.WriteFile(filepath.Join(inboxDir, "report-001.md"), content, 0o644)

	// when
	dmail, err := session.ReceiveDMailFromInbox(root, "report-001.md")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dmail.Name != "report-001" {
		t.Errorf("expected name report-001, got %s", dmail.Name)
	}
	// archived
	if _, statErr := os.Stat(filepath.Join(archiveDir, "report-001.md")); statErr != nil {
		t.Error("expected archive file to exist")
	}
	// removed from inbox
	if _, statErr := os.Stat(filepath.Join(inboxDir, "report-001.md")); !os.IsNotExist(statErr) {
		t.Error("expected inbox file to be removed")
	}
}

func TestReceiveDMail_SkipsIfAlreadyArchived(t *testing.T) {
	// given: same file exists in both inbox and archive
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	archiveDir := filepath.Join(root, "archive")
	os.MkdirAll(inboxDir, 0o755)
	os.MkdirAll(archiveDir, 0o755)

	content := []byte("---\nschema_version: \"1\"\nname: dup-001\nkind: report\ndescription: dup\n---\n")
	os.WriteFile(filepath.Join(inboxDir, "dup-001.md"), content, 0o644)
	os.WriteFile(filepath.Join(archiveDir, "dup-001.md"), content, 0o644)

	// when
	dmail, err := session.ReceiveDMailFromInbox(root, "dup-001.md")

	// then: nil dmail (dedup), no error, inbox file removed
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dmail != nil {
		t.Error("expected nil dmail for already-archived file")
	}
	if _, statErr := os.Stat(filepath.Join(inboxDir, "dup-001.md")); !os.IsNotExist(statErr) {
		t.Error("expected inbox file to be removed even for dup")
	}
}

func TestReceiveDMail_ParseError(t *testing.T) {
	// given: inbox file with invalid content
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	archiveDir := filepath.Join(root, "archive")
	os.MkdirAll(inboxDir, 0o755)
	os.MkdirAll(archiveDir, 0o755)

	os.WriteFile(filepath.Join(inboxDir, "bad.md"), []byte("not a dmail"), 0o644)

	// when
	dmail, err := session.ReceiveDMailFromInbox(root, "bad.md")

	// then: error returned, file left in inbox for retry
	if err == nil {
		t.Fatal("expected error for unparseable file")
	}
	if dmail != nil {
		t.Error("expected nil dmail on error")
	}
	// file should remain in inbox (partial write resilience)
	if _, statErr := os.Stat(filepath.Join(inboxDir, "bad.md")); os.IsNotExist(statErr) {
		t.Error("expected inbox file to remain for retry")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/tap/amadeus && go test ./internal/session/ -run TestReceiveDMail -v
```

Expected: FAIL (function not defined)

**Step 3: Implement ReceiveDMailFromInbox**

Add to `internal/session/dmail_io.go`:

```go
// ReceiveDMailFromInbox reads a single D-Mail file from inbox/, applies
// archive-based dedup, archives the file, and removes it from inbox/.
// Returns (nil, nil) if the file is already archived (dedup).
// Returns (nil, err) if the file cannot be read or parsed (left in inbox for retry).
func ReceiveDMailFromInbox(root, filename string) (*domain.DMail, error) {
	inboxPath := filepath.Join(root, "inbox", filename)
	archivePath := filepath.Join(root, "archive", filename)

	// Dedup: skip if already archived
	if _, err := os.Stat(archivePath); err == nil {
		// Remove from inbox (best-effort)
		os.Remove(inboxPath)
		return nil, nil
	}

	data, err := os.ReadFile(inboxPath)
	if err != nil {
		return nil, fmt.Errorf("read inbox %s: %w", filename, err)
	}

	dmail, err := domain.ParseDMail(data)
	if err != nil {
		return nil, fmt.Errorf("parse inbox %s: %w", filename, err)
	}

	// Archive (atomic: write then remove)
	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return nil, fmt.Errorf("archive %s: %w", filename, err)
	}
	if err := os.Remove(inboxPath); err != nil {
		// Already archived, tolerate remove failure
		return &dmail, nil
	}

	return &dmail, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/nino/tap/amadeus && go test ./internal/session/ -run TestReceiveDMail -v
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/nino/tap/amadeus && git add internal/session/dmail_io.go internal/session/dmail_io_test.go && git commit -m "feat: add ReceiveDMailFromInbox with archive-based dedup"
```

---

### Task 3: Add MonitorInbox function (fsnotify watcher)

**Files:**
- Create: `internal/session/inbox_watcher.go`
- Create: `internal/session/inbox_watcher_test.go`

**Step 1: Write failing test**

Create `internal/session/inbox_watcher_test.go`:

```go
package session_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestMonitorInbox_InitialDrain(t *testing.T) {
	// given: inbox with pre-existing D-Mail
	root := t.TempDir()
	for _, sub := range []string{"inbox", "archive"} {
		os.MkdirAll(filepath.Join(root, sub), 0o755)
	}
	content := []byte("---\nschema_version: \"1\"\nname: report-001\nkind: report\ndescription: initial\n---\nbody\n")
	os.WriteFile(filepath.Join(root, "inbox", "report-001.md"), content, 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when
	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// then: initial file delivered on channel
	select {
	case dmail := <-ch:
		if dmail.Name != "report-001" {
			t.Errorf("expected name report-001, got %s", dmail.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial drain")
	}
}

func TestMonitorInbox_WatchesNewFiles(t *testing.T) {
	// given: empty inbox
	root := t.TempDir()
	for _, sub := range []string{"inbox", "archive"} {
		os.MkdirAll(filepath.Join(root, sub), 0o755)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Drain any initial (should be none)
	select {
	case <-ch:
		t.Fatal("unexpected initial dmail")
	case <-time.After(50 * time.Millisecond):
	}

	// when: write a new file to inbox
	content := []byte("---\nschema_version: \"1\"\nname: feedback-001\nkind: report\ndescription: new\n---\nbody\n")
	os.WriteFile(filepath.Join(root, "inbox", "feedback-001.md"), content, 0o644)

	// then: new file delivered on channel
	select {
	case dmail := <-ch:
		if dmail.Name != "feedback-001" {
			t.Errorf("expected name feedback-001, got %s", dmail.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for fsnotify event")
	}

	// Verify archived
	if _, statErr := os.Stat(filepath.Join(root, "archive", "feedback-001.md")); statErr != nil {
		t.Error("expected archive file to exist")
	}
}

func TestMonitorInbox_ContextCancellation(t *testing.T) {
	// given
	root := t.TempDir()
	for _, sub := range []string{"inbox", "archive"} {
		os.MkdirAll(filepath.Join(root, sub), 0o755)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// when: cancel context
	cancel()

	// then: channel closes
	select {
	case _, ok := <-ch:
		if ok {
			// Got a value, that's fine, keep draining
		}
		// Channel will eventually close
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/tap/amadeus && go test ./internal/session/ -run TestMonitorInbox -v
```

Expected: FAIL (function not defined)

**Step 3: Implement MonitorInbox**

Create `internal/session/inbox_watcher.go`:

```go
package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/hironow/amadeus/internal/domain"
)

// MonitorInbox starts monitoring the inbox directory for D-Mail files.
// It first drains existing files (initial scan), then watches for new files via fsnotify.
// Each D-Mail is archived + removed from inbox. All valid D-Mails are sent to the
// returned channel. Archive-based dedup prevents double delivery.
// The channel is closed when the context is cancelled.
//
// This follows the same two-phase pattern as sightjack's MonitorInbox:
// Phase 1 (synchronous): watcher.Add first, then drain existing files.
// Phase 2 (async goroutine): select on fsnotify events + ctx.Done.
func MonitorInbox(ctx context.Context, root string, logger domain.Logger) (<-chan domain.DMail, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("inbox monitor: create watcher: %w", err)
	}

	inboxDir := filepath.Join(root, "inbox")
	if err := watcher.Add(inboxDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("inbox monitor: watch inbox: %w", err)
	}

	// Phase 1: Initial drain (synchronous).
	// watcher.Add is called first so files created during drain are caught by fsnotify.
	var initial []domain.DMail
	entries, listErr := os.ReadDir(inboxDir)
	if listErr == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			dmail, recvErr := ReceiveDMailFromInbox(root, e.Name())
			if recvErr != nil {
				logger.Warn("inbox drain: %v", recvErr)
				continue
			}
			if dmail != nil {
				initial = append(initial, *dmail)
			}
		}
	}

	ch := make(chan domain.DMail, len(initial)+8)
	for _, mail := range initial {
		ch <- mail
	}

	// Phase 2: Watch for new files (async).
	go func() {
		defer close(ch)
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) && strings.HasSuffix(event.Name, ".md") {
					filename := filepath.Base(event.Name)
					dmail, recvErr := ReceiveDMailFromInbox(root, filename)
					if recvErr != nil {
						// Parse failed — leave in inbox for retry on next Write event
						continue
					}
					if dmail != nil {
						select {
						case ch <- *dmail:
						case <-ctx.Done():
							return
						}
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return ch, nil
}
```

Note: Add `"fmt"` to the import if not already present.

**Step 4: Run test to verify it passes**

```bash
cd /Users/nino/tap/amadeus && go test ./internal/session/ -run TestMonitorInbox -v
```

Expected: PASS

**Step 5: Run all tests**

```bash
cd /Users/nino/tap/amadeus && go test ./internal/session/ -v -count=1
```

Expected: All PASS

**Step 6: Commit**

```bash
cd /Users/nino/tap/amadeus && git add internal/session/inbox_watcher.go internal/session/inbox_watcher_test.go && git commit -m "feat: add MonitorInbox with fsnotify for real-time D-Mail reception"
```

---

### Task 4: Refactor Run loop to use MonitorInbox channel

**Files:**
- Modify: `internal/session/run.go`
- Modify: `internal/session/run_test.go`

**Step 1: Update Run to use MonitorInbox**

Replace the polling loop in `internal/session/run.go`. The key change:
- Start `MonitorInbox` at the beginning of Run
- Replace `default: ScanInbox + sleep` with `case dmail := <-ch:`
- Process D-Mails one at a time from channel (accumulate report flag, batch-process)

New `Run` method:

```go
func (a *Amadeus) Run(ctx context.Context, opts domain.RunOptions, emitter port.CheckEventEmitter, state port.CheckStateProvider) error {
	if emitter != nil {
		a.Emitter = emitter
	}
	if state != nil {
		a.State = state
	}

	ctx, span := platform.Tracer.Start(ctx, "amadeus.run")
	defer span.End()

	// Auto-rebuild projections if needed
	if err := a.autoRebuildIfNeeded(opts.Quiet); err != nil {
		return fmt.Errorf("auto-rebuild: %w", err)
	}

	// Determine integration branch
	integrationBranch, err := a.Git.CurrentBranch()
	if err != nil {
		integrationBranch = "main"
		a.Logger.Warn("could not detect current branch, using %q", integrationBranch)
	}

	// Emit run.started
	now := time.Now().UTC()
	if err := a.Emitter.EmitRunStarted(domain.RunStartedData{
		IntegrationBranch: integrationBranch,
		BaseBranch:        opts.BaseBranch,
	}, now); err != nil {
		return fmt.Errorf("emit run started: %w", err)
	}

	if !opts.Quiet {
		a.Logger.Info("amadeus run: integration point = %s", integrationBranch)
		if opts.BaseBranch != "" {
			a.Logger.Info("amadeus run: post-merge checks enabled (base = %s)", opts.BaseBranch)
		}
	}

	// Start inbox monitor (fsnotify)
	inboxCh, monErr := MonitorInbox(ctx, a.stateDir(), a.Logger)
	if monErr != nil {
		return fmt.Errorf("inbox monitor: %w", monErr)
	}

	// Main loop: event-driven via channel
	for {
		select {
		case <-ctx.Done():
			stopNow := time.Now().UTC()
			_ = a.Emitter.EmitRunStopped(domain.RunStoppedData{Reason: "signal"}, stopNow)
			if !opts.Quiet {
				a.Logger.Info("amadeus run: stopped (signal)")
			}
			return nil

		case dmail, ok := <-inboxCh:
			if !ok {
				// Channel closed (watcher error or context)
				return nil
			}

			// Emit inbox consumed event
			inboxNow := time.Now().UTC()
			domain.LogBanner(a.Logger, domain.BannerRecv, string(dmail.Kind), dmail.Name, dmail.Description)
			if err := a.Emitter.EmitInboxConsumed(domain.InboxConsumedData{
				Name:   dmail.Name,
				Kind:   dmail.Kind,
				Source: dmail.Name + ".md",
			}, inboxNow); err != nil {
				a.Logger.Warn("emit inbox consumed: %v", err)
			}

			if !opts.Quiet {
				a.Logger.Info("consumed D-Mail from inbox: %s", dmail.Name)
			}

			// Trigger pre-merge pipeline on report D-Mails
			if dmail.Kind == domain.KindReport {
				dmails, prErr := a.runPreMergePipeline(ctx, integrationBranch)
				if prErr != nil {
					a.Logger.Warn("pre-merge pipeline error: %v", prErr)
				} else if len(dmails) > 0 && !opts.Quiet {
					a.Logger.OK("generated %d implementation-feedback D-Mail(s)", len(dmails))
				}

				// Trigger post-merge pipeline if BaseBranch is set
				if opts.BaseBranch != "" {
					previous, loadErr := a.Store.LoadLatest()
					if loadErr != nil {
						a.Logger.Warn("load previous state: %v", loadErr)
					} else {
						a.State.Restore(previous)
						checkOpts := domain.CheckOptions{
							Full:  opts.Full,
							Quiet: opts.Quiet,
							JSON:  opts.JSON,
						}
						if checkErr := a.runPostMergeCheck(ctx, checkOpts); checkErr != nil {
							if _, ok := checkErr.(*domain.DriftError); !ok {
								a.Logger.Warn("post-merge check error: %v", checkErr)
							}
						}
					}
				}
			}
		}
	}
}
```

Also need a `stateDir()` helper. Check if `a.Store` exposes Root, or derive from Store. Looking at `ProjectionStore`, its `Root` field is the state dir. But `a.Store` is a `port.StateReader` interface. We need the root path.

Approach: Add a `Root` field to `Amadeus` struct (or use existing `RepoDir` + `domain.StateDir`). The simplest: `filepath.Join(a.RepoDir, domain.StateDir)`.

**Step 2: Update existing run_test.go**

The existing tests use `runStore` mock with `ScanInbox`. We need to update them to work with the new channel-based approach. Since `MonitorInbox` operates on the filesystem, the tests need to switch to real filesystem-based tests or we need to make the inbox source injectable.

**Design decision:** Make the inbox channel injectable. Add an `InboxCh` field to `Amadeus`:

```go
// InboxCh overrides MonitorInbox when set (for testing).
// When nil, Run starts MonitorInbox automatically.
InboxCh <-chan domain.DMail
```

This keeps testability without requiring filesystem setup in unit tests.

**Step 3: Update tests to use InboxCh**

```go
// In run_test.go, create a helper to feed D-Mails via channel:
func feedInbox(dmails ...domain.DMail) <-chan domain.DMail {
	ch := make(chan domain.DMail, len(dmails))
	for _, d := range dmails {
		ch <- d
	}
	return ch
}
```

Update `TestRun_gracefulShutdown`: set `InboxCh` to empty closed channel.
Update `TestRun_inboxTriggerPreMerge`: feed report D-Mail via channel.
Update `TestRun_noPRReaderSkipsPreMerge`: feed report D-Mail via channel.

**Step 4: Run tests**

```bash
cd /Users/nino/tap/amadeus && go test ./internal/session/ -run TestRun -v
```

Expected: All PASS

**Step 5: Run full test suite**

```bash
cd /Users/nino/tap/amadeus && go test ./... -count=1
```

Expected: All PASS

**Step 6: Commit**

```bash
cd /Users/nino/tap/amadeus && git add internal/session/run.go internal/session/run_test.go internal/session/amadeus.go && git commit -m "feat: refactor Run loop from polling to fsnotify channel"
```

---

### Task 5: Remove PollInterval from RunOptions

**Files:**
- Modify: `internal/domain/run_options.go`
- Modify: `internal/cmd/run.go` (remove PollInterval from opts)
- Modify: any other references

**Step 1: Remove PollInterval and DefaultPollInterval**

Update `internal/domain/run_options.go`:

```go
package domain

// RunOptions configures the amadeus run daemon loop.
type RunOptions struct {
	CheckOptions              // embedded check options
	BaseBranch   string       // upstream branch for post-merge checks (empty = none)
}
```

**Step 2: Update cmd/run.go**

Remove `PollInterval: domain.DefaultPollInterval` from the `usecase.Run` call.

**Step 3: Update run_test.go**

Remove `PollInterval` from all `domain.RunOptions{}` in tests.

**Step 4: Find and fix any remaining references**

```bash
cd /Users/nino/tap/amadeus && grep -rn "PollInterval\|DefaultPollInterval" --include="*.go"
```

Fix all references.

**Step 5: Run full test suite**

```bash
cd /Users/nino/tap/amadeus && go test ./... -count=1
```

Expected: All PASS

**Step 6: Commit**

```bash
cd /Users/nino/tap/amadeus && git add -u && git commit -m "refactor: remove PollInterval (replaced by fsnotify)"
```

---

### Task 6: Update ScanInbox comment and StateReader doc

**Files:**
- Modify: `internal/session/dmail_io.go` (update ScanInbox comment)
- Modify: `internal/usecase/port/port.go` (update StateReader doc)

**Step 1: Update ScanInbox comment**

Replace the NOTE block in `dmail_io.go:119-128`:

```go
// ScanInbox reads all .md files from inbox/, parses them with ParseDMail,
// copies to archive/ (skip if already exists), and removes from inbox/.
// Returns the parsed D-Mails sorted by name.
//
// Used by RunCheck (one-shot check command). The Run daemon loop uses
// MonitorInbox (fsnotify-based) instead for real-time D-Mail reception.
```

**Step 2: Run tests**

```bash
cd /Users/nino/tap/amadeus && go test ./... -count=1
```

**Step 3: Commit**

```bash
cd /Users/nino/tap/amadeus && git add internal/session/dmail_io.go internal/usecase/port/port.go && git commit -m "docs: update ScanInbox and StateReader comments for fsnotify migration"
```

---

### Task 7: Integration test — fsnotify inbox watcher with Run loop

**Files:**
- Create: `tests/integration/inbox_watcher_test.go`

**Step 1: Write integration test**

```go
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestMonitorInbox_Integration_MultipleFiles(t *testing.T) {
	// given: inbox with 2 pre-existing files, then 1 new file arrives
	root := t.TempDir()
	for _, sub := range []string{"inbox", "archive"} {
		os.MkdirAll(filepath.Join(root, sub), 0o755)
	}

	makeDMail := func(name, kind string) []byte {
		return []byte("---\nschema_version: \"1\"\nname: " + name + "\nkind: " + kind + "\ndescription: test\n---\nbody\n")
	}

	// Pre-existing
	os.WriteFile(filepath.Join(root, "inbox", "report-001.md"), makeDMail("report-001", "report"), 0o644)
	os.WriteFile(filepath.Join(root, "inbox", "report-002.md"), makeDMail("report-002", "report"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Collect initial drain
	var received []string
	for i := 0; i < 2; i++ {
		select {
		case dm := <-ch:
			received = append(received, dm.Name)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for initial file %d", i+1)
		}
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 initial dmails, got %d", len(received))
	}

	// Write new file after watcher started
	time.Sleep(100 * time.Millisecond) // let watcher settle
	os.WriteFile(filepath.Join(root, "inbox", "feedback-001.md"), makeDMail("feedback-001", "report"), 0o644)

	select {
	case dm := <-ch:
		if dm.Name != "feedback-001" {
			t.Errorf("expected feedback-001, got %s", dm.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for new file via fsnotify")
	}

	// Verify all archived
	for _, name := range []string{"report-001.md", "report-002.md", "feedback-001.md"} {
		if _, statErr := os.Stat(filepath.Join(root, "archive", name)); statErr != nil {
			t.Errorf("expected %s in archive", name)
		}
	}

	// Verify inbox is empty
	entries, _ := os.ReadDir(filepath.Join(root, "inbox"))
	if len(entries) != 0 {
		t.Errorf("expected empty inbox, got %d files", len(entries))
	}
}

func TestMonitorInbox_Integration_DedupOnRestart(t *testing.T) {
	// given: file already in archive, same file appears in inbox
	root := t.TempDir()
	for _, sub := range []string{"inbox", "archive"} {
		os.MkdirAll(filepath.Join(root, sub), 0o755)
	}

	content := []byte("---\nschema_version: \"1\"\nname: dup-001\nkind: report\ndescription: dup\n---\n")
	os.WriteFile(filepath.Join(root, "inbox", "dup-001.md"), content, 0o644)
	os.WriteFile(filepath.Join(root, "archive", "dup-001.md"), content, 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := session.MonitorInbox(ctx, root, &domain.NopLogger{})
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// No dmail should be delivered (dedup)
	select {
	case dm := <-ch:
		t.Errorf("unexpected dmail delivered: %s", dm.Name)
	case <-time.After(500 * time.Millisecond):
		// OK: no delivery
	}

	// Inbox file should be cleaned up
	if _, statErr := os.Stat(filepath.Join(root, "inbox", "dup-001.md")); !os.IsNotExist(statErr) {
		t.Error("expected dedup'd inbox file to be removed")
	}
}
```

**Step 2: Run integration tests**

```bash
cd /Users/nino/tap/amadeus && go test ./tests/integration/ -run TestMonitorInbox -v -count=1
```

Expected: All PASS

**Step 3: Commit**

```bash
cd /Users/nino/tap/amadeus && git add tests/integration/inbox_watcher_test.go && git commit -m "test: add integration tests for fsnotify inbox watcher"
```

---

### Task 8: ADR for fsnotify migration

**Files:**
- Create: `docs/adr/NNNN-fsnotify-inbox-watcher.md` (check next available number)

**Step 1: Determine next ADR number**

```bash
ls /Users/nino/tap/amadeus/docs/adr/ | tail -3
```

**Step 2: Write ADR**

Standard format: Context (polling was a remnant of check CLI, run is now a daemon), Decision (adopt fsnotify matching sightjack/paintress pattern), Consequences.

**Step 3: Commit**

```bash
cd /Users/nino/tap/amadeus && git add docs/adr/ && git commit -m "docs: add ADR for fsnotify inbox watcher migration"
```
