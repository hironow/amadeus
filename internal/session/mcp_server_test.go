package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

func TestMCPServer_ListsAllPhase2bTools(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: all 4 Phase 2b tools advertised, with stable names
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools list missing: %v", result["tools"])
	}
	want := map[string]bool{
		"amadeus.ping":          false,
		"amadeus.next_review":   false,
		"amadeus.post_comment":  false,
		"amadeus.get_pr_status": false,
	}
	for _, t0 := range tools {
		entry, _ := t0.(map[string]any)
		if name, _ := entry["name"].(string); name != "" {
			want[name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing Phase 2b tool: %s", name)
		}
	}
}

func TestMCPServer_CallsPingTool(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"amadeus.ping","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content list mismatch: %v", result["content"])
	}
	first, _ := content[0].(map[string]any)
	if first["text"] != "pong" {
		t.Errorf("text = %v, want pong", first["text"])
	}
}

func TestMCPServer_RejectsUnknownTool(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"amadeus.does_not_exist","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	rpcErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestMCPServer_NextReview_UninitializedGateDir(t *testing.T) {
	// given: NewMCPServer without WithGateDir → uninitialized.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"amadeus.next_review","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false (empty gateDir)", body["initialized"])
	}
}

func TestMCPServer_NextReview_RealImpl_EmptyEventStore(t *testing.T) {
	// given: temp gateDir with no events → check_count=0.
	gateDir := t.TempDir()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"amadeus.next_review","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithGateDir(gateDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true", body["initialized"])
	}
	if got, _ := body["check_count"].(float64); int(got) != 0 {
		t.Errorf("check_count = %v, want 0", body["check_count"])
	}
}

func TestMCPServer_PostComment_RealImpl_PreviewOnly(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"amadeus.post_comment","arguments":{"pr_number":42,"body":"looks good to me"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: preview-only response with input echoed back.
	body := decodeFirstText(t, &out)
	if got, _ := body["pr_number"].(float64); int(got) != 42 {
		t.Errorf("pr_number = %v, want 42", body["pr_number"])
	}
	if got, _ := body["body_length"].(float64); int(got) != len("looks good to me") {
		t.Errorf("body_length = %v, want %d", body["body_length"], len("looks good to me"))
	}
	if got, _ := body["posted"].(bool); got {
		t.Errorf("posted = true, want false (preview-only must not post)")
	}
	if body["persistence"] != "preview-only" {
		t.Errorf("persistence = %v, want preview-only", body["persistence"])
	}
}

// fakeCommentPoster captures PostComment invocations for white-box
// testing of the MCP wiring without spawning the gh CLI.
type fakeCommentPoster struct {
	calls     []fakeCommentCall
	returnErr error
}

type fakeCommentCall struct {
	pr   string
	body string
}

func (f *fakeCommentPoster) PostComment(_ context.Context, prNumber, body string) error {
	f.calls = append(f.calls, fakeCommentCall{pr: prNumber, body: body})
	return f.returnErr
}

func TestMCPServer_PostComment_Phase4_PostsViaWriter(t *testing.T) {
	// given: MCP server wired with a fake comment poster (= Phase 4 #3
	// GitHub adapter contract).
	poster := &fakeCommentPoster{}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"amadeus.post_comment","arguments":{"pr_number":42,"body":"LGTM"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithCommentPoster(poster)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: writer was invoked + response reports posted=true with
	// persistence='github-comments-api'.
	if len(poster.calls) != 1 {
		t.Fatalf("PostComment calls = %d, want 1: %#v", len(poster.calls), poster.calls)
	}
	if poster.calls[0].pr != "42" {
		t.Errorf("pr = %q, want %q", poster.calls[0].pr, "42")
	}
	if poster.calls[0].body != "LGTM" {
		t.Errorf("body = %q, want %q", poster.calls[0].body, "LGTM")
	}
	body := decodeFirstText(t, &out)
	if got, _ := body["posted"].(bool); !got {
		t.Errorf("posted = false, want true (writer wired): %v", body)
	}
	if body["persistence"] != "github-comments-api" {
		t.Errorf("persistence = %v, want github-comments-api", body["persistence"])
	}
}

func TestMCPServer_PostComment_Phase4_SurfacesWriterError(t *testing.T) {
	// given: writer fails on post
	poster := &fakeCommentPoster{returnErr: errors.New("gh: rate limit exceeded")}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"amadeus.post_comment","arguments":{"pr_number":99,"body":"hi"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithCommentPoster(poster)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: posted=false + reason carries the writer error so the LLM
	// session can decide whether to retry.
	body := decodeFirstText(t, &out)
	if got, _ := body["posted"].(bool); got {
		t.Errorf("posted = true, want false (writer failed): %v", body)
	}
	if reason, _ := body["reason"].(string); !strings.Contains(reason, "rate limit") {
		t.Errorf("reason = %v, want it to surface 'rate limit'", body["reason"])
	}
}

func TestMCPServer_PostComment_RealImpl_RejectsMissingFields(t *testing.T) {
	// given: missing body
	in := strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"amadeus.post_comment","arguments":{"pr_number":42}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["persisted"] != false {
		t.Errorf("persisted = %v, want false", body["persisted"])
	}
	if _, ok := body["reason"]; !ok {
		t.Errorf("reason missing: %v", body)
	}
}

func TestMCPServer_GetPRStatus_UninitializedGateDir(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"amadeus.get_pr_status","arguments":{"pr_number":99}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false (empty gateDir)", body["initialized"])
	}
}

func TestMCPServer_GetPRStatus_RealImpl_NotFoundForPR(t *testing.T) {
	// given: temp gateDir + no events for PR 99 → found:false
	gateDir := t.TempDir()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"amadeus.get_pr_status","arguments":{"pr_number":99}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithGateDir(gateDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true", body["initialized"])
	}
	if body["found"] != false {
		t.Errorf("found = %v, want false", body["found"])
	}
}

// decodeFirstText extracts the JSON payload from the first content
// item of the MCP tools/call response. Stub responses ship a single
// JSON-string text entry so the body is a JSON object inside a string.
func decodeFirstText(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode inner JSON: %v (raw=%q)", err, text)
	}
	return body
}

func TestMCPServer_RejectsUnknownMethod(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"completion/complete"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	rpcErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestMCPServer_Initialize_Handshake(t *testing.T) {
	// given: client sends initialize with a different protocol version
	in := strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude-code","version":"1.0"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: server returns ITS supported version (not an echo), + tools cap + serverInfo
	var resp struct {
		Result struct {
			ProtocolVersion string                     `json:"protocolVersion"`
			Capabilities    map[string]json.RawMessage `json:"capabilities"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode initialize response: %v (raw=%q)", err, out.String())
	}
	if resp.Result.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion = %q, want 2024-11-05 (server supported, not echo of client 2025-06-18)", resp.Result.ProtocolVersion)
	}
	if _, ok := resp.Result.Capabilities["tools"]; !ok {
		t.Errorf("capabilities.tools missing: %v", resp.Result.Capabilities)
	}
	if resp.Result.ServerInfo.Name != "amadeus" {
		t.Errorf("serverInfo.name = %q, want amadeus", resp.Result.ServerInfo.Name)
	}
}

func TestMCPServer_NotificationsInitialized_NoResponse(t *testing.T) {
	// given: a JSON-RPC notification (no id)
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: notifications must not produce a response
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("notification must produce no response, got: %q", out.String())
	}
}
