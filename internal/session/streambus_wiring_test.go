package session

// white-box-reason: tests that ClaudeAdapter emits exactly 1 session_end via StreamBus,
// that CodingSessionID propagates through RunOption → normalizer → event,
// and that child spans inherit the caller's trace ID (GAP-TST-051).
// Also verifies trace propagation through RunDetailed (GAP-ARCH-050).

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// fakeStreamJSON returns minimal Claude stream-json output for testing.
func fakeStreamJSON() string {
	return strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"fake-sess","model":"test","tools":[]}`,
		`{"type":"result","subtype":"success","session_id":"fake-sess","result":"done","usage":{"input_tokens":100,"output_tokens":50},"total_cost_usd":0.001,"duration_ms":500}`,
	}, "\n") + "\n"
}

func TestStreamBusWiring_AdapterEmitsExactlyOneSessionEnd(t *testing.T) {
	// given: bus + subscriber
	bus := platform.NewInProcessSessionBus()
	defer bus.Close()
	sub := bus.Subscribe(64)
	defer sub.Close()

	adapter := &ClaudeAdapter{
		ClaudeCmd: "fake-claude",
		Model:     "test",
		Logger:    &domain.NopLogger{},
		StreamBus: bus,
		ToolName:  "amadeus",
		NewCmd: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "printf", "%s", fakeStreamJSON())
		},
	}

	// when: run adapter with CodingSessionID via RunOption
	_, err := adapter.RunDetailed(context.Background(), "test prompt", os.Stdout,
		port.WithCodingSessionID("test-coding-session-42"))
	if err != nil {
		t.Fatalf("RunDetailed: %v", err)
	}

	// then: collect events from subscriber
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

	// Verify: exactly 1 session_end event
	var sessionEnds []domain.SessionStreamEvent
	for _, ev := range events {
		if ev.Type == domain.StreamSessionEnd {
			sessionEnds = append(sessionEnds, ev)
		}
	}
	if len(sessionEnds) != 1 {
		t.Errorf("expected exactly 1 session_end, got %d", len(sessionEnds))
		for i, ev := range sessionEnds {
			t.Logf("  session_end[%d]: SessionID=%s, Data=%s", i, ev.SessionID, string(ev.Data))
		}
	}

	// Verify: session_end contains CodingSessionID
	if len(sessionEnds) > 0 {
		endEv := sessionEnds[0]
		if endEv.SessionID != "test-coding-session-42" {
			t.Errorf("expected CodingSessionID=test-coding-session-42, got %q", endEv.SessionID)
		}
		if endEv.Tool != "amadeus" {
			t.Errorf("expected Tool=amadeus, got %q", endEv.Tool)
		}
		data := string(endEv.Data)
		if !strings.Contains(data, "input_tokens") {
			t.Errorf("session_end should contain usage data, got: %s", data)
		}
	}

	if len(events) < 2 {
		t.Errorf("expected at least 2 events (start + end), got %d", len(events))
		for i, ev := range events {
			fmt.Printf("  event[%d]: type=%s\n", i, ev.Type)
		}
	}
}

func TestStreamBusWiring_SessionEndInheritsParentTrace(t *testing.T) {
	exp := setupTestTracer(t)

	bus := platform.NewInProcessSessionBus()
	defer bus.Close()

	adapter := &ClaudeAdapter{
		ClaudeCmd: "fake-claude",
		Model:     "test",
		Logger:    &domain.NopLogger{},
		StreamBus: bus,
		ToolName:  "amadeus",
		NewCmd: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "printf", "%s", fakeStreamJSON())
		},
	}

	// Create parent span
	parentCtx, parentSpan := platform.Tracer.Start(context.Background(), "test.parent")
	defer parentSpan.End()
	parentTraceID := parentSpan.SpanContext().TraceID()

	_, err := adapter.RunDetailed(parentCtx, "trace test", os.Stdout)
	if err != nil {
		t.Fatalf("RunDetailed: %v", err)
	}

	// Verify child spans inherit parent trace
	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span from RunDetailed")
	}
	for _, s := range spans {
		if s.SpanContext.TraceID() != parentTraceID {
			t.Errorf("span %q trace ID = %s, want parent trace ID = %s",
				s.Name, s.SpanContext.TraceID(), parentTraceID)
		}
	}
}
