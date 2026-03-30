package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func noopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("test")
}

// fakeNDJSON returns Claude-like stream-json lines for testing.
func fakeNDJSON() string {
	return strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"fake-sess-001","model":"opus","tools":["Read","Write","Bash"]}`,
		`{"type":"assistant","session_id":"fake-sess-001","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_01","name":"Read","input":{"file_path":"/src/main.go"}}]}}`,
		`{"type":"tool_result","session_id":"fake-sess-001","tool_use_id":"toolu_01"}`,
		`{"type":"assistant","session_id":"fake-sess-001","message":{"role":"assistant","content":[{"type":"text","text":"The file looks good."}]}}`,
		`{"type":"result","subtype":"success","session_id":"fake-sess-001","result":"Analysis complete.","usage":{"input_tokens":1000,"output_tokens":200},"total_cost_usd":0.01,"duration_ms":5000}`,
	}, "\n") + "\n"
}

// fakeNDJSONWithSubagent returns stream-json with a Task subagent.
func fakeNDJSONWithSubagent() string {
	return strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"fake-sess-002","model":"opus","tools":["Read","Task"]}`,
		`{"type":"assistant","session_id":"fake-sess-002","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_sub_01","name":"Task","input":{"description":"explore codebase"}}]}}`,
		`{"type":"tool_result","session_id":"fake-sess-002","tool_use_id":"toolu_sub_01"}`,
		`{"type":"result","subtype":"success","session_id":"fake-sess-002","result":"Done.","usage":{"input_tokens":500,"output_tokens":100},"duration_ms":3000}`,
	}, "\n") + "\n"
}

func TestSessionStream_NormalizerBusPipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// given: event bus + subscriber
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()
	sub := bus.Subscribe(64)
	defer sub.Close()

	// given: normalizer wired to bus
	normalizer := platform.NewStreamNormalizer("sightjack", domain.ProviderClaudeCode)
	normalizer.SetCodingSessionID("test-session-001")

	// given: stream reader with hook
	reader := platform.NewStreamReader(strings.NewReader(fakeNDJSON()))
	emitter := platform.NewSpanEmittingStreamReader(reader, ctx, noopTracer())
	emitter.SetStreamMessageHandler(func(msg *platform.StreamMessage, raw json.RawMessage) {
		if ev := normalizer.Normalize(msg, raw); ev != nil {
			bus.Publish(ctx, *ev)
		}
	})

	// when: collect all messages (triggers processMessage for each line)
	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}

	// then: drain events from subscriber
	var events []domain.SessionStreamEvent
	timeout := time.After(2 * time.Second)
