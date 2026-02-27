package amadeus

// Policy represents an implicit reactive rule: WHEN [EVENT] THEN [COMMAND].
// P1-17 initial scope: type definition and registry only; dispatch engine is P2+.
type Policy struct {
	Name    string    // unique identifier for the policy
	Trigger EventType // domain event that activates this policy
	Action  string    // description of the resulting command
}

// Policies registers all known implicit policies in amadeus.
// These document the existing reactive behaviors for future automation.
var Policies = []Policy{
	{Name: "CheckCompletedGenerateDMail", Trigger: EventCheckCompleted, Action: "GenerateDMail"},
	{Name: "ConvergenceDetectedNotify", Trigger: EventConvergenceDetected, Action: "NotifyConvergence"},
	{Name: "InboxConsumedUpdateProjection", Trigger: EventInboxConsumed, Action: "UpdateProjection"},
	{Name: "DMailGeneratedFlushOutbox", Trigger: EventDMailGenerated, Action: "FlushOutbox"},
}
