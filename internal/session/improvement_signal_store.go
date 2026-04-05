package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hironow/amadeus/internal/domain"

	_ "modernc.org/sqlite"
)

type ImprovementCursor struct {
	CreatedAt  time.Time
	FeedbackID string
}

type NormalizedImprovementSignal struct {
	DedupKey         string
	FeedbackID       string
	ProjectID        string
	WeaveRef         string
	FeedbackType     string
	SourceSurface    string
	SchemaVersion    string
	FailureType      string
	Severity         string
	SecondaryType    string
	TargetAgent      string
	RoutingMode      string
	RoutingHistory   []string
	OwnerHistory     []string
	RecurrenceCount  int
	CorrectiveAction string
	RetryAllowed     *bool
	EscalationReason string
	CorrelationID    string
	TraceID          string
	Outcome          string
	IgnoredReason    string
	PayloadJSON      string
	CreatedAt        time.Time
}

type SQLiteImprovementCollectorStore struct {
	db *sql.DB
}

const improvementCollectorSchema = `
CREATE TABLE IF NOT EXISTS improvement_ingestion_cursor (
	name TEXT PRIMARY KEY,
	created_at TEXT NOT NULL DEFAULT '',
	feedback_id TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS improvement_ingestion_seen (
	dedup_key TEXT PRIMARY KEY,
	created_at TEXT NOT NULL,
	feedback_id TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS improvement_signal (
	dedup_key TEXT PRIMARY KEY,
	feedback_id TEXT NOT NULL,
	project_id TEXT NOT NULL,
	weave_ref TEXT NOT NULL,
	feedback_type TEXT NOT NULL,
	source_surface TEXT NOT NULL,
	schema_version TEXT NOT NULL,
	failure_type TEXT NOT NULL,
	severity TEXT NOT NULL,
	secondary_type TEXT NOT NULL,
	target_agent TEXT NOT NULL,
	routing_mode TEXT NOT NULL,
	routing_history TEXT NOT NULL,
	owner_history TEXT NOT NULL,
	recurrence_count INTEGER NOT NULL,
	corrective_action TEXT NOT NULL,
	retry_allowed TEXT NOT NULL,
	escalation_reason TEXT NOT NULL,
	correlation_id TEXT NOT NULL,
	trace_id TEXT NOT NULL,
	outcome TEXT NOT NULL,
	ignored_reason TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TEXT NOT NULL
);
`

func NewSQLiteImprovementCollectorStore(dbPath string) (*SQLiteImprovementCollectorStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("improvement collector store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath) // nosemgrep: d4-sql-open-without-defer-close -- stored in struct, closed via Close() [permanent]
	if err != nil {
		return nil, fmt.Errorf("improvement collector store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("improvement collector store: pragma: %w", err)
		}
	}
	if _, err := db.Exec(improvementCollectorSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("improvement collector store: schema: %w", err)
	}
	return &SQLiteImprovementCollectorStore{db: db}, nil
}

func (s *SQLiteImprovementCollectorStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteImprovementCollectorStore) LoadCursor(ctx context.Context) (ImprovementCursor, error) {
	if s == nil || s.db == nil {
		return ImprovementCursor{}, fmt.Errorf("improvement collector store: nil db")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT created_at, feedback_id FROM improvement_ingestion_cursor WHERE name = ?`,
		"default",
	)
	var createdAtRaw string
	var cursor ImprovementCursor
	err := row.Scan(&createdAtRaw, &cursor.FeedbackID)
	if errors.Is(err, sql.ErrNoRows) {
		return ImprovementCursor{}, nil
	}
	if err != nil {
		return ImprovementCursor{}, fmt.Errorf("improvement collector store: load cursor: %w", err)
	}
	if createdAtRaw != "" {
		cursor.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtRaw)
		if err != nil {
			return ImprovementCursor{}, fmt.Errorf("improvement collector store: parse cursor time: %w", err)
		}
	}
	return cursor, nil
}

func (s *SQLiteImprovementCollectorStore) ApplyFeedback(ctx context.Context, row ImprovementFeedbackRow, signal NormalizedImprovementSignal, appendEntry func() error) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("improvement collector store: nil db")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("improvement collector store: get conn: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, fmt.Errorf("improvement collector store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	dedupKey := improvementFeedbackDedupKey(row)
	var exists int
	err = conn.QueryRowContext(ctx,
		`SELECT 1 FROM improvement_ingestion_seen WHERE dedup_key = ?`,
		dedupKey,
	).Scan(&exists)
	switch {
	case err == nil:
		if err := saveImprovementCursor(ctx, conn, row); err != nil {
			return false, err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return false, fmt.Errorf("improvement collector store: commit duplicate: %w", err)
		}
		committed = true
		return false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return false, fmt.Errorf("improvement collector store: check dedup: %w", err)
	}

	if err := saveImprovementSignal(ctx, conn, signal); err != nil {
		return false, err
	}
	if err := appendEntry(); err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO improvement_ingestion_seen(dedup_key, created_at, feedback_id) VALUES (?, ?, ?)`,
		dedupKey,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
		row.ID,
	); err != nil {
		return false, fmt.Errorf("improvement collector store: insert dedup: %w", err)
	}
	if err := saveImprovementCursor(ctx, conn, row); err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, fmt.Errorf("improvement collector store: commit: %w", err)
	}
	committed = true
	return true, nil
}

