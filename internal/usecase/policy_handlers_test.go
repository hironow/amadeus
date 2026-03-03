package usecase

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

func TestPolicyHandler_CheckCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger)

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

func TestPolicyHandler_ConvergenceDetected_DebugOnly_NoInfoOutput(t *testing.T) {
	// given: Debug-only handler should NOT produce output when verbose=false
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerCheckPolicies(engine, logger)

	ev, err := domain.NewEvent(domain.EventConvergenceDetected, map[string]string{
		"status": "converged",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: no output
	output := buf.String()
	if output != "" {
		t.Errorf("expected no output for Debug-only handler with verbose=false, got: %s", output)
	}
}
