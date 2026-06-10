package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// refs issue 0032 (amadeus reviewer write path, decision D2(a)):
// refresh_reviews ingests open PRs into the gate event store via a
// narrow lister port, post_comment records EventReviewPosted, and
// next_review serves the oldest un-reviewed PR from the latest
// snapshot (intake contract), falling back to the legacy
// check.completed read model when no snapshot exists.

type fakePRLister struct {
	prs []domain.PRState
	err error
}

func (f *fakePRLister) ListOpenPRs(_ context.Context, _ string) ([]domain.PRState, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.prs, nil
}

type recordingReviewEmitter struct {
	snapshots [][]domain.PRSnapshotEntry
	posted    []string
	failWith  error
}

func (r *recordingReviewEmitter) EmitPRSnapshotIngested(prs []domain.PRSnapshotEntry, _ time.Time) error {
	if r.failWith != nil {
		return r.failWith
	}
	r.snapshots = append(r.snapshots, prs)
	return nil
}

func (r *recordingReviewEmitter) EmitReviewPosted(prNumber string, _ time.Time) error {
	if r.failWith != nil {
		return r.failWith
	}
	r.posted = append(r.posted, prNumber)
	return nil
}

func mustPRState(t *testing.T, number, title string) domain.PRState {
	t.Helper()
	pr, err := domain.NewPRState(number, title, "main", "feat/"+number, true, 0, nil, nil, "sha-"+number)
	if err != nil {
		t.Fatalf("NewPRState: %v", err)
	}
	return pr
}

func decodeReviewToolJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, string(raw))
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content := result["content"].([]any)
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode tool body: %v (text=%q)", err, text)
	}
	return body
}

func seedEvent(t *testing.T, gateDir string, eventType domain.EventType, data any) {
	t.Helper()
	ev, err := domain.NewEvent(eventType, data, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewEvent(%s): %v", eventType, err)
	}
	store := session.NewEventStore(gateDir, nil)
	if _, err := store.Append(context.Background(), ev); err != nil {
		t.Fatalf("seed append: %v", err)
	}
}

func TestMCPServer_ToolsList_IncludesRefreshReviews(t *testing.T) {
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
		if entry["name"] == "refresh_reviews" {
			found = true
		}
	}
	if !found {
		t.Error("missing refresh_reviews in tools/list")
	}
}

func TestMCPServer_RefreshReviews_IngestsSnapshot(t *testing.T) {
	// given
	gateDir := t.TempDir()
	lister := &fakePRLister{prs: []domain.PRState{
		mustPRState(t, "101", "first"),
		mustPRState(t, "102", "second"),
	}}
	emitter := &recordingReviewEmitter{}
	req := `{"jsonrpc":"2.0","id":30,"method":"tools/call","params":{"name":"refresh_reviews","arguments":{}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).
		WithGateDir(gateDir).WithPRLister(lister).WithReviewEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["ingested"] != true || body["persistence"] != "event-store" {
		t.Fatalf("refresh response mismatch: %v", body)
	}
	if int(body["pr_count"].(float64)) != 2 {
		t.Errorf("pr_count = %v, want 2", body["pr_count"])
	}
	if len(emitter.snapshots) != 1 || len(emitter.snapshots[0]) != 2 {
		t.Fatalf("snapshot emission mismatch: %+v", emitter.snapshots)
	}
	if emitter.snapshots[0][0].Number != "101" {
		t.Errorf("snapshot entry mismatch: %+v", emitter.snapshots[0])
	}
}

func TestMCPServer_RefreshReviews_PreviewOnlyWithoutLister(t *testing.T) {
	// given
	gateDir := t.TempDir()
	req := `{"jsonrpc":"2.0","id":31,"method":"tools/call","params":{"name":"refresh_reviews","arguments":{}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["ingested"] != false || body["persistence"] != "preview-only" {
		t.Errorf("want preview-only without lister, got: %v", body)
	}
}

func TestMCPServer_NextReview_ServesOldestUnreviewedFromSnapshot(t *testing.T) {
	// given: snapshot with 2 PRs, one already reviewed
	gateDir := t.TempDir()
	seedEvent(t, gateDir, domain.EventPRSnapshotIngested, domain.PRSnapshotIngestedData{
		IngestedAt: time.Now().UTC(),
		PRs: []domain.PRSnapshotEntry{
			{Number: "101", Title: "first", BaseBranch: "main", HeadBranch: "f1"},
			{Number: "102", Title: "second", BaseBranch: "main", HeadBranch: "f2"},
		},
	})
	seedEvent(t, gateDir, domain.EventReviewPosted, domain.ReviewPostedData{
		PRNumber: "101", PostedAt: time.Now().UTC(),
	})
	req := `{"jsonrpc":"2.0","id":32,"method":"tools/call","params":{"name":"next_review","arguments":{}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: 102 is the next intake item; 101 is excluded as posted
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["source"] != "pr-snapshot" {
		t.Fatalf("source = %v, want pr-snapshot (body=%v)", body["source"], body)
	}
	next, _ := body["next_pr"].(map[string]any)
	if next == nil || next["number"] != "102" {
		t.Errorf("next_pr = %v, want PR 102", body["next_pr"])
	}
	if int(body["pending_count"].(float64)) != 1 {
		t.Errorf("pending_count = %v, want 1", body["pending_count"])
	}
}

func TestMCPServer_NextReview_NonePendingWhenAllReviewed(t *testing.T) {
	// given
	gateDir := t.TempDir()
	seedEvent(t, gateDir, domain.EventPRSnapshotIngested, domain.PRSnapshotIngestedData{
		IngestedAt: time.Now().UTC(),
		PRs:        []domain.PRSnapshotEntry{{Number: "101", Title: "only", BaseBranch: "main", HeadBranch: "f1"}},
	})
	seedEvent(t, gateDir, domain.EventReviewPosted, domain.ReviewPostedData{PRNumber: "101", PostedAt: time.Now().UTC()})
	req := `{"jsonrpc":"2.0","id":33,"method":"tools/call","params":{"name":"next_review","arguments":{}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["none_pending"] != true {
		t.Errorf("none_pending = %v, want true (body=%v)", body["none_pending"], body)
	}
}

func TestMCPServer_PostComment_RecordsReviewPostedEvent(t *testing.T) {
	// given
	gateDir := t.TempDir()
	poster := &fakeCommentPoster{}
	emitter := &recordingReviewEmitter{}
	req := `{"jsonrpc":"2.0","id":34,"method":"tools/call","params":{"name":"post_comment","arguments":{"pr_number":77,"body":"axis findings"}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).
		WithGateDir(gateDir).WithCommentPoster(poster).WithReviewEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: posted to GitHub AND recorded in the gate ledger
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["posted"] != true {
		t.Fatalf("posted = %v, want true (body=%v)", body["posted"], body)
	}
	if len(emitter.posted) != 1 || emitter.posted[0] != "77" {
		t.Errorf("EmitReviewPosted calls = %v, want [77]", emitter.posted)
	}
}
