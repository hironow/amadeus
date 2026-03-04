package session_test

import (
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

func ensureGateDirs(t *testing.T, root string) {
	t.Helper()
	for _, sub := range []string{".run", "archive", "outbox"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
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

	err := store.Stage("test-mail.md", []byte("hello"))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	archivePath := filepath.Join(root, "archive", "test-mail.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("archive content: got %q, want %q", string(data), "hello")
	}

	outboxPath := filepath.Join(root, "outbox", "test-mail.md")
	data, err = os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("outbox content: got %q, want %q", string(data), "hello")
	}
}

func TestSQLiteOutboxStore_StageIdempotent(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	if err := store.Stage("dup.md", []byte("first")); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}
	if err := store.Stage("dup.md", []byte("second")); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}

	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	outboxPath := filepath.Join(root, "outbox", "dup.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "first" {
		t.Errorf("content: got %q, want %q", string(data), "first")
	}
}

func TestSQLiteOutboxStore_FlushEmpty(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 0 {
		t.Errorf("flushed count: got %d, want 0", n)
	}
}

func TestSQLiteOutboxStore_FlushOnlyUnflushed(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	store.Stage("first.md", []byte("one"))
	store.Flush()

	store.Stage("second.md", []byte("two"))

	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	for _, name := range []string{"first.md", "second.md"} {
		outboxPath := filepath.Join(root, "outbox", name)
		if _, err := os.Stat(outboxPath); err != nil {
			t.Errorf("outbox %s missing: %v", name, err)
		}
	}
}

func TestSQLiteOutboxStore_MultipleStageThenFlush(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	store.Stage("a.md", []byte("aaa"))
	store.Stage("b.md", []byte("bbb"))
	store.Stage("c.md", []byte("ccc"))

	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 3 {
		t.Errorf("flushed count: got %d, want 3", n)
	}

	for _, name := range []string{"a.md", "b.md", "c.md"} {
		for _, sub := range []string{"archive", "outbox"} {
			p := filepath.Join(root, sub, name)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("%s/%s missing: %v", sub, name, err)
			}
		}
	}
}

func TestSQLiteOutboxStore_ConcurrentStageAndFlush(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)

	dbPath := filepath.Join(root, ".run", "outbox.db")
	archiveDir := filepath.Join(root, "archive")
	outboxDir := filepath.Join(root, "outbox")

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

	var wg sync.WaitGroup
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			name := fmt.Sprintf("a-%03d.md", i)
			if err := storeA.Stage(name, []byte("from-A-"+name)); err != nil {
				errA <- err
				return
			}
			if _, err := storeA.Flush(); err != nil {
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
			if err := storeB.Stage(name, []byte("from-B-"+name)); err != nil {
				errB <- err
				return
			}
			if _, err := storeB.Flush(); err != nil {
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
				p := filepath.Join(root, sub, name)
				data, readErr := os.ReadFile(p)
				if readErr != nil {
					t.Errorf("%s/%s missing: %v", sub, name, readErr)
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
	dbPath := filepath.Join(root, ".run", "outbox.db")
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

	dbPath := filepath.Join(root, ".run", "outbox.db")
	archiveDir := filepath.Join(root, "archive")
	outboxDir := filepath.Join(root, "outbox")

	store, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	store.Stage("fail.md", []byte("data"))

	// Make archive dir read-only so atomicWrite fails
	os.Chmod(archiveDir, 0o444)
	defer os.Chmod(archiveDir, 0o755)

	// when: flush 3 times (each fails, incrementing retry_count to 3)
	for i := range 3 {
		n, _ := store.Flush()
		if n != 0 {
			t.Errorf("flush %d: expected 0 flushed, got %d", i+1, n)
		}
	}

	// Restore permissions
	os.Chmod(archiveDir, 0o755)

	// when: flush again — item should be dead-letter
	n, err := store.Flush()
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

	dbPath := filepath.Join(root, ".run", "outbox.db")
	archiveDir := filepath.Join(root, "archive")
	outboxDir := filepath.Join(root, "outbox")

	store, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	store.Stage("retry.md", []byte("retry-data"))

	// First flush fails
	os.Chmod(archiveDir, 0o444)
	n, _ := store.Flush()
	if n != 0 {
		t.Errorf("first flush: expected 0, got %d", n)
	}

	// Restore — second flush succeeds
	os.Chmod(archiveDir, 0o755)
	n, err = store.Flush()
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

	dbPath := filepath.Join(root, ".run", "outbox.db")
	archiveDir := filepath.Join(root, "archive")
	outboxDir := filepath.Join(root, "outbox")

	storeSetup, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create setup store: %v", err)
	}
	if err := storeSetup.Stage("shared.md", []byte("shared-content")); err != nil {
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
		nA, eA = storeA.Flush()
	}()
	go func() {
		defer wg.Done()
		nB, eB = storeB.Flush()
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

	outboxPath := filepath.Join(root, "outbox", "shared.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "shared-content" {
		t.Errorf("content: got %q, want %q", string(data), "shared-content")
	}
}
