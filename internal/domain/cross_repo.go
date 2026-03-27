package domain

import "time"

// ToolName identifies a tool in the TAP ecosystem.
type ToolName string

const (
	ToolPhonewave ToolName = "phonewave"
	ToolSightjack ToolName = "sightjack"
	ToolPaintress ToolName = "paintress"
	ToolAmadeus   ToolName = "amadeus"
)

// AllTools lists all tools in the TAP ecosystem.
var AllTools = []ToolName{ToolPhonewave, ToolSightjack, ToolPaintress, ToolAmadeus}

// toolStateDirs maps each tool to its state directory name.
var toolStateDirs = map[ToolName]string{
	ToolPhonewave: ".phonewave",
	ToolSightjack: ".siren",
	ToolPaintress: ".expedition",
	ToolAmadeus:   ".gate",
}

// ToolStateDir returns the state directory name for the given tool.
// Returns empty string for unknown tools.
func ToolStateDir(tool ToolName) string {
	return toolStateDirs[tool]
}

// ToolSnapshot holds the divergence state for a single tool.
type ToolSnapshot struct {
	Tool       ToolName  `json:"tool"`
	Divergence float64   `json:"divergence"`
	Severity   Severity  `json:"severity"`
	LastCheck  time.Time `json:"last_check"`
	Available  bool      `json:"available"`
}

// CrossRepoSnapshot aggregates divergence data across all tools.
type CrossRepoSnapshot struct {
	Snapshots      []ToolSnapshot `json:"snapshots"`
	EcosystemScore float64        `json:"ecosystem_score"`
	MaxSeverity    Severity       `json:"max_severity"`
	GeneratedAt    time.Time      `json:"generated_at"`
}

// NewCrossRepoSnapshot constructs a CrossRepoSnapshot with computed aggregate fields.
func NewCrossRepoSnapshot(snapshots []ToolSnapshot, generatedAt time.Time) CrossRepoSnapshot {
	return CrossRepoSnapshot{
		Snapshots:      snapshots,
		EcosystemScore: ComputeEcosystemScore(snapshots),
		MaxSeverity:    MaxSeverityAcrossTools(snapshots),
		GeneratedAt:    generatedAt,
	}
}

// ComputeEcosystemScore returns the average divergence across available tools.
// Returns 0.0 if no tools are available.
func ComputeEcosystemScore(snapshots []ToolSnapshot) float64 {
	var sum float64
	var count int
	for _, s := range snapshots {
		if s.Available {
			sum += s.Divergence
			count++
		}
	}
	if count == 0 {
		return 0.0
	}
	return sum / float64(count)
}

// MaxSeverityAcrossTools returns the highest severity among available tools.
// Returns SeverityLow if no tools are available.
func MaxSeverityAcrossTools(snapshots []ToolSnapshot) Severity {
	result := SeverityLow
	for _, s := range snapshots {
		if !s.Available {
			continue
		}
		switch s.Severity {
		case SeverityHigh:
			return SeverityHigh
		case SeverityMedium:
			result = SeverityMedium
		}
	}
	return result
}
