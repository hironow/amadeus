package session_test

// white-box-reason: tests closeReadyIssues orchestration internals (label listing, close calls, error handling)

import (
	"context"
	"fmt"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// --- issue close test doubles ---

type mockIssueWriter struct {
	openIssues map[string][]string // label -> issue numbers
	closed     []string            // issue numbers that were closed
	closeErr   error               // error to return from CloseIssue
	listErr    error               // error to return from ListOpenIssuesByLabel
}

func (m *mockIssueWriter) ListOpenIssuesByLabel(_ context.Context, label string) ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.openIssues[label], nil
}

func (m *mockIssueWriter) CloseIssue(_ context.Context, issueNumber, _ string) error {
	if m.closeErr != nil {
		return m.closeErr
	}
	m.closed = append(m.closed, issueNumber)
	return nil
}

func TestCloseReadyIssues_ClosesLabeledIssues(t *testing.T) {
	// given: 3 open issues with the ready label
	mock := &mockIssueWriter{
		openIssues: map[string][]string{
			"sightjack:ready": {"13", "5", "21"},
		},
	}
	a := &session.Amadeus{
		Logger:      &domain.NopLogger{},
		IssueWriter: mock,
	}

	// when
	session.ExportCloseReadyIssues(a, context.Background(), "sightjack:ready")

	// then: all 3 issues closed
	if len(mock.closed) != 3 {
		t.Fatalf("expected 3 closed issues, got %d: %v", len(mock.closed), mock.closed)
	}
	expected := map[string]bool{"13": true, "5": true, "21": true}
	for _, num := range mock.closed {
		if !expected[num] {
			t.Errorf("unexpected closed issue: %s", num)
		}
	}
}

func TestCloseReadyIssues_EmptyList(t *testing.T) {
	// given: no issues with the ready label
	mock := &mockIssueWriter{
		openIssues: map[string][]string{},
	}
	a := &session.Amadeus{
		Logger:      &domain.NopLogger{},
		IssueWriter: mock,
	}

	// when
	session.ExportCloseReadyIssues(a, context.Background(), "sightjack:ready")

	// then: no close calls
	if len(mock.closed) != 0 {
		t.Errorf("expected 0 closed issues, got %d", len(mock.closed))
	}
}

func TestCloseReadyIssues_NilIssueWriter(t *testing.T) {
	// given: IssueWriter is nil (no --base flag)
	a := &session.Amadeus{
		Logger:      &domain.NopLogger{},
		IssueWriter: nil,
	}

	// when: should not panic
	session.ExportCloseReadyIssues(a, context.Background(), "sightjack:ready")
}

func TestCloseReadyIssues_EmptyLabel(t *testing.T) {
	// given: empty ready label
	mock := &mockIssueWriter{
		openIssues: map[string][]string{
			"sightjack:ready": {"1"},
		},
	}
	a := &session.Amadeus{
		Logger:      &domain.NopLogger{},
		IssueWriter: mock,
	}

	// when
	session.ExportCloseReadyIssues(a, context.Background(), "")

	// then: no close calls (empty label = disabled)
	if len(mock.closed) != 0 {
		t.Errorf("expected 0 closed issues, got %d", len(mock.closed))
	}
}

func TestCloseReadyIssues_CloseError_ContinuesOthers(t *testing.T) {
	// given: close fails for one issue
	callCount := 0
	mock := &mockIssueWriter{
		openIssues: map[string][]string{
			"sightjack:ready": {"1", "2", "3"},
		},
		closeErr: fmt.Errorf("gh error"),
	}
	// Override CloseIssue to fail on first call only
	mock.closeErr = nil
	a := &session.Amadeus{
		Logger: &domain.NopLogger{},
		IssueWriter: &failOnceIssueWriter{
			inner:     mock,
			failOnIdx: 0,
			callCount: &callCount,
		},
	}

	// when
	session.ExportCloseReadyIssues(a, context.Background(), "sightjack:ready")

	// then: 2 of 3 issues closed (first one failed, others continued)
	if len(mock.closed) != 2 {
		t.Errorf("expected 2 closed issues (1 failed), got %d: %v", len(mock.closed), mock.closed)
	}
}

func TestCloseReadyIssues_ListError_NoCloseCalls(t *testing.T) {
	// given: list fails
	mock := &mockIssueWriter{
		listErr: fmt.Errorf("gh list error"),
	}
	a := &session.Amadeus{
		Logger:      &domain.NopLogger{},
		IssueWriter: mock,
	}

	// when
	session.ExportCloseReadyIssues(a, context.Background(), "sightjack:ready")

	// then: no close calls
	if len(mock.closed) != 0 {
		t.Errorf("expected 0 closed issues, got %d", len(mock.closed))
	}
}

// failOnceIssueWriter wraps mockIssueWriter but fails CloseIssue on a specific call index.
type failOnceIssueWriter struct {
	inner     *mockIssueWriter
	failOnIdx int
	callCount *int
}

func (f *failOnceIssueWriter) ListOpenIssuesByLabel(ctx context.Context, label string) ([]string, error) {
	return f.inner.ListOpenIssuesByLabel(ctx, label)
}

func (f *failOnceIssueWriter) CloseIssue(ctx context.Context, issueNumber, comment string) error {
	idx := *f.callCount
	*f.callCount++
	if idx == f.failOnIdx {
		return fmt.Errorf("simulated close failure for issue %s", issueNumber)
	}
	return f.inner.CloseIssue(ctx, issueNumber, comment)
}

// --- version test ---
