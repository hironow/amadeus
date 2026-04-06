package domain

// RoutingPolicy holds configurable routing decision parameters.
// Uses string values instead of DMailAction to ensure cross-tool compatibility
// (DMailAction is amadeus-specific).
type RoutingPolicy struct {
	RecurrenceThreshold int                   // escalate after this many recurrences (default 2)
	SeverityActionMap   map[Severity]string   // severity → action string ("escalate", "retry")
	TargetAgentMap      map[FailureType]string // failure type → target agent override
}

// DefaultRoutingPolicy returns the policy that matches the previous hardcoded behavior.
func DefaultRoutingPolicy() RoutingPolicy {
	return RoutingPolicy{
		RecurrenceThreshold: 2,
		SeverityActionMap: map[Severity]string{
			SeverityHigh:   "escalate",
			SeverityMedium: "retry",
			SeverityLow:    "retry",
		},
		TargetAgentMap: map[FailureType]string{
			FailureTypeScopeViolation:     "sightjack",
			FailureTypeMissingAcceptance:  "sightjack",
			FailureTypeExecutionFailure:   "paintress",
			FailureTypeProviderFailure:    "paintress",
			FailureTypeRoutingFailure:     "paintress",
		},
	}
}

// LookupSeverityAction returns the action for the given severity.
// Returns "retry" if not found in the map.
func (p RoutingPolicy) LookupSeverityAction(sev Severity) string {
	if action, ok := p.SeverityActionMap[NormalizeSeverity(sev)]; ok {
		return action
	}
	return "retry"
}

// LookupTargetAgent returns the target agent override for the given failure type.
// Returns empty string if no override is configured.
func (p RoutingPolicy) LookupTargetAgent(ft FailureType) string {
	return p.TargetAgentMap[ft]
}
