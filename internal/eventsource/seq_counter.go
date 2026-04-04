package eventsource

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SeqCounter provides globally monotonic sequence number allocation
// backed by SQLite WAL. Safe for concurrent multi-process use.
type SeqCounter struct {
	db *sql.DB
}

// NewSeqCounter opens (or creates) a SQLite database at dbPath and
// initialises the seq_counter schema.
func NewSeqCounter(dbPath string) (*SeqCounter, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("seq counter: create dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath) // nosemgrep: d4-sql-open-without-defer-close -- stored in struct, closed via Close() [permanent]
	if err != nil {
		return nil, fmt.Errorf("seq counter: open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("seq counter: %s: %w", pragma, err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS seq_counter (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		next_seq INTEGER NOT NULL DEFAULT 1
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seq counter: create table: %w", err)
	}
	// Ensure the single row exists.
	if _, err := db.Exec(`INSERT OR IGNORE INTO seq_counter (id, next_seq) VALUES (1, 1)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seq counter: init row: %w", err)
	}
	return &SeqCounter{db: db}, nil
}

// AllocSeqNr atomically increments and returns the next global SeqNr.
// Safe for concurrent multi-process use via SQLite WAL locking.
func (c *SeqCounter) AllocSeqNr(_ context.Context) (uint64, error) {
	var seq uint64
	err := c.db.QueryRow(
		`UPDATE seq_counter SET next_seq = next_seq + 1 RETURNING next_seq - 1`,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("seq counter: alloc: %w", err)
	}
	return seq, nil
}

// LatestSeqNr returns the highest allocated SeqNr (next_seq - 1).
// Returns 0 if no allocations have been made.
func (c *SeqCounter) LatestSeqNr(_ context.Context) (uint64, error) {
	var nextSeq uint64
	err := c.db.QueryRow(`SELECT next_seq FROM seq_counter WHERE id = 1`).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("seq counter: latest: %w", err)
	}
	if nextSeq <= 1 {
		return 0, nil
	}
	return nextSeq - 1, nil
}

// InitializeAt sets the counter so the next allocation returns startAt + 1,
// but only if the counter has not already advanced past startAt.
// This is used during cutover to set the initial sequence number.
func (c *SeqCounter) InitializeAt(_ context.Context, startAt uint64) error {
	_, err := c.db.Exec(
		`UPDATE seq_counter SET next_seq = ? WHERE id = 1 AND next_seq <= ?`,
		startAt+1, startAt+1,
	)
	if err != nil {
		return fmt.Errorf("seq counter: initialize: %w", err)
	}
	return nil
}

// Close releases the database connection.
func (c *SeqCounter) Close() error {
	return c.db.Close()
}
