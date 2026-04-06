package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"

	_ "modernc.org/sqlite"
)

// ImprovementTaskDispatcher dispatches improvement tasks with idempotent dedup.
// Uses SQLite to ensure the same (correlationID, failureType) combination
// is dispatched at most once.
type ImprovementTaskDispatcher struct {
	db     *sql.DB
	logger domain.Logger
}

// NewImprovementTaskDispatcher creates a dispatcher with SQLite dedup store.
func NewImprovementTaskDispatcher(stateDir string, logger domain.Logger) (*ImprovementTaskDispatcher, error) {
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("improvement task dispatcher: create dir: %w", err)
	}

	dbPath := filepath.Join(runDir, "improvement_tasks.db")
	db, err := sql.Open("sqlite", dbPath) // nosemgrep: d4-sql-open-without-defer-close -- stored in struct, closed via Close() [permanent]
	if err != nil {
		return nil, fmt.Errorf("improvement task dispatcher: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("improvement task dispatcher: set WAL: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS improvement_tasks (
		dedup_key   TEXT PRIMARY KEY,
		task_id     TEXT NOT NULL,
		source_event TEXT NOT NULL,
		target_agent TEXT NOT NULL,
		suggested_action TEXT NOT NULL,
		failure_type TEXT NOT NULL,
		severity    TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		expires_at  TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("improvement task dispatcher: create schema: %w", err)
	}

	return &ImprovementTaskDispatcher{db: db, logger: logger}, nil
}

// Dispatch stores and logs an improvement task, skipping duplicates.
// Dedup key = correlationID + "-" + failureType.
func (d *ImprovementTaskDispatcher) Dispatch(ctx context.Context, task domain.ImprovementTask, correlationID string) error {
	dedupKey := correlationID + "-" + string(task.FailureType)

	result, err := d.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO improvement_tasks (dedup_key, task_id, source_event, target_agent, suggested_action, failure_type, severity, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dedupKey,
		task.ID,
		task.SourceEvent,
		task.TargetAgent,
		task.SuggestedAction,
		string(task.FailureType),
		string(task.Severity),
		task.CreatedAt.UTC().Format(time.RFC3339Nano),
		task.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("improvement task dispatch: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("improvement task dispatch: rows: %w", err)
	}

	if rows == 0 {
		d.logger.Debug("Improvement task skipped (duplicate): %s", dedupKey)
		return nil
	}

	d.logger.Info("Improvement task dispatched: %s → %s (%s)", task.SourceEvent, task.TargetAgent, task.SuggestedAction)
	return nil
}

// Close releases database resources.
func (d *ImprovementTaskDispatcher) Close() error {
	return d.db.Close()
}
