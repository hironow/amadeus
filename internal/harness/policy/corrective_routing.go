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

// DetermineCorrectionDecision turns corrective metadata inputs into a
// deterministic action/routing decision.
func DetermineCorrectionDecision(kind domain.DMailKind, severity domain.Severity, candidateAction domain.DMailAction, failureType domain.FailureType, recurrenceCount int, trigger domain.CorrectionMetadata) CorrectionDecision {
	action := candidateAction
	explicitAction := action != ""
	if action == "" {
		action = domain.DefaultDMailAction(severity)
	}
	targetAgent := CorrectiveTargetAgentForFailure(kind, failureType)
	defaultTarget := targetAgentForKind(kind)
	if targetAgent == "" {
		targetAgent = defaultTarget
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
