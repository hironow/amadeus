package session

import (
	"context"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"go.opentelemetry.io/otel/trace"
)

// white-box-reason: session internals: tests unexported corrective metadata decision helper

func TestDMailCorrectionMetadata_AllowsRetryForFirstMediumPass(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation", Action: "retry"},
		domain.KindImplFeedback,
		"feedback-1",
		domain.SeverityMedium,
		nil,
		1,
		domain.CorrectionMetadata{},
		trace.SpanFromContext(context.Background()),
	)

	if meta.RetryAllowed == nil || !*meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
	if meta.CorrectiveAction != string(domain.ActionRetry) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionRetry)
	}
	if meta.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", meta.TargetAgent)
	}
	if meta.EscalationReason != "" {
		t.Fatalf("EscalationReason = %q, want empty", meta.EscalationReason)
	}
}

func TestDMailCorrectionMetadata_EscalatesHighSeverity(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation"},
		domain.KindImplFeedback,
		"feedback-2",
		domain.SeverityHigh,
		nil,
		1,
		domain.CorrectionMetadata{},
		trace.SpanFromContext(context.Background()),
	)

	if meta.RetryAllowed == nil || *meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/true, want false")
	}
	if meta.CorrectiveAction != string(domain.ActionEscalate) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionEscalate)
	}
	if meta.TargetAgent != "" {
		t.Fatalf("TargetAgent = %q, want empty", meta.TargetAgent)
	}
	if meta.EscalationReason != "high-severity" {
		t.Fatalf("EscalationReason = %q, want high-severity", meta.EscalationReason)
	}
}

func TestDMailCorrectionMetadata_EscalatesAfterRecurrenceThreshold(t *testing.T) {
	meta := dmailCorrectionMetadata(
		domain.ClaudeDMailCandidate{Category: "implementation", Action: "retry"},
		domain.KindImplFeedback,
		"feedback-3",
		domain.SeverityMedium,
		nil,
		1,
		domain.CorrectionMetadata{
			SchemaVersion:   domain.ImprovementSchemaVersion,
			RecurrenceCount: 1,
			RetryAllowed:    domain.BoolPtr(true),
		},
		trace.SpanFromContext(context.Background()),
	)

	if meta.RecurrenceCount != 2 {
		t.Fatalf("RecurrenceCount = %d, want 2", meta.RecurrenceCount)
	}
	if meta.RetryAllowed == nil || *meta.RetryAllowed {
		t.Fatal("RetryAllowed = nil/true, want false")
	}
	if meta.CorrectiveAction != string(domain.ActionEscalate) {
		t.Fatalf("CorrectiveAction = %q, want %q", meta.CorrectiveAction, domain.ActionEscalate)
	}
	if meta.EscalationReason != "recurrence-threshold" {
		t.Fatalf("EscalationReason = %q, want recurrence-threshold", meta.EscalationReason)
	}
	if meta.Outcome != domain.ImprovementOutcomeFailedAgain {
		t.Fatalf("Outcome = %q, want %q", meta.Outcome, domain.ImprovementOutcomeFailedAgain)
	}
}
