package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hironow/amadeus/internal/usecase/port"

	_ "modernc.org/sqlite"
)

// Compile-time check that SQLiteRunLockStore implements port.RunLockStore.
var _ port.RunLockStore = (*SQLiteRunLockStore)(nil)

// SQLiteRunLockStore implements RunLockStore using SQLite WAL.
// Provides cross-process run locking with automatic stale lock cleanup.
type SQLiteRunLockStore struct {
	db       *sql.DB
	holderID string
}

// NewSQLiteRunLockStore opens (or creates) a SQLite run lock store at dbPath.
// Each instance gets a unique holder ID (UUID) for lock ownership.
func NewSQLiteRunLockStore(dbPath string) (*SQLiteRunLockStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("run lock store: create dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("run lock store: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("run lock store: set WAL: %w", err)
	}

	if err := createRunLockSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run lock store: create schema: %w", err)
	}

	return &SQLiteRunLockStore{
		db:       db,
		holderID: uuid.New().String(),
	}, nil
}

func createRunLockSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS run_locks (
		key         TEXT PRIMARY KEY,
		holder      TEXT NOT NULL,
		acquired_at TEXT NOT NULL,
		expires_at  TEXT NOT NULL
	)`)
	return err
}

// TryAcquire attempts to acquire a lock for the given run key.
// Stale locks (past expires_at) are cleaned up automatically before the attempt.
func (s *SQLiteRunLockStore) TryAcquire(ctx context.Context, runKey string, ttl time.Duration) (bool, string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	// Clean up stale locks first
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM run_locks WHERE key = ? AND expires_at < ?`,
		runKey, now.Format(time.RFC3339Nano))
	if err != nil {
		return false, "", fmt.Errorf("run lock: cleanup stale: %w", err)
	}

	// Try to insert
	result, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO run_locks (key, holder, acquired_at, expires_at) VALUES (?, ?, ?, ?)`,
		runKey, s.holderID, now.Format(time.RFC3339Nano), expiresAt.Format(time.RFC3339Nano))
	if err != nil {
		return false, "", fmt.Errorf("run lock: insert: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, "", fmt.Errorf("run lock: rows affected: %w", err)
	}

	if rows > 0 {
		return true, s.holderID, nil
	}

	// Lock already held — return the current holder
	var holder string
	err = s.db.QueryRowContext(ctx,
		`SELECT holder FROM run_locks WHERE key = ?`, runKey).Scan(&holder)
	if err != nil {
		return false, "", fmt.Errorf("run lock: query holder: %w", err)
	}
	return false, holder, nil
}

// Release releases a lock previously acquired by this holder.
func (s *SQLiteRunLockStore) Release(ctx context.Context, runKey string, holder string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM run_locks WHERE key = ? AND holder = ?`,
		runKey, holder)
	if err != nil {
		return fmt.Errorf("run lock: release: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("run lock: release rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("run lock: release: lock not held by %s", holder)
	}
	return nil
}

// IsHeld returns whether the lock is currently held and by whom.
// Expired locks are not considered held.
func (s *SQLiteRunLockStore) IsHeld(ctx context.Context, runKey string) (bool, string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var holder string
	err := s.db.QueryRowContext(ctx,
		`SELECT holder FROM run_locks WHERE key = ? AND expires_at >= ?`,
		runKey, now).Scan(&holder)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("run lock: is held: %w", err)
	}
	return true, holder, nil
}

// Close releases database resources.
func (s *SQLiteRunLockStore) Close() error {
	return s.db.Close()
}
