package session_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func testOutboxStore(t *testing.T, root string) *session.SQLiteOutboxStore {
	t.Helper()
	store, err := session.NewOutboxStoreForDir(root)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func ensureGateDirs(t *testing.T, repoRoot string) {
	t.Helper()
	for _, sub := range []string{".run", "archive", "outbox"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, ".gate", sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
}

func TestSQLiteOutboxStore_PragmaSynchronousNormal(t *testing.T) {
	// given
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	// when: query PRAGMA on the store's own connection
	var synchronous string
	if err := store.DBForTest().QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("query PRAGMA synchronous: %v", err)
	}

	// then: synchronous = 1 (NORMAL)
	if synchronous != "1" {
		t.Errorf("PRAGMA synchronous: got %q, want %q (NORMAL)", synchronous, "1")
	}
}

func TestSQLiteOutboxStore_StageAndFlush(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	err := store.Stage(ctx, "test-mail.md", []byte("hello"))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	archivePath := filepath.Join(root, ".gate", "archive", "test-mail.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("archive content: got %q, want %q", string(data), "hello")
	}

	outboxPath := filepath.Join(root, ".gate", "outbox", "test-mail.md")
	data, err = os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("outbox content: got %q, want %q", string(data), "hello")
	}
}

func TestSQLiteOutboxStore_StageUpsert_LatestDataWins(t *testing.T) {
	// given: same name staged twice before flush
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	if err := store.Stage(ctx, "dup.md", []byte("first")); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}
	if err := store.Stage(ctx, "dup.md", []byte("second")); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}

	// when
	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	// then: latest data wins (upsert semantics)
	outboxPath := filepath.Join(root, ".gate", "outbox", "dup.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("content: got %q, want %q", string(data), "second")
	}
}

func TestSQLiteOutboxStore_FlushEmpty(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 0 {
		t.Errorf("flushed count: got %d, want 0", n)
	}
}

func TestSQLiteOutboxStore_RestageAfterFlush_EnablesRedelivery(t *testing.T) {
	// given: a D-Mail that has been staged and flushed
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	if err := store.Stage(ctx, "conflict.md", []byte("first-attempt")); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}
	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush 1: %v", err)
	}
	if n != 1 {
		t.Errorf("first flush count: got %d, want 1", n)
	}

	// when: re-stage the same name with updated data (e.g. conflict still exists)
	if err := store.Stage(ctx, "conflict.md", []byte("second-attempt")); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}

	// then: second flush should deliver the updated D-Mail
	n, err = store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush 2: %v", err)
	}
	if n != 1 {
		t.Errorf("second flush count: got %d, want 1 (re-delivery)", n)
	}

	// then: outbox file should contain the updated data
	outboxPath := filepath.Join(root, ".gate", "outbox", "conflict.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "second-attempt" {
		t.Errorf("content: got %q, want %q", string(data), "second-attempt")
	}
}

func TestSQLiteOutboxStore_RestageBeforeFlush_UpdatesData(t *testing.T) {
	// given: a D-Mail staged but not yet flushed
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	if err := store.Stage(ctx, "update.md", []byte("original")); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}

	// when: re-stage with updated data before flushing
	if err := store.Stage(ctx, "update.md", []byte("updated")); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}

	// then: flush should deliver the latest data
	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	outboxPath := filepath.Join(root, ".gate", "outbox", "update.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "updated" {
		t.Errorf("content: got %q, want %q", string(data), "updated")
	}
}

func TestSQLiteOutboxStore_FlushOnlyUnflushed(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	store.Stage(ctx, "first.md", []byte("one"))
	store.Flush(ctx)

	store.Stage(ctx, "second.md", []byte("two"))

	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	for _, name := range []string{"first.md", "second.md"} {
		outboxPath := filepath.Join(root, ".gate", "outbox", name)
		if _, err := os.Stat(outboxPath); err != nil {
			t.Errorf("outbox %s missing: %v", name, err)
		}
	}
}

func TestSQLiteOutboxStore_MultipleStageThenFlush(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	store.Stage(ctx, "a.md", []byte("aaa"))
	store.Stage(ctx, "b.md", []byte("bbb"))
	store.Stage(ctx, "c.md", []byte("ccc"))

	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 3 {
		t.Errorf("flushed count: got %d, want 3", n)
	}

	for _, name := range []string{"a.md", "b.md", "c.md"} {
		for _, sub := range []string{"archive", "outbox"} {
			p := filepath.Join(root, ".gate", sub, name)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("%s/%s missing: %v", sub, name, err)
			}
		}
	}
}

