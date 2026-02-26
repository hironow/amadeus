package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	amadeus "github.com/hironow/amadeus"

	_ "modernc.org/sqlite"
)

// Compile-time check that SQLiteOutboxStore implements amadeus.OutboxStore.
var _ amadeus.OutboxStore = (*SQLiteOutboxStore)(nil)

// SQLiteOutboxStore implements OutboxStore using a SQLite database as the
// transactional write-ahead log. Staged D-Mails are flushed to archive/ and
// outbox/ using atomic file writes (temp file + rename).
type SQLiteOutboxStore struct {
	db         *sql.DB
	archiveDir string
	outboxDir  string
}

// NewSQLiteOutboxStore opens (or creates) a SQLite database at dbPath and
// initialises the schema. archiveDir and outboxDir are the target directories
// for flushed D-Mail files.
func NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir string) (*SQLiteOutboxStore, error) {
	for _, dir := range []string{filepath.Dir(dbPath), archiveDir, outboxDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("outbox store: create dir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("outbox store: open db: %w", err)
	}
	// SQLite is a single-file database: limit to one connection to prevent
	// "database is locked" errors from the Go connection pool. WAL mode
	// handles concurrent access from OTHER processes; this setting governs
	// connections within THIS process.
	db.SetMaxOpenConns(1)
	// Set PRAGMAs explicitly — modernc.org/sqlite does not support
	// underscore-prefixed query parameters like mattn/go-sqlite3.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("outbox store: %s: %w", pragma, err)
		}
	}
	if err := createOutboxSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteOutboxStore{
		db:         db,
		archiveDir: archiveDir,
		outboxDir:  outboxDir,
	}, nil
}

func createOutboxSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS staged (
		name    TEXT PRIMARY KEY,
		data    BLOB    NOT NULL,
		flushed INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return fmt.Errorf("outbox store: create schema: %w", err)
	}
	return nil
}

// Stage inserts a D-Mail into the staging table. Idempotent: re-staging the
// same name is silently ignored (INSERT OR IGNORE).
func (s *SQLiteOutboxStore) Stage(name string, data []byte) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO staged (name, data) VALUES (?, ?)`, name, data)
	if err != nil {
		return fmt.Errorf("outbox store: stage %s: %w", name, err)
	}
	return nil
}

// Flush writes all unflushed D-Mails to archive/ and outbox/ using atomic
// file writes, then marks them as flushed in the database. The entire flush
// is wrapped in a BEGIN IMMEDIATE transaction so that concurrent CLI
// processes wait (up to busy_timeout) instead of deadlocking. A partial
// failure leaves items eligible for retry on the next Flush call.
func (s *SQLiteOutboxStore) Flush() (int, error) {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("outbox store: get conn: %w", err)
	}
	defer conn.Close()

	// BEGIN IMMEDIATE acquires a RESERVED lock immediately, preventing
	// the SHARED→EXCLUSIVE deadlock that occurs with DEFERRED transactions
	// when two connections SELECT then UPDATE concurrently.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return 0, fmt.Errorf("outbox store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	rows, err := conn.QueryContext(ctx, `SELECT name, data FROM staged WHERE flushed = 0`)
	if err != nil {
		return 0, fmt.Errorf("outbox store: query staged: %w", err)
	}

	type item struct {
		name string
		data []byte
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.name, &it.data); err != nil {
			rows.Close()
			return 0, fmt.Errorf("outbox store: scan row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("outbox store: rows iter: %w", err)
	}

	if len(items) == 0 {
		conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		committed = true
		return 0, nil
	}

	for _, it := range items {
		archivePath := filepath.Join(s.archiveDir, it.name)
		if err := atomicWrite(archivePath, it.data); err != nil {
			return 0, fmt.Errorf("outbox store: atomic write archive %s: %w", it.name, err)
		}
		outboxPath := filepath.Join(s.outboxDir, it.name)
		if err := atomicWrite(outboxPath, it.data); err != nil {
			return 0, fmt.Errorf("outbox store: atomic write outbox %s: %w", it.name, err)
		}
	}

	for _, it := range items {
		if _, err := conn.ExecContext(ctx, `UPDATE staged SET flushed = 1 WHERE name = ?`, it.name); err != nil {
			return 0, fmt.Errorf("outbox store: mark flushed %s: %w", it.name, err)
		}
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, fmt.Errorf("outbox store: commit: %w", err)
	}
	committed = true
	return len(items), nil
}

// Close closes the underlying database connection.
func (s *SQLiteOutboxStore) Close() error {
	return s.db.Close()
}

// NewOutboxStoreForGateDir creates a SQLiteOutboxStore using conventional paths
// derived from the .gate/ directory: DB at .run/outbox.db, targets at archive/
// and outbox/.
func NewOutboxStoreForGateDir(root string) (*SQLiteOutboxStore, error) {
	dbPath := filepath.Join(root, ".run", "outbox.db")
	archiveDir := filepath.Join(root, "archive")
	outboxDir := filepath.Join(root, "outbox")
	return NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
}

// atomicWrite writes data to a temporary file in the same directory, then
// renames it to the target path (atomic on the same filesystem).
func atomicWrite(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, ".amadeus-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, targetPath)
}
