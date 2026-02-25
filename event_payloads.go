package amadeus

// CheckCompletedData is the payload for EventCheckCompleted.
type CheckCompletedData struct {
	Result CheckResult `json:"result"`
}

// BaselineUpdatedData is the payload for EventBaselineUpdated.
type BaselineUpdatedData struct {
	Commit     string  `json:"commit"`
	Divergence float64 `json:"divergence"`
}

// ForceFullNextSetData is the payload for EventForceFullNextSet.
type ForceFullNextSetData struct {
	PreviousDivergence float64 `json:"previous_divergence"`
	CurrentDivergence  float64 `json:"current_divergence"`
}

// DMailGeneratedData is the payload for EventDMailGenerated.
type DMailGeneratedData struct {
	DMail DMail `json:"dmail"`
}

// InboxConsumedData is the payload for EventInboxConsumed.
type InboxConsumedData struct {
	Name   string    `json:"name"`
	Kind   DMailKind `json:"kind"`
	Source string    `json:"source"`
}

// DMailCommentedData is the payload for EventDMailCommented.
type DMailCommentedData struct {
	DMail   string `json:"dmail"`
	IssueID string `json:"issue_id"`
}

// ConvergenceDetectedData is the payload for EventConvergenceDetected.
type ConvergenceDetectedData struct {
	Alert ConvergenceAlert `json:"alert"`
}

// ArchivePrunedData is the payload for EventArchivePruned.
type ArchivePrunedData struct {
	Paths []string `json:"paths"`
	Count int      `json:"count"`
}
