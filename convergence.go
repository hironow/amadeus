package amadeus

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ConvergenceAlert represents a detected world-line convergence:
// multiple D-Mails targeting the same area within a time window.
type ConvergenceAlert struct {
	Target    string    `json:"target"`
	Count     int       `json:"count"`
	Window    int       `json:"window_days"`
	DMails    []string  `json:"dmails"`
	Severity  Severity  `json:"severity"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// AnalyzeConvergence detects targets referenced by multiple D-Mails within
// the configured time window. Returns alerts for targets meeting the threshold.
func AnalyzeConvergence(dmails []DMail, cfg ConvergenceConfig, now time.Time) []ConvergenceAlert {
	windowStart := now.AddDate(0, 0, -cfg.WindowDays)

	// Group D-Mails by target within the time window
	type targetInfo struct {
		dmailNames []string
		firstSeen  time.Time
		lastSeen   time.Time
	}
	targets := make(map[string]*targetInfo)

	for _, d := range dmails {
		if d.Kind == KindConvergence {
			continue
		}
		createdStr, ok := d.Metadata["created_at"]
		if !ok {
			continue
		}
		created, err := time.Parse(time.RFC3339, createdStr)
		if err != nil {
			continue
		}
		if created.Before(windowStart) {
			continue
		}

		for _, target := range d.Targets {
			info, exists := targets[target]
			if !exists {
				info = &targetInfo{firstSeen: created, lastSeen: created}
				targets[target] = info
			}
			info.dmailNames = append(info.dmailNames, d.Name)
			if created.Before(info.firstSeen) {
				info.firstSeen = created
			}
			if created.After(info.lastSeen) {
				info.lastSeen = created
			}
		}
	}

	escalation := cfg.EscalationMultiplier
	if escalation <= 0 {
		escalation = 2
	}

	var alerts []ConvergenceAlert
	for target, info := range targets {
		if len(info.dmailNames) < cfg.Threshold {
			continue
		}
		severity := SeverityMedium
		if len(info.dmailNames) >= cfg.Threshold*escalation {
			severity = SeverityHigh
		}
		alerts = append(alerts, ConvergenceAlert{
			Target:    target,
			Count:     len(info.dmailNames),
			Window:    cfg.WindowDays,
			DMails:    info.dmailNames,
			Severity:  severity,
			FirstSeen: info.firstSeen,
			LastSeen:  info.lastSeen,
		})
	}

	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Target < alerts[j].Target
	})
	return alerts
}

// GenerateConvergenceDMails creates D-Mail entries for HIGH severity convergence alerts.
func GenerateConvergenceDMails(alerts []ConvergenceAlert) []DMail {
	var dmails []DMail
	now := time.Now().UTC()

	for _, alert := range alerts {
		if alert.Severity != SeverityHigh {
			continue
		}

		body := fmt.Sprintf("# World Line Convergence: %s\n\n", alert.Target)
		body += fmt.Sprintf("**%d D-Mails** targeting this area within %d days.\n\n", alert.Count, alert.Window)
		body += "Related D-Mails:\n"
		for _, name := range alert.DMails {
			body += fmt.Sprintf("- %s\n", name)
		}
		body += "\nThis convergence indicates a structural issue requiring attention.\n"

		dmails = append(dmails, DMail{
			Kind:        KindConvergence,
			Description: fmt.Sprintf("World line convergence on %s (%d hits)", alert.Target, alert.Count),
			Targets:     []string{alert.Target},
			Severity:    SeverityHigh,
			Metadata: map[string]string{
				"created_at":      now.Format(time.RFC3339),
				"convergence_for": strings.Join(alert.DMails, ","),
			},
			Body: body,
		})
	}
	return dmails
}
