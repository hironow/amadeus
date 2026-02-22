package amadeus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// spanNames extracts span names from a slice of SpanStubs.
func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}

// containsSpan returns true if any span in stubs has the given name.
func containsSpan(spans tracetest.SpanStubs, name string) bool {
	for _, s := range spans {
		if s.Name == name {
			return true
		}
	}
	return false
}

// newTestAmadeus creates an Amadeus instance wired for testing.
func newTestAmadeus(t *testing.T, repoRoot string) *Amadeus {
	t.Helper()
	divRoot := filepath.Join(repoRoot, ".gate")
	if err := os.MkdirAll(divRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	store := NewStateStore(divRoot)
	logger := NewLogger(&bytes.Buffer{}, false)
	git := NewGitClient(repoRoot)
	return &Amadeus{Config: cfg, Store: store, Git: git, Logger: logger, DataOut: &bytes.Buffer{}}
}

func TestRunCheck_CreatesRootSpan(t *testing.T) {
	// given: a test tracer and an Amadeus instance with a real git repo
	exp := setupTestTracer(t)
	repo := setupTestRepo(t)
	a := newTestAmadeus(t, repo.dir)

	// when: RunCheck is called with DryRun (skips Claude call)
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})
	if err != nil {
		t.Fatalf("RunCheck failed: %v", err)
	}

	// then: an "amadeus.check" root span should exist
	spans := exp.GetSpans()
	if !containsSpan(spans, "amadeus.check") {
		t.Errorf("expected root span 'amadeus.check', got spans: %v", spanNames(spans))
	}

	// then: root span should have check.full and check.dry_run attributes
	for _, s := range spans {
		if s.Name == "amadeus.check" {
			foundFull := false
			foundDryRun := false
			for _, attr := range s.Attributes {
				if string(attr.Key) == "check.full" {
					foundFull = true
					if !attr.Value.AsBool() {
						t.Errorf("expected check.full=true, got false")
					}
				}
				if string(attr.Key) == "check.dry_run" {
					foundDryRun = true
					if !attr.Value.AsBool() {
						t.Errorf("expected check.dry_run=true, got false")
					}
				}
			}
			if !foundFull {
				t.Error("expected check.full attribute on root span")
			}
			if !foundDryRun {
				t.Error("expected check.dry_run attribute on root span")
			}
		}
	}
}

func TestRunCheck_Phase1Span(t *testing.T) {
	// given: a test tracer and an Amadeus instance with a real git repo
	exp := setupTestTracer(t)
	repo := setupTestRepo(t)
	a := newTestAmadeus(t, repo.dir)

	// when: RunCheck is called (full mode, DryRun)
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})
	if err != nil {
		t.Fatalf("RunCheck failed: %v", err)
	}

	// then: a "reading_steiner" child span should exist
	spans := exp.GetSpans()
	if !containsSpan(spans, "reading_steiner") {
		t.Errorf("expected child span 'reading_steiner', got spans: %v", spanNames(spans))
	}

	// then: reading_steiner should be a child of amadeus.check
	var rootSpanID, phase1ParentSpanID string
	for _, s := range spans {
		if s.Name == "amadeus.check" {
			rootSpanID = s.SpanContext.SpanID().String()
		}
		if s.Name == "reading_steiner" {
			phase1ParentSpanID = s.Parent.SpanID().String()
		}
	}
	if rootSpanID == "" {
		t.Fatal("root span 'amadeus.check' not found")
	}
	if phase1ParentSpanID != rootSpanID {
		t.Errorf("expected reading_steiner parent=%s, got parent=%s", rootSpanID, phase1ParentSpanID)
	}
}

func TestRunCheck_Phase1ShiftDetectedEvent(t *testing.T) {
	// given: a test tracer and an Amadeus instance with a real git repo
	// Full mode always produces a significant shift report
	exp := setupTestTracer(t)
	repo := setupTestRepo(t)
	a := newTestAmadeus(t, repo.dir)

	// when: RunCheck is called in full mode (always significant)
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})
	if err != nil {
		t.Fatalf("RunCheck failed: %v", err)
	}

	// then: reading_steiner span should have a "shift.detected" event
	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "reading_steiner" {
			for _, event := range s.Events {
				if event.Name == "shift.detected" {
					found = true
					// Check that the event has shift.pr_count attribute
					hasPRCount := false
					for _, attr := range event.Attributes {
						if string(attr.Key) == "shift.pr_count" {
							hasPRCount = true
						}
					}
					if !hasPRCount {
						t.Error("expected shift.pr_count attribute on shift.detected event")
					}
				}
			}
		}
	}
	if !found {
		t.Error("expected 'shift.detected' event on reading_steiner span")
	}
}

func TestRunCheck_Phase2SpanExists(t *testing.T) {
	// given
	exp := setupTestTracer(t)
	repo := setupTestRepo(t)
	a := newTestAmadeus(t, repo.dir)

	// when: DryRun enters Phase 2 (prompt building) then returns early
	a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})

	// then: divergence_meter span should exist (created before DryRun return)
	spans := exp.GetSpans()
	if !containsSpan(spans, "divergence_meter") {
		t.Errorf("expected 'divergence_meter' span, got: %v", spanNames(spans))
	}

	// then: divergence_meter should be child of amadeus.check
	var rootSpanID, phase2ParentSpanID string
	for _, s := range spans {
		if s.Name == "amadeus.check" {
			rootSpanID = s.SpanContext.SpanID().String()
		}
		if s.Name == "divergence_meter" {
			phase2ParentSpanID = s.Parent.SpanID().String()
		}
	}
	if rootSpanID != "" && phase2ParentSpanID != rootSpanID {
		t.Errorf("expected divergence_meter parent=%s, got parent=%s", rootSpanID, phase2ParentSpanID)
	}
}
