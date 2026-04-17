package domain

import (
	"time"
)

// DivergenceTrendClass categorizes the direction of divergence change over time.
type DivergenceTrendClass string

const (
	// DivergenceTrendWorsening indicates divergence is consistently increasing.
	DivergenceTrendWorsening DivergenceTrendClass = "worsening"
	// DivergenceTrendImproving indicates divergence is consistently decreasing.
	DivergenceTrendImproving DivergenceTrendClass = "improving"
	// DivergenceTrendStable indicates divergence is not significantly changing.
	DivergenceTrendStable DivergenceTrendClass = "stable"
)

// DivergenceTrend holds a trend analysis of divergence scores over recent checks.
type DivergenceTrend struct {
	Class   DivergenceTrendClass `json:"class"`
	Delta   float64              `json:"delta"` // positive = worsening, negative = improving
	Message string               `json:"message"`
}

// CheckType represents the type of divergence check performed.
type CheckType string

const (
	// CheckTypeDiff indicates a diff-based incremental check.
	CheckTypeDiff CheckType = "diff"
	// CheckTypeFull indicates a full repository scan.
	CheckTypeFull CheckType = "full"
)

// CheckResult holds the outcome of a single divergence check.
type CheckResult struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go — JSON event payload for check result; ImpactRadius/PRsEvaluated/DMails/ConvergenceAlerts are snapshot lists at check time [permanent]
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
	GateDenied          bool               `json:"gate_denied,omitempty"`
	CooldownRemaining   int                `json:"cooldown_remaining,omitempty"`
	ADRAlignment        ADRAlignmentMap    `json:"adr_alignment,omitempty"` // E19: per-ADR compliance scores
}
