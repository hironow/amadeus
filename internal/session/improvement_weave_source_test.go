package session

// white-box-reason: session internals: tests Weave HTTP source request shaping and env-based collector wiring

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestHTTPImprovementFeedbackSourceQueryFeedback(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{
					"id":            "fb-1",
					"project_id":    "proj-1",
					"weave_ref":     "call-1",
					"feedback_type": "comment",
					"created_at":    "2026-04-05T12:00:00Z",
					"payload": map[string]any{
						"failure_type": "execution_failure",
						"severity":     "high",
					},
				},
			},
		})
	}))
	defer srv.Close()

	source, err := NewHTTPImprovementFeedbackSource(srv.URL, "key-123")
	if err != nil {
		t.Fatalf("NewHTTPImprovementFeedbackSource: %v", err)
	}
	rows, err := source.QueryFeedback(context.Background(), ImprovementFeedbackQuery{
		ProjectID: "proj-1",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("QueryFeedback: %v", err)
	}
	if gotAuth != "Basic "+base64.StdEncoding.EncodeToString([]byte("api:key-123")) {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody["project_id"] != "proj-1" {
		t.Fatalf("project_id = %v, want proj-1", gotBody["project_id"])
	}
	if len(rows) != 1 || rows[0].ID != "fb-1" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestNewImprovementCollectorFromEnv(t *testing.T) {
	base := t.TempDir()
	insightsDir := filepath.Join(base, "insights")
	runDir := filepath.Join(base, ".run")
	if err := os.MkdirAll(insightsDir, 0o755); err != nil {
		t.Fatalf("mkdir insights: %v", err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	t.Setenv("WANDB_API_KEY", "key-123")
	t.Setenv("WANDB_ENTITY", "team-a")
	t.Setenv("WANDB_PROJECT", "proj-a")
	t.Setenv("WEAVE_API_URL", "https://trace.wandb.ai")

	collector, closeFn, err := NewImprovementCollectorFromEnv(base, NewInsightWriter(insightsDir, runDir), &domain.NopLogger{})
	if err != nil {
		t.Fatalf("NewImprovementCollectorFromEnv: %v", err)
	}
	defer closeFn()
	if collector == nil {
		t.Fatal("collector = nil, want non-nil")
	}
	if collector.ProjectID != "team-a/proj-a" {
		t.Fatalf("ProjectID = %q, want team-a/proj-a", collector.ProjectID)
	}
}

func TestNewImprovementCollector_ConfigOverridesEnv(t *testing.T) {
	base := t.TempDir()
	insightsDir := filepath.Join(base, "insights")
	runDir := filepath.Join(base, ".run")
	if err := os.MkdirAll(insightsDir, 0o755); err != nil {
		t.Fatalf("mkdir insights: %v", err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	t.Setenv("WANDB_API_KEY", "key-123")
	t.Setenv("WANDB_ENTITY", "team-a")
	t.Setenv("WANDB_PROJECT", "proj-a")
	t.Setenv("WEAVE_API_URL", "https://trace.wandb.ai")

	enabled := true
	collector, closeFn, err := NewImprovementCollector(base, domain.ImprovementCollectorConfig{
		Enabled:       &enabled,
		ProjectID:     "team-b/proj-b",
		APIURL:        "https://weave.example.test/base/",
		QueryLimit:    25,
		FeedbackTypes: []string{"comment", "ci-outcome"},
	}, NewInsightWriter(insightsDir, runDir), &domain.NopLogger{})
	if err != nil {
		t.Fatalf("NewImprovementCollector: %v", err)
	}
	defer closeFn()
	if collector == nil {
		t.Fatal("collector = nil, want non-nil")
	}
	if collector.ProjectID != "team-b/proj-b" {
		t.Fatalf("ProjectID = %q, want team-b/proj-b", collector.ProjectID)
	}
	if collector.QueryLimit != 25 {
		t.Fatalf("QueryLimit = %d, want 25", collector.QueryLimit)
	}
	if len(collector.AllowedFeedbackTypes) != 2 {
		t.Fatalf("AllowedFeedbackTypes = %v, want 2 entries", collector.AllowedFeedbackTypes)
	}
	source, ok := collector.Source.(*HTTPImprovementFeedbackSource)
	if !ok {
		t.Fatalf("Source = %T, want *HTTPImprovementFeedbackSource", collector.Source)
	}
	if source.BaseURL != "https://weave.example.test/base" {
		t.Fatalf("BaseURL = %q, want trimmed override", source.BaseURL)
	}
}

func TestNewImprovementCollector_DisabledExplicitly(t *testing.T) {
	base := t.TempDir()
	insightsDir := filepath.Join(base, "insights")
	runDir := filepath.Join(base, ".run")
	if err := os.MkdirAll(insightsDir, 0o755); err != nil {
		t.Fatalf("mkdir insights: %v", err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	t.Setenv("WANDB_API_KEY", "key-123")
	t.Setenv("WANDB_ENTITY", "team-a")
	t.Setenv("WANDB_PROJECT", "proj-a")

	enabled := false
	collector, closeFn, err := NewImprovementCollector(base, domain.ImprovementCollectorConfig{
		Enabled: &enabled,
	}, NewInsightWriter(insightsDir, runDir), &domain.NopLogger{})
	if err != nil {
		t.Fatalf("NewImprovementCollector: %v", err)
	}
	defer closeFn()
	if collector != nil {
		t.Fatalf("collector = %#v, want nil", collector)
	}
}

func TestHTTPImprovementFeedbackSourceFiltersFeedbackTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{
					"id":            "fb-1",
					"project_id":    "proj-1",
					"weave_ref":     "call-1",
					"feedback_type": "comment",
					"created_at":    "2026-04-05T12:00:00Z",
					"payload":       map[string]any{},
				},
				{
					"id":            "fb-2",
					"project_id":    "proj-1",
					"weave_ref":     "call-2",
					"feedback_type": "ci-outcome",
					"created_at":    "2026-04-05T12:01:00Z",
					"payload":       map[string]any{},
				},
			},
		})
	}))
	defer srv.Close()

	source, err := NewHTTPImprovementFeedbackSource(srv.URL, "key-123")
	if err != nil {
		t.Fatalf("NewHTTPImprovementFeedbackSource: %v", err)
	}
	rows, err := source.QueryFeedback(context.Background(), ImprovementFeedbackQuery{
		ProjectID:     "proj-1",
		FeedbackTypes: []string{"ci-outcome"},
	})
	if err != nil {
		t.Fatalf("QueryFeedback: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "fb-2" {
		t.Fatalf("rows = %#v, want only fb-2", rows)
	}
}

func TestHTTPImprovementFeedbackSourceFiltersCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{
					"id":            "fb-1",
					"project_id":    "proj-1",
					"weave_ref":     "call-1",
					"feedback_type": "comment",
					"created_at":    "2026-04-05T12:00:00Z",
					"payload":       map[string]any{},
				},
				{
					"id":            "fb-2",
					"project_id":    "proj-1",
					"weave_ref":     "call-2",
					"feedback_type": "comment",
					"created_at":    "2026-04-05T12:00:00Z",
					"payload":       map[string]any{},
				},
			},
		})
	}))
	defer srv.Close()

	source, err := NewHTTPImprovementFeedbackSource(srv.URL, "key-123")
	if err != nil {
		t.Fatalf("NewHTTPImprovementFeedbackSource: %v", err)
	}
	rows, err := source.QueryFeedback(context.Background(), ImprovementFeedbackQuery{
		ProjectID:     "proj-1",
		CreatedAfter:  time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		AfterFeedback: "fb-1",
	})
	if err != nil {
		t.Fatalf("QueryFeedback: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "fb-2" {
		t.Fatalf("rows = %#v, want only fb-2", rows)
	}
}
