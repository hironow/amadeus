package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// writeDivergenceInsight writes a divergence insight entry after scoring.
// Fails silently (log warning) to avoid breaking the check pipeline.
func (a *Amadeus) writeDivergenceInsight(result domain.DivergenceResult, sessionID string, commitRange string, reasoning string) {
	if a.Insights == nil {
		return
	}

	// Build axis details for the "why" field
	whyParts := highScoringAxisDetails(result.Axes)
	why := "No high-scoring axes"
	if len(whyParts) > 0 {
		why = strings.Join(whyParts, "; ")
	}

	how := "Focus remediation on highest-scoring axis"
	if reasoning != "" {
		how = reasoning
	}

	entry := domain.InsightEntry{
		Title:       fmt.Sprintf("divergence-%s", sessionID),
		What:        fmt.Sprintf("Divergence score: %f, severity: %s", result.Value, result.Severity),
		Why:         why,
		How:         how,
		When:        fmt.Sprintf("Check on commits %s", commitRange),
		Who:         fmt.Sprintf("amadeus run (session-%s)", sessionID),
		Constraints: "Scores relative to configured weights",
		Extra:       divergenceExtra(result),
	}

	if err := a.Insights.Append("divergence.md", "divergence", "amadeus", entry); err != nil {
		a.Logger.Warn("insight write (divergence): %v", err)
	}
}

// writeConvergenceInsight writes a convergence insight entry for a HIGH severity alert.
// Fails silently (log warning) to avoid breaking the check pipeline.
func (a *Amadeus) writeConvergenceInsight(alert domain.ConvergenceAlert, sessionID string, archiveDir string) {
	if a.Insights == nil {
		return
	}

	if alert.Severity != domain.SeverityHigh {
		return
	}

	why := "Multiple feedback signals targeting same area indicates structural issue"
	if descs := collectDMailDescriptions(archiveDir, alert.DMails); len(descs) > 0 {
		why = fmt.Sprintf("Converging feedback: %s", strings.Join(descs, "; "))
	}

	entry := domain.InsightEntry{
		Title:       fmt.Sprintf("convergence-%s", alert.Target),
		What:        fmt.Sprintf("World line convergence on %s: %d D-Mails in %d days", alert.Target, alert.Count, alert.Window),
		Why:         why,
		How:         "Investigate for shared root cause",
		When:        fmt.Sprintf("When %d D-Mails target same area within window", alert.Count),
		Who:         fmt.Sprintf("amadeus convergence detector (session-%s)", sessionID),
		Constraints: "Escalation threshold",
		Extra: map[string]string{
			"related-dmails": strings.Join(alert.DMails, ", "),
		},
	}

	if err := a.Insights.Append("convergence.md", "convergence", "amadeus", entry); err != nil {
		a.Logger.Warn("insight write (convergence): %v", err)
	}
}

// collectDMailDescriptions reads D-Mail files from the archive directory
// and extracts their description frontmatter fields.
func collectDMailDescriptions(archiveDir string, names []string) []string {
	var descs []string
	for _, name := range names {
		path := filepath.Join(archiveDir, name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		dmail, err := domain.ParseDMail(data)
		if err != nil {
			continue
		}
		if dmail.Description != "" {
			descs = append(descs, dmail.Description)
		}
	}
	return descs
}

// highScoringAxisDetails returns detail strings for axes with score >= 50.
func highScoringAxisDetails(axes map[domain.Axis]domain.AxisScore) []string {
	var parts []string
	// Sort keys for deterministic output
	keys := make([]string, 0, len(axes))
	for k := range axes {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)

	for _, k := range keys {
		axis := domain.Axis(k)
		as := axes[axis]
		if as.Score >= 50 {
			parts = append(parts, fmt.Sprintf("%s=%d (%s)", axis, as.Score, as.Details))
		}
	}
	return parts
}

// divergenceExtra builds the Extra map for a divergence insight entry.
func divergenceExtra(result domain.DivergenceResult) map[string]string {
	// Format axis scores
	var axisParts []string
	keys := make([]string, 0, len(result.Axes))
	for k := range result.Axes {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		axis := domain.Axis(k)
		as := result.Axes[axis]
		axisParts = append(axisParts, fmt.Sprintf("%s=%d", axis, as.Score))
	}

	return map[string]string{
		"axis-scores": strings.Join(axisParts, ", "),
	}
}
