package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestWriteDivergenceInsight_CreatesFile(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	result := domain.DivergenceResult{
		Value:    0.45,
		Internal: 45.0,
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR:        {Score: 70, Details: "missing ADR for new pattern"},
			domain.AxisDoD:        {Score: 30, Details: "mostly complete"},
			domain.AxisDependency: {Score: 20, Details: "deps aligned"},
			domain.AxisImplicit:   {Score: 10, Details: "ok"},
		},
		Severity: domain.SeverityMedium,
	}

	reasoning := "ADR coverage gap detected in new pattern introduction"

	// when
	session.ExportWriteDivergenceInsight(a, result, "abc123", "def456..abc123", reasoning)

	// then
	data, err := os.ReadFile(filepath.Join(insightsDir, "divergence.md"))
	if err != nil {
		t.Fatalf("expected divergence.md to exist: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}

	entry := file.Entries[0]
	if entry.Title != "divergence-abc123" {
		t.Errorf("title: got %q", entry.Title)
	}
	if !strings.Contains(entry.What, "0.450000") {
		t.Errorf("what should contain score: got %q", entry.What)
	}
	if !strings.Contains(entry.What, "medium") {
		t.Errorf("what should contain severity: got %q", entry.What)
	}
	if !strings.Contains(entry.Why, "adr_integrity=70") {
		t.Errorf("why should contain high-scoring axis: got %q", entry.Why)
	}
	if entry.How != reasoning {
		t.Errorf("how should contain reasoning: got %q, want %q", entry.How, reasoning)
	}
	if !strings.Contains(entry.When, "def456..abc123") {
		t.Errorf("when should contain commit range: got %q", entry.When)
	}
	if !strings.Contains(entry.Who, "session-abc123") {
		t.Errorf("who should contain session ID: got %q", entry.Who)
	}
	if entry.Extra["axis-scores"] == "" {
		t.Error("extra should have axis-scores")
	}
	if file.Kind != "divergence" {
		t.Errorf("kind: got %q", file.Kind)
	}
	if file.Tool != "amadeus" {
		t.Errorf("tool: got %q", file.Tool)
	}
}

func TestWriteImprovementOutcomeInsight_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)
	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}
	inbox := []domain.DMail{{
		Name: "pt-report-1",
		Kind: domain.KindReport,
		Metadata: domain.CorrectionMetadata{
			FailureType:      domain.FailureTypeExecutionFailure,
			CorrelationID:    "corr-1",
			TraceID:          "trace-1",
			CorrectiveAction: "retry",
		}.Apply(nil),
	}}

	session.ExportWriteImprovementOutcomeInsight(a, inbox, "abc123", 0)

	data, err := os.ReadFile(filepath.Join(insightsDir, "improvement-loop.md"))
	if err != nil {
		t.Fatalf("expected improvement-loop.md to exist: %v", err)
	}
	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	entry := file.Entries[0]
	if entry.Title != "improvement-corr-1-resolved" {
		t.Fatalf("title = %q", entry.Title)
	}
	if entry.Extra["outcome"] != "resolved" {
		t.Fatalf("outcome = %q, want resolved", entry.Extra["outcome"])
	}
	if entry.Extra["correlation-id"] != "corr-1" {
		t.Fatalf("correlation-id = %q, want corr-1", entry.Extra["correlation-id"])
	}
}

func TestWriteDivergenceInsight_NilInsightsSkips(t *testing.T) {
	// given: Amadeus with nil Insights
	a := &session.Amadeus{
		Logger: &domain.NopLogger{},
	}

	result := domain.DivergenceResult{
		Value:    0.5,
		Severity: domain.SeverityHigh,
		Axes:     map[domain.Axis]domain.AxisScore{},
	}

	// when/then: should not panic
	session.ExportWriteDivergenceInsight(a, result, "abc", "x..y", "some reasoning")
}

