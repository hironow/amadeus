package amadeus

// MeterResult holds the complete output of Phase 2 scoring orchestration.
type MeterResult struct {
	Divergence      DivergenceResult
	DMailCandidates []ClaudeDMailCandidate
	Reasoning       string
	ImpactRadius    []ImpactEntry
}

// DivergenceMeter bridges Claude output and the scoring engine.
type DivergenceMeter struct {
	Config Config
}

// ProcessResponse takes a ClaudeResponse, runs CalcDivergence and
// DetermineSeverity, and returns a MeterResult.
func (dm *DivergenceMeter) ProcessResponse(resp ClaudeResponse) MeterResult {
	divergence := CalcDivergence(resp.Axes, dm.Config.Weights)
	severityCfg := SeverityConfig{
		Thresholds:      dm.Config.Thresholds,
		PerAxisOverride: dm.Config.PerAxisOverride,
	}
	divergence = DetermineSeverity(divergence, severityCfg)
	return MeterResult{
		Divergence:      divergence,
		DMailCandidates: resp.DMails,
		Reasoning:       resp.Reasoning,
		ImpactRadius:    resp.ImpactRadius,
	}
}