drain:
	for {
		select {
		case ev := <-sub.C():
			events = append(events, ev)
		case <-timeout:
			break drain
		default:
			if len(events) > 0 {
				// Small delay to catch any remaining events.
				time.Sleep(10 * time.Millisecond)
				select {
				case ev := <-sub.C():
					events = append(events, ev)
				default:
					break drain
				}
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	if len(events) == 0 {
		t.Fatal("expected events, got none")
	}

	// Verify event types received.
	typeMap := make(map[domain.StreamEventType]int)
	for _, ev := range events {
		typeMap[ev.Type]++
		// All events should have our session ID.
		if ev.SessionID != "test-session-001" {
			t.Errorf("SessionID = %q, want %q", ev.SessionID, "test-session-001")
		}
		// All events should have provider session ID.
		if ev.ProviderSessionID != "fake-sess-001" {
			t.Errorf("ProviderSessionID = %q, want %q", ev.ProviderSessionID, "fake-sess-001")
		}
		// Schema version should be v1.
		if ev.SchemaVersion != domain.StreamSchemaVersion {
			t.Errorf("SchemaVersion = %d, want %d", ev.SchemaVersion, domain.StreamSchemaVersion)
		}
		// Tool should be set.
		if ev.Tool != "sightjack" {
			t.Errorf("Tool = %q, want %q", ev.Tool, "sightjack")
		}
	}

	// Should have: session_start, tool_use_start, tool_result, assistant_text, session_end
	for _, expected := range []domain.StreamEventType{
		domain.StreamSessionStart,
		domain.StreamToolUseStart,
		domain.StreamToolResult,
		domain.StreamAssistantText,
		domain.StreamSessionEnd,
	} {
		if typeMap[expected] == 0 {
			t.Errorf("missing event type %q", expected)
		}
	}
}

func TestSessionStream_SubagentTracking(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bus := platform.NewInProcessSessionBus()
	defer bus.Close()
	sub := bus.Subscribe(64)
	defer sub.Close()

	normalizer := platform.NewStreamNormalizer("sightjack", domain.ProviderClaudeCode)
	normalizer.SetCodingSessionID("test-session-sub")

	reader := platform.NewStreamReader(strings.NewReader(fakeNDJSONWithSubagent()))
	emitter := platform.NewSpanEmittingStreamReader(reader, ctx, noopTracer())
	emitter.SetStreamMessageHandler(func(msg *platform.StreamMessage, raw json.RawMessage) {
		if ev := normalizer.Normalize(msg, raw); ev != nil {
			bus.Publish(ctx, *ev)
		}
	})

	_, _, err := emitter.CollectAll()
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}

	var events []domain.SessionStreamEvent
	time.Sleep(50 * time.Millisecond)
	for {
		select {
		case ev := <-sub.C():
			events = append(events, ev)
		default:
			goto done
		}
	}
done:

	// Should have subagent_start and subagent_end.
	typeMap := make(map[domain.StreamEventType]int)
	for _, ev := range events {
		typeMap[ev.Type]++
	}

	if typeMap[domain.StreamSubagentStart] == 0 {
		t.Error("missing subagent_start event")
	}
	if typeMap[domain.StreamSubagentEnd] == 0 {
		t.Error("missing subagent_end event")
	}

	// Verify subagent_start has parent_session_id.
	for _, ev := range events {
		if ev.Type == domain.StreamSubagentStart {
			if ev.ParentSessionID != "test-session-sub" {
				t.Errorf("ParentSessionID = %q, want %q", ev.ParentSessionID, "test-session-sub")
			}
			if ev.SubagentID == "" {
				t.Error("SubagentID should be non-empty")
			}
		}
	}
}

// fakeDetailedRunner implements port.DetailedRunner with configurable output.
type fakeDetailedRunner struct {
	sessionID string
	text      string
	err       error
}

func (f *fakeDetailedRunner) RunDetailed(_ context.Context, _ string, _ io.Writer, _ ...port.RunOption) (port.RunResult, error) {
	return port.RunResult{Text: f.text, ProviderSessionID: f.sessionID}, f.err
}

func TestSessionStream_FullPipeline_SessionStore(t *testing.T) {
	t.Parallel()

	// given: session store.
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, domain.StateDir)
	os.MkdirAll(filepath.Join(stateDir, ".run"), 0o755)
	dbPath := filepath.Join(stateDir, ".run", "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	// given: mock runner + tracker.
	runner := &fakeDetailedRunner{sessionID: "claude-sess-full", text: "Analysis complete."}
	tracker := session.NewSessionTrackingAdapter(runner, store, domain.ProviderClaudeCode)

	// when: run session.
	ctx := context.Background()
	rec, text, runErr := tracker.RunSession(ctx, "analyze code", io.Discard,
		port.WithWorkDir(tmpDir),
	)

	// then: no error.
	if runErr != nil {
		t.Fatalf("RunSession: %v", runErr)
	}
	if text != "Analysis complete." {
		t.Errorf("text = %q, want %q", text, "Analysis complete.")
	}

	// Verify session record was persisted with correct fields.
	loaded, loadErr := store.Load(ctx, rec.ID)
	if loadErr != nil {
		t.Fatalf("Load session: %v", loadErr)
	}
	if loaded.Provider != domain.ProviderClaudeCode {
		t.Errorf("Provider = %q, want %q", loaded.Provider, domain.ProviderClaudeCode)
	}
	if loaded.Status != domain.SessionCompleted {
		t.Errorf("Status = %q, want %q", loaded.Status, domain.SessionCompleted)
	}
	if loaded.ProviderSessionID != "claude-sess-full" {
		t.Errorf("ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "claude-sess-full")
	}

	// Verify session is queryable.
	records, err := store.List(ctx, port.ListSessionOpts{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}

	// Verify findByProviderSessionID.
	found, err := store.FindByProviderSessionID(ctx, domain.ProviderClaudeCode, "claude-sess-full")
	if err != nil {
		t.Fatalf("FindByProviderSessionID: %v", err)
	}
	if len(found) != 1 {
		t.Errorf("expected 1 record, got %d", len(found))
	}
}

func TestSessionStream_FullPipeline_FailedSession(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, domain.StateDir)
	os.MkdirAll(filepath.Join(stateDir, ".run"), 0o755)
	dbPath := filepath.Join(stateDir, ".run", "sessions.db")
	store, err := session.NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	runner := &fakeDetailedRunner{sessionID: "claude-sess-fail", text: "partial", err: context.DeadlineExceeded}
	tracker := session.NewSessionTrackingAdapter(runner, store, domain.ProviderClaudeCode)

	ctx := context.Background()
	rec, _, runErr := tracker.RunSession(ctx, "timeout task", io.Discard)
	if runErr == nil {
		t.Fatal("expected error")
	}

	loaded, _ := store.Load(ctx, rec.ID)
	if loaded.Status != domain.SessionFailed {
		t.Errorf("Status = %q, want %q", loaded.Status, domain.SessionFailed)
	}
	if loaded.Metadata["failure_reason"] == "" {
		t.Error("expected failure_reason in metadata")
	}
	if loaded.ProviderSessionID != "claude-sess-fail" {
		t.Errorf("ProviderSessionID = %q, want %q", loaded.ProviderSessionID, "claude-sess-fail")
	}
}

func TestSessionStream_RawTruncation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bus := platform.NewInProcessSessionBus()
	defer bus.Close()
	sub := bus.Subscribe(64)
	defer sub.Close()

	normalizer := platform.NewStreamNormalizer("sightjack", domain.ProviderClaudeCode)

	// Build a message with very long text content.
	longText := strings.Repeat("x", 10000)
	ndjson := `{"type":"assistant","session_id":"sess","message":{"role":"assistant","content":[{"type":"text","text":"` + longText + `"}]}}` + "\n"
	ndjson += `{"type":"result","subtype":"success","session_id":"sess","result":"done"}` + "\n"

	reader := platform.NewStreamReader(strings.NewReader(ndjson))
	emitter := platform.NewSpanEmittingStreamReader(reader, ctx, noopTracer())
	emitter.SetStreamMessageHandler(func(msg *platform.StreamMessage, raw json.RawMessage) {
		if ev := normalizer.Normalize(msg, raw); ev != nil {
			bus.Publish(ctx, *ev)
		}
	})

	_, _, _ = emitter.CollectAll()

	time.Sleep(50 * time.Millisecond)
	for {
		select {
		case ev := <-sub.C():
			if ev.Type == domain.StreamAssistantText {
				if !ev.RawTruncated {
					t.Error("expected raw to be truncated for large message")
				}
				if len(ev.Raw) > domain.RawFieldMaxBytes {
					t.Errorf("raw length %d exceeds max %d", len(ev.Raw), domain.RawFieldMaxBytes)
				}
				return
			}
		default:
			t.Fatal("expected assistant_text event with truncation")
		}
	}
}
