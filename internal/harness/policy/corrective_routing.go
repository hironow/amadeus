package policy

import "github.com/hironow/amadeus/internal/domain"

// CorrectionDecision captures the deterministic routing decision for a
// corrective D-Mail before session-specific metadata is attached.
type CorrectionDecision struct {
	Action           domain.DMailAction
	RoutingMode      domain.RoutingMode
	TargetAgent      string
	RetryAllowed     *bool
	EscalationReason string
}

// CorrectiveTargetAgentForFailure resolves the owner that should receive
// corrective feedback for a given failure type.
func CorrectiveTargetAgentForFailure(kind domain.DMailKind, failureType domain.FailureType) string {
	switch failureType {
	case domain.FailureTypeScopeViolation, domain.FailureTypeMissingAcceptance:
		return "sightjack"
	case domain.FailureTypeExecutionFailure, domain.FailureTypeProviderFailure, domain.FailureTypeRoutingFailure:
		return "paintress"
	default:
		return targetAgentForKind(kind)
	}
}

// DetectOwnerLoop checks if the owner history shows a repeated agent ping-pong.
// Returns true if the last 4 entries form [A, B, A, B] pattern where A != B.
func DetectOwnerLoop(ownerHistory []string) bool {
	n := len(ownerHistory)
	if n < 4 {
		return false
	}
	a, b, c, d := ownerHistory[n-4], ownerHistory[n-3], ownerHistory[n-2], ownerHistory[n-1]
	return a == c && b == d && a != b
}

// DetermineCorrectionDecision turns corrective metadata inputs into a
// deterministic action/routing decision.
//
// Decision priority:
//   0. Provider state gate (paused/degraded-exhausted/waiting-exhausted → escalate)
//   0.5. Owner loop detection (ping-pong → escalate)
//   1. RetryAllowed=false → escalate
//   2. RecurrenceCount >= 2 → escalate
//   3. Explicit escalation → escalate
//   4. Explicit non-escalation action → use it
//   5. High severity → escalate
//   6. Default → retry/reroute
//
// Escalation target resolution:
// Escalated D-Mails are delivered to the same target agent (sightjack/paintress)
// but with routing_mode=escalate. The receiving tool's existing approval gate
// (sightjack convergence gate / paintress inbox HIGH-severity gate) handles
// the human-in-the-loop handoff. No dedicated human endpoint is needed.
func DetermineCorrectionDecision(kind domain.DMailKind, severity domain.Severity, candidateAction domain.DMailAction, failureType domain.FailureType, recurrenceCount int, trigger domain.CorrectionMetadata, providerSnapshot domain.ProviderStateSnapshot) CorrectionDecision {
	targetAgent := CorrectiveTargetAgentForFailure(kind, failureType)
	defaultTarget := targetAgentForKind(kind)
	if targetAgent == "" {
		targetAgent = defaultTarget
	}

	// Priority 0: Provider state gate
	// Only 3 specific conditions escalate — avoids over-escalation from B2's
	// EvaluateExhaustion which maps degraded(any) and active(0) to Pause.
	if providerEscalation := providerStateGate(providerSnapshot, targetAgent); providerEscalation != nil {
		return *providerEscalation
	}

	// Priority 0.5: Owner loop detection
	if DetectOwnerLoop(trigger.OwnerHistory) {
		return CorrectionDecision{
			Action:           domain.ActionEscalate,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: "owner-loop-detected",
		}
	}

	action := candidateAction
	explicitAction := action != ""
	if action == "" {
		action = domain.DefaultDMailAction(severity)
	}

	if trigger.RetryAllowed != nil && !*trigger.RetryAllowed {
		return CorrectionDecision{
			Action:           domain.ActionEscalate,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: fallbackEscalationReason(trigger.EscalationReason),
		}
	}
	switch {
	case recurrenceCount >= 2:
		return CorrectionDecision{
			Action:           domain.ActionEscalate,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: "recurrence-threshold",
		}
	case explicitAction && action == domain.ActionEscalate:
		return CorrectionDecision{
			Action:           action,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: "candidate-requested-escalation",
		}
	case explicitAction:
		return CorrectionDecision{
			Action:       action,
			RoutingMode:  routingModeForTarget(defaultTarget, targetAgent),
			TargetAgent:  targetAgent,
			RetryAllowed: domain.BoolPtr(true),
		}
	case domain.NormalizeSeverity(severity) == domain.SeverityHigh:
		return CorrectionDecision{
			Action:           domain.ActionEscalate,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: "high-severity",
		}
	case action == domain.ActionEscalate:
		return CorrectionDecision{
			Action:           action,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: "candidate-requested-escalation",
		}
	default:
		return CorrectionDecision{
			Action:       action,
			RoutingMode:  routingModeForTarget(defaultTarget, targetAgent),
			TargetAgent:  targetAgent,
			RetryAllowed: domain.BoolPtr(true),
		}
	}
}

// providerStateGate checks provider health and returns an escalation decision
// when the provider cannot support retry. Returns nil when existing logic
// should handle the decision.
//
// Decision table (routing-specific, NOT delegating to EvaluateExhaustion):
//
//	| state    | budget | action    | reason                       |
//	|----------|--------|-----------|------------------------------|
//	| paused   | any    | Escalate  | provider-paused              |
//	| degraded | 0      | Escalate  | provider-degraded-exhausted  |
//	| waiting  | 0      | Escalate  | provider-waiting-exhausted   |
//	| other    | any    | nil       | (delegate to existing logic) |
func providerStateGate(snapshot domain.ProviderStateSnapshot, targetAgent string) *CorrectionDecision {
	switch snapshot.State {
	case domain.ProviderStatePaused:
		return &CorrectionDecision{
			Action:           domain.ActionEscalate,
			RoutingMode:      domain.RoutingModeEscalate,
			TargetAgent:      targetAgent,
			RetryAllowed:     domain.BoolPtr(false),
			EscalationReason: "provider-paused",
		}
	case domain.ProviderStateDegraded:
		if snapshot.RetryBudget <= 0 {
			return &CorrectionDecision{
				Action:           domain.ActionEscalate,
				RoutingMode:      domain.RoutingModeEscalate,
				TargetAgent:      targetAgent,
				RetryAllowed:     domain.BoolPtr(false),
				EscalationReason: "provider-degraded-exhausted",
			}
		}
	case domain.ProviderStateWaiting:
		if snapshot.RetryBudget <= 0 {
			return &CorrectionDecision{
				Action:           domain.ActionEscalate,
				RoutingMode:      domain.RoutingModeEscalate,
				TargetAgent:      targetAgent,
				RetryAllowed:     domain.BoolPtr(false),
				EscalationReason: "provider-waiting-exhausted",
			}
		}
	}
	return nil
}

func targetAgentForKind(kind domain.DMailKind) string {
	switch kind {
	case domain.KindDesignFeedback:
		return "sightjack"
	case domain.KindImplFeedback:
		return "paintress"
	default:
		return ""
	}
}

func routingModeForTarget(defaultTarget, targetAgent string) domain.RoutingMode {
	if targetAgent != "" && defaultTarget != "" && targetAgent != defaultTarget {
		return domain.RoutingModeReroute
	}
	return domain.RoutingModeRetry
}

func fallbackEscalationReason(reason string) string {
	if reason == "" {
		return "retry-disabled"
	}
	return reason
}