func (s *SQLiteImprovementCollectorStore) LoadSignals(ctx context.Context, limit int) ([]NormalizedImprovementSignal, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("improvement collector store: nil db")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			dedup_key, feedback_id, project_id, weave_ref, feedback_type, source_surface,
			schema_version, failure_type, severity, secondary_type, target_agent,
			routing_mode, routing_history, owner_history, recurrence_count,
			corrective_action, retry_allowed, escalation_reason, correlation_id,
			trace_id, outcome, ignored_reason, payload_json, created_at
		FROM improvement_signal
		ORDER BY created_at ASC, feedback_id ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("improvement collector store: load signals: %w", err)
	}
	defer rows.Close()

	var out []NormalizedImprovementSignal
	for rows.Next() {
		var signal NormalizedImprovementSignal
		var retryAllowedRaw string
		var routingHistoryRaw string
		var ownerHistoryRaw string
		var createdAtRaw string
		if err := rows.Scan(
			&signal.DedupKey,
			&signal.FeedbackID,
			&signal.ProjectID,
			&signal.WeaveRef,
			&signal.FeedbackType,
			&signal.SourceSurface,
			&signal.SchemaVersion,
			&signal.FailureType,
			&signal.Severity,
			&signal.SecondaryType,
			&signal.TargetAgent,
			&signal.RoutingMode,
			&routingHistoryRaw,
			&ownerHistoryRaw,
			&signal.RecurrenceCount,
			&signal.CorrectiveAction,
			&retryAllowedRaw,
			&signal.EscalationReason,
			&signal.CorrelationID,
			&signal.TraceID,
			&signal.Outcome,
			&signal.IgnoredReason,
			&signal.PayloadJSON,
			&createdAtRaw,
		); err != nil {
			return nil, fmt.Errorf("improvement collector store: scan signal: %w", err)
		}
		if retryAllowedRaw != "" {
			parsed, err := strconv.ParseBool(retryAllowedRaw)
			if err != nil {
				return nil, fmt.Errorf("improvement collector store: parse retry_allowed: %w", err)
			}
			signal.RetryAllowed = domain.BoolPtr(parsed)
		}
		signal.RoutingHistory = domain.ParseImprovementHistory(routingHistoryRaw)
		signal.OwnerHistory = domain.ParseImprovementHistory(ownerHistoryRaw)
		if createdAtRaw != "" {
			createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
			if err != nil {
				return nil, fmt.Errorf("improvement collector store: parse signal time: %w", err)
			}
			signal.CreatedAt = createdAt
		}
		out = append(out, signal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("improvement collector store: iterate signals: %w", err)
	}
	return out, nil
}

func saveImprovementCursor(ctx context.Context, conn *sql.Conn, row ImprovementFeedbackRow) error {
	_, err := conn.ExecContext(ctx,
		`INSERT INTO improvement_ingestion_cursor(name, created_at, feedback_id)
		VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET created_at = excluded.created_at, feedback_id = excluded.feedback_id`,
		"default",
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
		row.ID,
	)
	if err != nil {
		return fmt.Errorf("improvement collector store: save cursor: %w", err)
	}
	return nil
}

func saveImprovementSignal(ctx context.Context, conn *sql.Conn, signal NormalizedImprovementSignal) error {
	retryAllowed := ""
	if signal.RetryAllowed != nil {
		retryAllowed = strconv.FormatBool(*signal.RetryAllowed)
	}
	_, err := conn.ExecContext(ctx, `
		INSERT INTO improvement_signal(
			dedup_key, feedback_id, project_id, weave_ref, feedback_type, source_surface,
			schema_version, failure_type, severity, secondary_type, target_agent,
			routing_mode, routing_history, owner_history, recurrence_count,
			corrective_action, retry_allowed, escalation_reason, correlation_id,
			trace_id, outcome, ignored_reason, payload_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		signal.DedupKey,
		signal.FeedbackID,
		signal.ProjectID,
		signal.WeaveRef,
		signal.FeedbackType,
		signal.SourceSurface,
		signal.SchemaVersion,
		signal.FailureType,
		signal.Severity,
		signal.SecondaryType,
		signal.TargetAgent,
		signal.RoutingMode,
		domain.FormatImprovementHistory(signal.RoutingHistory),
		domain.FormatImprovementHistory(signal.OwnerHistory),
		signal.RecurrenceCount,
		signal.CorrectiveAction,
		retryAllowed,
		signal.EscalationReason,
		signal.CorrelationID,
		signal.TraceID,
		signal.Outcome,
		signal.IgnoredReason,
		signal.PayloadJSON,
		signal.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("improvement collector store: insert signal: %w", err)
	}
	return nil
}

func marshalImprovementPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}
