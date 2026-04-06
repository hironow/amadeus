package session_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func newTestImprovementStore(t *testing.T) *session.SQLiteImprovementCollectorStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), ".run", "improvement-ingestion.db")
	store, err := session.NewSQLiteImprovementCollectorStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestAppendOutcomeTransition(t *testing.T) {
	// given
	store := newTestImprovementStore(t)
	ctx := context.Background()

	// when — append pending transition
	err := store.AppendOutcomeTransition(ctx, "corr-001", domain.ImprovementOutcomePending, "execution_failure")
	if err != nil {
		t.Fatalf("append pending: %v", err)
	}

	// when — append resolved transition for same correlation
	err = store.AppendOutcomeTransition(ctx, "corr-001", domain.ImprovementOutcomeResolved, "execution_failure")
	if err != nil {
		t.Fatalf("append resolved: %v", err)
	}

	// then — both should exist (append-only, no overwrite)
	signals, err := store.LoadSignals(ctx, 100)
	if err != nil {
		t.Fatalf("load signals: %v", err)
	}

	found := 0
	for _, s := range signals {
		if s.CorrelationID == "corr-001" {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 signals for corr-001, got %d", found)
	}
}

func TestGetOutcomeStats(t *testing.T) {
	// given
	store := newTestImprovementStore(t)
	ctx := context.Background()

	// seed data
	_ = store.AppendOutcomeTransition(ctx, "c1", domain.ImprovementOutcomeResolved, "execution_failure")
	_ = store.AppendOutcomeTransition(ctx, "c2", domain.ImprovementOutcomeFailedAgain, "execution_failure")
	_ = store.AppendOutcomeTransition(ctx, "c3", domain.ImprovementOutcomeResolved, "scope_violation")
	_ = store.AppendOutcomeTransition(ctx, "c4", domain.ImprovementOutcomeEscalated, "scope_violation")

	// when
	stats, err := store.GetOutcomeStats(ctx)

	// then
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("expected non-empty stats")
	}

	// Check execution_failure stats
	for _, s := range stats {
		if s.FailureType == "execution_failure" {
			if s.Resolved != 1 {
				t.Errorf("execution_failure resolved = %d, want 1", s.Resolved)
			}
			if s.FailedAgain != 1 {
				t.Errorf("execution_failure failed_again = %d, want 1", s.FailedAgain)
			}
		}
		if s.FailureType == "scope_violation" {
			if s.Resolved != 1 {
				t.Errorf("scope_violation resolved = %d, want 1", s.Resolved)
			}
			if s.Escalated != 1 {
				t.Errorf("scope_violation escalated = %d, want 1", s.Escalated)
			}
		}
	}
}

func TestGetFailurePatterns(t *testing.T) {
	// given
	store := newTestImprovementStore(t)
	ctx := context.Background()

	_ = store.AppendOutcomeTransition(ctx, "c1", domain.ImprovementOutcomeResolved, "execution_failure")
	_ = store.AppendOutcomeTransition(ctx, "c2", domain.ImprovementOutcomeFailedAgain, "execution_failure")
	_ = store.AppendOutcomeTransition(ctx, "c3", domain.ImprovementOutcomeResolved, "scope_violation")

	// when
	patterns, err := store.GetFailurePatterns(ctx)

	// then
	if err != nil {
		t.Fatalf("get patterns: %v", err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected non-empty patterns")
	}
	for _, p := range patterns {
		if p.FailureType == "execution_failure" {
			if p.TotalOccurrences != 2 {
				t.Errorf("execution_failure total = %d, want 2", p.TotalOccurrences)
			}
			if p.ResolvedCount != 1 {
				t.Errorf("execution_failure resolved = %d, want 1", p.ResolvedCount)
			}
		}
	}
}
