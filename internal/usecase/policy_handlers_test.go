package usecase

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

type notifyCall struct {
	title   string
	message string
}

type spyNotifier struct {
	calls []notifyCall
}

type metricsCall struct {
	eventType string
	status    string
}

type spyPolicyMetrics struct {
	calls []metricsCall
}

func (s *spyPolicyMetrics) RecordPolicyEvent(_ context.Context, eventType, status string) {
	s.calls = append(s.calls, metricsCall{eventType: eventType, status: status})
}

func (s *spyNotifier) Notify(_ context.Context, title, message string) error {
	s.calls = append(s.calls, notifyCall{title: title, message: message})
	return nil
}

func TestPolicyHandler_CheckCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger, &port.NopNotifier{}, &port.NopPolicyMetrics{})

	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: domain.CheckResult{
			Divergence: 0.42,
			Commit:     "abc1234",
		},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Info-level output should contain divergence and commit
	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level output, got: %s", output)
	}
	if !strings.Contains(output, "0.42") {
		t.Errorf("expected divergence in output, got: %s", output)
	}
	if !strings.Contains(output, "abc1234") {
		t.Errorf("expected commit in output, got: %s", output)
	}
}

func TestPolicyHandler_CheckCompleted_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger, spy, &port.NopPolicyMetrics{})

	ev, err := domain.NewEvent(domain.EventCheckCompleted, domain.CheckCompletedData{
		Result: domain.CheckResult{
			Divergence: 0.42,
			Commit:     "abc1234",
		},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Notify should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Amadeus") {
		t.Errorf("expected title to contain 'Amadeus', got: %s", call.title)
	}
	if !strings.Contains(call.message, "0.42") {
		t.Errorf("expected message to contain divergence, got: %s", call.message)
	}
	if !strings.Contains(call.message, "abc1234") {
		t.Errorf("expected message to contain commit, got: %s", call.message)
	}
}

func TestPolicyHandler_ConvergenceDetected_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventConvergenceDetected, map[string]string{
		"status": "converged",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: metrics should have been recorded
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "convergence.detected" {
		t.Errorf("expected eventType 'convergence.detected', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_InboxConsumed_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventInboxConsumed, map[string]string{
		"kind": "specification",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "inbox.consumed" {
		t.Errorf("expected eventType 'inbox.consumed', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_DMailGenerated_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventDMailGenerated, map[string]string{
		"kind": "feedback",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "dmail.generated" {
		t.Errorf("expected eventType 'dmail.generated', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_ConvergenceDetected_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger, spy, &port.NopPolicyMetrics{})

	ev, err := domain.NewEvent(domain.EventConvergenceDetected, map[string]string{
		"status": "converged",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Notify should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Amadeus") {
		t.Errorf("expected title to contain 'Amadeus', got: %s", call.title)
	}
	if !strings.Contains(call.message, "Convergence") {
		t.Errorf("expected message to contain 'Convergence', got: %s", call.message)
	}
}
