package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"

	_ "modernc.org/sqlite"
)

// Compile-time check.
var _ port.CodingSessionStore = (*SQLiteCodingSessionStore)(nil)

// SQLiteCodingSessionStore implements CodingSessionStore using SQLite.
type SQLiteCodingSessionStore struct {
	db *sql.DB
}

const codingSessionSchema = `
CREATE TABLE IF NOT EXISTS coding_sessions (
	id                  TEXT PRIMARY KEY,
	provider_session_id TEXT DEFAULT '',
	provider            TEXT NOT NULL,
	status              TEXT NOT NULL DEFAULT 'running',
	model               TEXT DEFAULT '',
	work_dir            TEXT DEFAULT '',
	created_at          TEXT NOT NULL,
	updated_at          TEXT NOT NULL,
	metadata            TEXT DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_cs_provider_sid ON coding_sessions(provider_session_id);
CREATE INDEX IF NOT EXISTS idx_cs_status ON coding_sessions(status);
CREATE INDEX IF NOT EXISTS idx_cs_created ON coding_sessions(created_at);
`

// NewSQLiteCodingSessionStore opens or creates a SQLite database at dbPath.
func NewSQLiteCodingSessionStore(dbPath string) (*SQLiteCodingSessionStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("coding session store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath) // nosemgrep: d4-sql-open-without-defer-close — stored in struct, closed via Close() [permanent]
	if err != nil {
		return nil, fmt.Errorf("coding session store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("coding session store: pragma: %w", err)
		}
	}
	if _, err := db.Exec(codingSessionSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("coding session store: schema: %w", err)
	}
	return &SQLiteCodingSessionStore{db: db}, nil
}

func (s *SQLiteCodingSessionStore) Save(ctx context.Context, record domain.CodingSessionRecord) error {
	meta, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO coding_sessions
		(id, provider_session_id, provider, status, model, work_dir, created_at, updated_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.ProviderSessionID,
		string(record.Provider),
		string(record.Status),
		record.Model,
		record.WorkDir,
		record.CreatedAt.Format(time.RFC3339Nano),
		record.UpdatedAt.Format(time.RFC3339Nano),
		string(meta),
	)
	return err
}

func (s *SQLiteCodingSessionStore) Load(ctx context.Context, id string) (domain.CodingSessionRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_session_id, provider, status, model, work_dir, created_at, updated_at, metadata
		FROM coding_sessions WHERE id = ?`, id)
	return scanRecord(row)
}

func (s *SQLiteCodingSessionStore) FindByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) ([]domain.CodingSessionRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_session_id, provider, status, model, work_dir, created_at, updated_at, metadata
		FROM coding_sessions WHERE provider = ? AND provider_session_id = ? ORDER BY created_at ASC`,
		string(provider), pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (s *SQLiteCodingSessionStore) LatestByProviderSessionID(ctx context.Context, provider domain.Provider, pid string) (domain.CodingSessionRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_session_id, provider, status, model, work_dir, created_at, updated_at, metadata
		FROM coding_sessions WHERE provider = ? AND provider_session_id = ? ORDER BY created_at DESC LIMIT 1`,
		string(provider), pid)
	return scanRecord(row)
}

func (s *SQLiteCodingSessionStore) List(ctx context.Context, opts port.ListSessionOpts) ([]domain.CodingSessionRecord, error) {
	query := `SELECT id, provider_session_id, provider, status, model, work_dir, created_at, updated_at, metadata FROM coding_sessions WHERE 1=1`
	var args []any
	if opts.Provider != nil {
		query += " AND provider = ?"
		args = append(args, string(*opts.Provider))
	}
	if opts.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*opts.Status))
	}
	query += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (s *SQLiteCodingSessionStore) UpdateStatus(ctx context.Context, id string, status domain.SessionStatus, providerSessionID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE coding_sessions SET status = ?, provider_session_id = ?, updated_at = ? WHERE id = ?`,
		string(status), providerSessionID, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %q not found", id)
	}
	return nil
}

func (s *SQLiteCodingSessionStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (domain.CodingSessionRecord, error) {
	var rec domain.CodingSessionRecord
	var provider, status, createdAt, updatedAt, metaJSON string
	err := row.Scan(&rec.ID, &rec.ProviderSessionID, &provider, &status,
		&rec.Model, &rec.WorkDir, &createdAt, &updatedAt, &metaJSON)
	if err != nil {
		return rec, err
	}
	rec.Provider = domain.Provider(provider)
	rec.Status = domain.SessionStatus(status)
	rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	rec.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if metaJSON != "" {
		_ = json.Unmarshal([]byte(metaJSON), &rec.Metadata)
	}
	return rec, nil
}

func scanRecords(rows *sql.Rows) ([]domain.CodingSessionRecord, error) {
	var records []domain.CodingSessionRecord
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