func TestWriteConvergenceInsight_HighSeverity(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	alert := domain.ConvergenceAlert{
		Target:   "internal/domain/scoring.go",
		Count:    5,
		Window:   7,
		DMails:   []string{"dmail-001", "dmail-002", "dmail-003"},
		Severity: domain.SeverityHigh,
	}

	// when
	session.ExportWriteConvergenceInsight(a, alert, "def456")

	// then
	data, err := os.ReadFile(filepath.Join(insightsDir, "convergence.md"))
	if err != nil {
		t.Fatalf("expected convergence.md to exist: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}

	entry := file.Entries[0]
	if entry.Title != "convergence-internal/domain/scoring.go" {
		t.Errorf("title: got %q", entry.Title)
	}
	if !strings.Contains(entry.What, "5 D-Mails in 7 days") {
		t.Errorf("what: got %q", entry.What)
	}
	// With empty archive dir, falls back to default why
	if !strings.Contains(entry.Why, "structural issue") {
		t.Errorf("why: got %q", entry.Why)
	}
	if !strings.Contains(entry.Extra["related-dmails"], "dmail-001") {
		t.Errorf("extra related-dmails: got %q", entry.Extra["related-dmails"])
	}
	if file.Kind != "convergence" {
		t.Errorf("kind: got %q", file.Kind)
	}
}

func TestWriteConvergenceInsight_MediumSeveritySkipped(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	alert := domain.ConvergenceAlert{
		Target:   "some-target",
		Count:    3,
		Window:   7,
		DMails:   []string{"dmail-001"},
		Severity: domain.SeverityMedium, // not HIGH
	}

	// when
	session.ExportWriteConvergenceInsight(a, alert, "abc123")

	// then: no file should be created
	_, err := os.Stat(filepath.Join(insightsDir, "convergence.md"))
	if err == nil {
		t.Error("convergence.md should not exist for medium severity")
	}
}

func TestWriteConvergenceInsight_IncludesDMailDescriptions(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	alert := domain.ConvergenceAlert{
		Target: "internal/domain/scoring.go",
		Count:  3,
		Window: 7,
		DMails: []string{"dmail-001", "dmail-002"},
		Descriptions: map[string]string{
			"dmail-001": "ADR-003 violation in auth module",
			"dmail-002": "Dependency drift in logging subsystem",
		},
		Severity: domain.SeverityHigh,
	}

	// when
	session.ExportWriteConvergenceInsight(a, alert, "xyz789")

	// then
	data, err := os.ReadFile(filepath.Join(insightsDir, "convergence.md"))
	if err != nil {
		t.Fatalf("expected convergence.md to exist: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}

	entry := file.Entries[0]
	if !strings.Contains(entry.Why, "ADR-003 violation in auth module") {
		t.Errorf("why should contain first dmail description: got %q", entry.Why)
	}
	if !strings.Contains(entry.Why, "Dependency drift in logging subsystem") {
		t.Errorf("why should contain second dmail description: got %q", entry.Why)
	}
	if !strings.Contains(entry.Why, "Converging feedback") {
		t.Errorf("why should start with 'Converging feedback': got %q", entry.Why)
	}
}

func TestWriteConvergenceInsight_NoDescriptionsFallsBackToDefault(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	alert := domain.ConvergenceAlert{
		Target:   "internal/domain/scoring.go",
		Count:    3,
		Window:   7,
		DMails:   []string{"dmail-without-desc"},
		Severity: domain.SeverityHigh,
	}

	// when
	session.ExportWriteConvergenceInsight(a, alert, "abc123")

	// then
	data, err := os.ReadFile(filepath.Join(insightsDir, "convergence.md"))
	if err != nil {
		t.Fatalf("expected convergence.md to exist: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}

	entry := file.Entries[0]
	if !strings.Contains(entry.Why, "structural issue") {
		t.Errorf("why should fall back to default message: got %q", entry.Why)
	}
}

func TestWriteDivergenceInsight_Idempotent(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	result := domain.DivergenceResult{
		Value:    0.3,
		Severity: domain.SeverityLow,
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR: {Score: 30, Details: "ok"},
		},
	}

	// when: write twice with same session ID (same title)
	session.ExportWriteDivergenceInsight(a, result, "abc123", "x..y", "")
	session.ExportWriteDivergenceInsight(a, result, "abc123", "x..y", "")

	// then: should have exactly 1 entry (idempotent)
	data, _ := os.ReadFile(filepath.Join(insightsDir, "divergence.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.Entries) != 1 {
		t.Errorf("expected 1 entry (idempotent), got %d", len(file.Entries))
	}
}

func TestWriteDivergenceInsight_EmptyReasoningFallback(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	writer := session.NewInsightWriter(insightsDir, runDir)

	a := &session.Amadeus{
		Logger:   &domain.NopLogger{},
		Insights: writer,
	}

	result := domain.DivergenceResult{
		Value:    0.3,
		Severity: domain.SeverityLow,
		Axes: map[domain.Axis]domain.AxisScore{
			domain.AxisADR: {Score: 30, Details: "ok"},
		},
	}

	// when: empty reasoning should fall back to default
	session.ExportWriteDivergenceInsight(a, result, "abc123", "x..y", "")

	// then
	data, err := os.ReadFile(filepath.Join(insightsDir, "divergence.md"))
	if err != nil {
		t.Fatalf("expected divergence.md to exist: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}

	entry := file.Entries[0]
	if entry.How != "Focus remediation on highest-scoring axis" {
		t.Errorf("how should fall back to default: got %q", entry.How)
	}
}

func TestHighScoringAxisDetails_ThresholdAt50(t *testing.T) {
	// given
	axes := map[domain.Axis]domain.AxisScore{
		domain.AxisADR:        {Score: 50, Details: "exactly at threshold"},
		domain.AxisDoD:        {Score: 49, Details: "just below"},
		domain.AxisDependency: {Score: 80, Details: "way above"},
		domain.AxisImplicit:   {Score: 10, Details: "low"},
	}

	// when
	parts := session.ExportHighScoringAxisDetails(axes)

	// then: should include score >= 50 only
	if len(parts) != 2 {
		t.Fatalf("expected 2 high-scoring axes, got %d: %v", len(parts), parts)
	}
	if !strings.Contains(parts[0], "adr_integrity=50") {
		t.Errorf("first part: got %q", parts[0])
	}
	if !strings.Contains(parts[1], "dependency_integrity=80") {
		t.Errorf("second part: got %q", parts[1])
	}
}