func TestSQLiteOutboxStore_ConcurrentStageAndFlush(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)

	dbPath := filepath.Join(root, ".gate", ".run", "outbox.db")
	archiveDir := filepath.Join(root, ".gate", "archive")
	outboxDir := filepath.Join(root, ".gate", "outbox")

	storeA, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	defer storeB.Close()

	const itemsPerStore = 10
	ctx := context.Background()

	var wg sync.WaitGroup
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			name := fmt.Sprintf("a-%03d.md", i)
			if err := storeA.Stage(ctx, name, []byte("from-A-"+name)); err != nil {
				errA <- err
				return
			}
			if _, err := storeA.Flush(ctx); err != nil {
				errA <- err
				return
			}
		}
		errA <- nil
	}()
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			name := fmt.Sprintf("b-%03d.md", i)
			if err := storeB.Stage(ctx, name, []byte("from-B-"+name)); err != nil {
				errB <- err
				return
			}
			if _, err := storeB.Flush(ctx); err != nil {
				errB <- err
				return
			}
		}
		errB <- nil
	}()
	wg.Wait()

	if e := <-errA; e != nil {
		t.Fatalf("store A error: %v", e)
	}
	if e := <-errB; e != nil {
		t.Fatalf("store B error: %v", e)
	}

	for _, prefix := range []string{"a", "b"} {
		for i := range itemsPerStore {
			name := fmt.Sprintf("%s-%03d.md", prefix, i)
			for _, sub := range []string{"archive", "outbox"} {
				p := filepath.Join(root, ".gate", sub, name)
				data, readErr := os.ReadFile(p)
				if readErr != nil {
					t.Errorf(".gate/%s/%s missing: %v", sub, name, readErr)
					continue
				}
				expected := fmt.Sprintf("from-%s-%s", strings.ToUpper(prefix), name)
				if string(data) != expected {
					t.Errorf("%s/%s content: got %q, want %q", sub, name, string(data), expected)
				}
			}
		}
	}
}

func TestSQLiteOutboxStore_FilePermission(t *testing.T) {
	if os.Getenv("CI") != "" && strings.Contains(strings.ToLower(os.Getenv("RUNNER_OS")), "windows") {
		t.Skip("NTFS does not support Unix file permissions")
	}

	// given
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	_ = store

	// when
	dbPath := filepath.Join(root, ".gate", ".run", "outbox.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}

	// then: permission should be 0o600 (owner read/write only)
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("db permission: got %o, want %o", perm, 0o600)
	}
}

func TestSQLiteOutboxStore_RetryCount_DeadLetterAfterMaxRetries(t *testing.T) {
	// given
	root := t.TempDir()
	ensureGateDirs(t, root)

	dbPath := filepath.Join(root, ".gate", ".run", "outbox.db")
	archiveDir := filepath.Join(root, ".gate", "archive")
	outboxDir := filepath.Join(root, ".gate", "outbox")

	store, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	store.Stage(ctx, "fail.md", []byte("data"))

	// Make archive dir read-only so atomicWrite fails
	os.Chmod(archiveDir, 0o444)
	defer os.Chmod(archiveDir, 0o755)

	// when: flush 3 times (each fails, incrementing retry_count to 3)
	for i := range 3 {
		n, _ := store.Flush(ctx)
		if n != 0 {
			t.Errorf("flush %d: expected 0 flushed, got %d", i+1, n)
		}
	}

	// Restore permissions
	os.Chmod(archiveDir, 0o755)

	// when: flush again — item should be dead-letter
	n, err := store.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 flushed (dead-letter), got %d", n)
	}
}

