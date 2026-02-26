package amadeus

import (
	"embed"
	"time"
)

//go:embed templates/skills/*/SKILL.md
var SkillTemplateFS embed.FS

// CheckType represents the type of divergence check performed.
type CheckType string

const (
	// CheckTypeDiff indicates a diff-based incremental check.
	CheckTypeDiff CheckType = "diff"
	// CheckTypeFull indicates a full repository scan.
	CheckTypeFull CheckType = "full"
)

// CheckResult holds the outcome of a single divergence check.
type CheckResult struct {
	CheckedAt           time.Time          `json:"checked_at"`
	Commit              string             `json:"commit"`
	Type                CheckType          `json:"type"`
	Divergence          float64            `json:"divergence"`
	Axes                map[Axis]AxisScore `json:"axes"`
	ImpactRadius        []ImpactEntry      `json:"impact_radius,omitempty"`
	PRsEvaluated        []string           `json:"prs_evaluated"`
	DMails              []string           `json:"dmails"`
	ConvergenceAlerts   []ConvergenceAlert `json:"convergence_alerts,omitempty"`
	CheckCountSinceFull int                `json:"check_count_since_full"`
	ForceFullNext       bool               `json:"force_full_next,omitempty"`
}
