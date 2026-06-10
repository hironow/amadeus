package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// refs issue 0034 (P4): get_insights exposes the verifier's learning
// loop — persisted insight-ledger files + a live summary derived from
// the gate event store (reviews posted / latest snapshot / latest
// check divergence).

func callAmInsights(t *testing.T, gateDir, args string) map[string]any {
	t.Helper()
	req := `{"jsonrpc":"2.0","id":80,"method":"tools/call","params":{"name":"get_insights","arguments":` + args + `}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir)
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	return decodeReviewToolJSON(t, out.Bytes())
}

func TestMCPServer_ToolsList_IncludesGetInsights(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tools := resp["result"].(map[string]any)["tools"].([]any)
	found := false
	for _, t0 := range tools {
		entry, _ := t0.(map[string]any)
		if entry["name"] == "get_insights" {
			found = true
		}
	}
	if !found {
		t.Error("missing get_insights in tools/list")
	}
}

func TestMCPServer_GetInsights_EmptyStateIsNotAnError(t *testing.T) {
	// given
	gateDir := t.TempDir()

	// when
	body := callAmInsights(t, gateDir, `{}`)

	// then
	if body["initialized"] != true {
		t.Fatalf("initialized = %v, want true (body=%v)", body["initialized"], body)
	}
	insights, _ := body["insights"].([]any)
	if len(insights) != 0 {
		t.Errorf("insights = %v, want empty array", body["insights"])
	}
}

func TestMCPServer_GetInsights_ReadsPersistedInsightFiles(t *testing.T) {
	// given
	gateDir := t.TempDir()
	insightsDir := filepath.Join(gateDir, "insights")
	runDir := filepath.Join(gateDir, ".run")
	for _, d := range []string{insightsDir, runDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	w := session.NewInsightWriter(insightsDir, runDir)
	if err := w.Append("improvement-loop.md", "improvement-loop", "amadeus", domain.InsightEntry{
		Title: "correction-1", What: "w", Why: "y", How: "h", When: "always", Who: "amadeus", Constraints: "none",
	}); err != nil {
		t.Fatalf("seed insight: %v", err)
	}

	// when
	body := callAmInsights(t, gateDir, `{"kind":"improvement"}`)

	// then
	insights, _ := body["insights"].([]any)
	if len(insights) != 1 {
		t.Fatalf("insights = %v, want 1 (body=%v)", body["insights"], body)
	}
	file, _ := insights[0].(map[string]any)
	entries, _ := file["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %v, want 1", file["entries"])
	}
}

func TestMCPServer_GetInsights_LiveReviewSummaryFromEvents(t *testing.T) {
	// given: snapshot + one posted review in the gate ledger
	gateDir := t.TempDir()
	seedEvent(t, gateDir, domain.EventPRSnapshotIngested, domain.PRSnapshotIngestedData{
		IngestedAt: time.Now().UTC(),
		PRs: []domain.PRSnapshotEntry{
			{Number: "101", Title: "a", BaseBranch: "main", HeadBranch: "f1"},
			{Number: "102", Title: "b", BaseBranch: "main", HeadBranch: "f2"},
		},
	})
	seedEvent(t, gateDir, domain.EventReviewPosted, domain.ReviewPostedData{PRNumber: "101", PostedAt: time.Now().UTC()})

	// when
	body := callAmInsights(t, gateDir, `{}`)

	// then
	live, _ := body["live_review"].(map[string]any)
	if live == nil {
		t.Fatalf("live_review missing: %v", body)
	}
	if int(live["reviews_posted"].(float64)) != 1 {
		t.Errorf("reviews_posted = %v, want 1", live["reviews_posted"])
	}
	if int(live["latest_snapshot_size"].(float64)) != 2 {
		t.Errorf("latest_snapshot_size = %v, want 2", live["latest_snapshot_size"])
	}
	if int(live["pending_reviews"].(float64)) != 1 {
		t.Errorf("pending_reviews = %v, want 1", live["pending_reviews"])
	}
}

func TestMCPServer_GetInsights_UninitializedWithoutGateDir(t *testing.T) {
	// given
	req := `{"jsonrpc":"2.0","id":81,"method":"tools/call","params":{"name":"get_insights","arguments":{}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false", body["initialized"])
	}
}
