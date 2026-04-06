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

// AppendOutcomeTransition records an outcome transition as a new row (append-only).
// This preserves history: the same correlation_id may have multiple rows showing
// the transition from pending → failed_again → resolved.
func (s *SQLiteImprovementCollectorStore) AppendOutcomeTransition(ctx context.Context, correlationID string, outcome domain.ImprovementOutcome, failureType string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("improvement collector store: nil db")
	}
	now := time.Now().UTC()
	dedupKey := fmt.Sprintf("outcome-%s-%s-%s", correlationID, outcome, now.Format(time.RFC3339Nano))
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO improvement_signal(
			dedup_key, feedback_id, project_id, weave_ref, feedback_type, source_surface,
			schema_version, failure_type, severity, secondary_type, target_agent,
			routing_mode, routing_history, owner_history, recurrence_count,
			corrective_action, retry_allowed, escalation_reason, correlation_id,
			trace_id, outcome, ignored_reason, payload_json, created_at
		) VALUES (?, '', '', '', 'outcome_transition', 'amadeus_recheck',
			'1', ?, '', '', '',
			'', '', '', 0,
			'', '', '', ?,
			'', ?, '', '{}', ?)
	`, dedupKey, failureType, correlationID, string(outcome), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("improvement collector store: append outcome: %w", err)
	}
	return nil
}

// OutcomeStats holds aggregated outcome counts for a failure type.
type OutcomeStats struct {
	FailureType string
	Resolved    int
	FailedAgain int
	Escalated   int
	Pending     int
}

// GetOutcomeStats returns outcome counts grouped by failure_type.
func (s *SQLiteImprovementCollectorStore) GetOutcomeStats(ctx context.Context) ([]OutcomeStats, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("improvement collector store: nil db")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT failure_type, outcome, COUNT(*) as count
		FROM improvement_signal
		WHERE outcome != '' AND failure_type != ''
		GROUP BY failure_type, outcome
	`)
	if err != nil {
		return nil, fmt.Errorf("improvement collector store: query stats: %w", err)
	}
	defer rows.Close()

	statsMap := make(map[string]*OutcomeStats)
	for rows.Next() {
		var ft, outcome string
		var count int
		if err := rows.Scan(&ft, &outcome, &count); err != nil {
			return nil, fmt.Errorf("improvement collector store: scan stats: %w", err)
		}
		if _, ok := statsMap[ft]; !ok {
			statsMap[ft] = &OutcomeStats{FailureType: ft}
		}
		s := statsMap[ft]
		switch domain.ImprovementOutcome(outcome) {
		case domain.ImprovementOutcomeResolved:
			s.Resolved = count
		case domain.ImprovementOutcomeFailedAgain:
			s.FailedAgain = count
		case domain.ImprovementOutcomeEscalated:
			s.Escalated = count
		case domain.ImprovementOutcomePending:
			s.Pending = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("improvement collector store: iterate stats: %w", err)
	}

	result := make([]OutcomeStats, 0, len(statsMap))
	for _, v := range statsMap {
		result = append(result, *v)
	}
	return result, nil
}

// FailurePatternSummary holds aggregated failure pattern data.
type FailurePatternSummary struct {
	FailureType      string
	TotalOccurrences int
	ResolvedCount    int
	FailedAgainCount int
}

// GetFailurePatterns returns failure pattern summaries for the learning layer.
func (s *SQLiteImprovementCollectorStore) GetFailurePatterns(ctx context.Context) ([]FailurePatternSummary, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("improvement collector store: nil db")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT failure_type,
			COUNT(*) as total,
			SUM(CASE WHEN outcome='resolved' THEN 1 ELSE 0 END) as resolved,
			SUM(CASE WHEN outcome='failed_again' THEN 1 ELSE 0 END) as failed_again
		FROM improvement_signal
		WHERE failure_type != ''
		GROUP BY failure_type
		ORDER BY total DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("improvement collector store: query patterns: %w", err)
	}
	defer rows.Close()

	var result []FailurePatternSummary
	for rows.Next() {
		var s FailurePatternSummary
		if err := rows.Scan(&s.FailureType, &s.TotalOccurrences, &s.ResolvedCount, &s.FailedAgainCount); err != nil {
			return nil, fmt.Errorf("improvement collector store: scan pattern: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("improvement collector store: iterate patterns: %w", err)
	}
	return result, nil
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