func TestSQLiteOutboxStore_RetryCount_SuccessBeforeMaxRetries(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)

	dbPath := filepath.Join(root, ".gate", ".run", "outbox.db")
	archiveDir := filepath.Join(root, ".gate", "archive")
	outboxDir := filepath.Join(root, ".gate", "outbox")

	store, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	store.Stage(ctx, "retry.md", []byte("retry-data"))

	// First flush fails
	os.Chmod(archiveDir, 0o444)
	n, _ := store.Flush(ctx)
	if n != 0 {
		t.Errorf("first flush: expected 0, got %d", n)
	}

	// Restore — second flush succeeds
	os.Chmod(archiveDir, 0o755)
	n, err = store.Flush(ctx)
	if err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("second flush: expected 1, got %d", n)
	}

	data, err := os.ReadFile(filepath.Join(archiveDir, "retry.md"))
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(data) != "retry-data" {
		t.Errorf("content: got %q, want %q", string(data), "retry-data")
	}
}

func TestSQLiteOutboxStore_ConcurrentFlushSameItem(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)

	dbPath := filepath.Join(root, ".gate", ".run", "outbox.db")
	archiveDir := filepath.Join(root, ".gate", "archive")
	outboxDir := filepath.Join(root, ".gate", "outbox")

	storeSetup, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create setup store: %v", err)
	}
	ctx := context.Background()
	if err := storeSetup.Stage(ctx, "shared.md", []byte("shared-content")); err != nil {
		t.Fatalf("stage: %v", err)
	}
	storeSetup.Close()

	storeA, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	defer storeB.Close()

	var wg sync.WaitGroup
	var nA, nB int
	var eA, eB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		nA, eA = storeA.Flush(ctx)
	}()
	go func() {
		defer wg.Done()
		nB, eB = storeB.Flush(ctx)
	}()
	wg.Wait()

	if eA != nil {
		t.Fatalf("store A flush error: %v", eA)
	}
	if eB != nil {
		t.Fatalf("store B flush error: %v", eB)
	}

	total := nA + nB
	if total < 1 || total > 2 {
		t.Errorf("total flushed: got %d (A=%d, B=%d), want 1 or 2", total, nA, nB)
	}

	outboxPath := filepath.Join(root, ".gate", "outbox", "shared.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "shared-content" {
		t.Errorf("content: got %q, want %q", string(data), "shared-content")
	}
}

func TestDeadLetterCount_NoDeadLetters(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	ctx := context.Background()
	count, err := store.DeadLetterCount(ctx)
	if err != nil {
		t.Fatalf("DeadLetterCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 dead letters, got %d", count)
	}
}

func TestDeadLetterCount_AfterMaxRetries(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root")
	}
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	// Stage an item
	if err := store.Stage(ctx, "dead-letter.md", []byte("content")); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// Make archive dir unwritable to force flush failures
	archiveDir := filepath.Join(root, ".gate", "archive")
	os.Chmod(archiveDir, 0o555)
	defer os.Chmod(archiveDir, 0o755)

	// Flush 3 times to exceed maxRetryCount
	for range 3 {
		store.Flush(ctx) //nolint:errcheck
	}

	// Restore permissions
	os.Chmod(archiveDir, 0o755)

	// Now the item should be a dead letter
	count, err := store.DeadLetterCount(ctx)
	if err != nil {
		t.Fatalf("DeadLetterCount: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 dead letter, got %d", count)
	}

	// Flush should not pick up dead letters
	flushed, flushErr := store.Flush(ctx)
	if flushErr != nil {
		t.Fatalf("Flush after dead letter: %v", flushErr)
	}
	if flushed != 0 {
		t.Errorf("expected 0 flushed (dead letter skipped), got %d", flushed)
	}
}

func TestPurgeDeadLetters(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root")
	}
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)
	ctx := context.Background()

	// Stage and create dead letter
	if err := store.Stage(ctx, "dead-letter.md", []byte("content")); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	archiveDir := filepath.Join(root, ".gate", "archive")
	os.Chmod(archiveDir, 0o555)
	for range 3 {
		store.Flush(ctx) //nolint:errcheck
	}
	os.Chmod(archiveDir, 0o755)

	// Purge
	purged, err := store.PurgeDeadLetters(ctx)
	if err != nil {
		t.Fatalf("PurgeDeadLetters: %v", err)
	}
	if purged != 1 {
		t.Errorf("expected 1 purged, got %d", purged)
	}

	// Count should be 0 now
	count, err := store.DeadLetterCount(ctx)
	if err != nil {
		t.Fatalf("DeadLetterCount after purge: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 dead letters after purge, got %d", count)
	}
}
