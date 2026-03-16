package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// dataOut writes a formatted line to DataOut (stdout / machine-facing).
func (a *Amadeus) dataOut(format string, args ...any) {
	fmt.Fprintf(a.DataOut, "  "+format+"\n", args...)
}

// writeDataJSON marshals v as indented JSON and writes it to DataOut.
func (a *Amadeus) writeDataJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Fprintln(a.DataOut, string(data))
	return nil
}

// PrintCheckOutput renders the CLI display for a completed check.
func (a *Amadeus) PrintCheckOutput(result domain.CheckResult, dmails []domain.DMail, previousDivergence float64) {
	a.dataOut("")
	a.dataOut("Divergence: %s (%s)",
		domain.FormatDivergence(result.Divergence*100),
		domain.FormatDelta(result.Divergence, previousDivergence))

	axisOrder := []domain.Axis{domain.AxisADR, domain.AxisDoD, domain.AxisDependency, domain.AxisImplicit}
	axisNames := map[domain.Axis]string{
		domain.AxisADR:        "ADR Integrity",
		domain.AxisDoD:        "DoD Fulfillment",
		domain.AxisDependency: "Dependency Integrity",
		domain.AxisImplicit:   "Implicit Constraints",
	}

	for _, axis := range axisOrder {
		if score, ok := result.Axes[axis]; ok {
			weight := a.Config.WeightFor(axis)
			contribution := float64(score.Score) * weight
			a.dataOut("  %-22s %s — %s",
				axisNames[axis]+":",
				domain.FormatDivergence(contribution),
				score.Details)
		}
	}

	if len(result.ImpactRadius) > 0 {
		a.dataOut("")
		a.dataOut("Impact Radius:")
		for _, entry := range result.ImpactRadius {
			a.dataOut("  [%s] %s — %s", entry.Impact, entry.Area, entry.Detail)
		}
	}

	if len(dmails) > 0 {
		a.dataOut("")
		a.dataOut("D-Mails:")
		for _, d := range dmails {
			var prefix string
			switch d.Severity {
			case domain.SeverityHigh:
				prefix = "[HIGH]"
			case domain.SeverityMedium:
				prefix = "[MED] "
			default:
				prefix = "[LOW] "
			}
			a.dataOut("  %s %s %s → sent",
				prefix, d.Name, d.Description)
		}
	}

	if len(result.ConvergenceAlerts) > 0 {
		a.dataOut("")
		a.dataOut("Convergence Alerts:")
		for _, alert := range result.ConvergenceAlerts {
			a.dataOut("  [%s] %s — %d hits in %d days (%d D-Mails)",
				strings.ToUpper(string(alert.Severity)),
				alert.Target,
				alert.Count,
				alert.Window,
				len(alert.DMails))
		}
	}
}

// PrintCheckOutputJSON writes the check result as JSON to DataOut.
func (a *Amadeus) PrintCheckOutputJSON(result domain.CheckResult, dmails []domain.DMail, previousDivergence float64) error {
	convergenceAlerts := result.ConvergenceAlerts
	if convergenceAlerts == nil {
		convergenceAlerts = []domain.ConvergenceAlert{}
	}
	output := struct {
		Divergence        float64                          `json:"divergence"`
		Delta             float64                          `json:"delta"`
		Axes              map[domain.Axis]domain.AxisScore `json:"axes"`
		ImpactRadius      []domain.ImpactEntry             `json:"impact_radius"`
		DMails            []domain.DMail                   `json:"dmails"`
		ConvergenceAlerts []domain.ConvergenceAlert        `json:"convergence_alerts"`
	}{
		Divergence:        result.Divergence,
		Delta:             result.Divergence - previousDivergence,
		Axes:              result.Axes,
		ImpactRadius:      result.ImpactRadius,
		DMails:            dmails,
		ConvergenceAlerts: convergenceAlerts,
	}
	if output.DMails == nil {
		output.DMails = []domain.DMail{}
	}
	if output.ImpactRadius == nil {
		output.ImpactRadius = []domain.ImpactEntry{}
	}
	return a.writeDataJSON(output)
}

// PrintCheckOutputQuiet renders a single-line summary for --quiet mode.
func (a *Amadeus) PrintCheckOutputQuiet(result domain.CheckResult, dmails []domain.DMail, previousDivergence float64) {
	dmailLabel := "D-Mails"
	if len(dmails) == 1 {
		dmailLabel = "D-Mail"
	}

	convergenceStr := ""
	if len(result.ConvergenceAlerts) > 0 {
		convergenceStr = fmt.Sprintf(" %d convergence", len(result.ConvergenceAlerts))
	}

	a.dataOut("%s (%s) %d %s%s",
		domain.FormatDivergence(result.Divergence*100),
		domain.FormatDelta(result.Divergence, previousDivergence),
		len(dmails),
		dmailLabel,
		convergenceStr)
}

// loadCheckHistory returns CheckResults extracted from the event store.
func (a *Amadeus) loadCheckHistory() ([]domain.CheckResult, error) {
	if a.Events == nil {
		return nil, nil
	}
	events, _, err := a.Events.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	var results []domain.CheckResult
	for _, ev := range events {
		if ev.Type != domain.EventCheckCompleted {
			continue
		}
		var data domain.CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return nil, fmt.Errorf("unmarshal check event %s: %w", ev.ID, err)
		}
		results = append(results, data.Result)
	}
	// Events are chronological; history is newest-first
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results, nil
}

// PrintLog renders the history and D-Mail log to DataOut.
func (a *Amadeus) PrintLog() error {
	history, err := a.loadCheckHistory()
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	a.dataOut("")
	if len(history) == 0 {
		a.dataOut("No history yet. Run `amadeus check` first.")
		return nil
	}

	a.dataOut("History:")
	for i, h := range history {
		var delta string
		if h.Type == domain.CheckTypeFull {
			delta = "(baseline)"
		} else if i+1 < len(history) {
			delta = "(" + domain.FormatDelta(h.Divergence, history[i+1].Divergence) + ")"
		} else {
			delta = "(first)"
		}
		dmailCount := len(h.DMails)
		dmailLabel := "D-Mails"
		if dmailCount == 1 {
			dmailLabel = "D-Mail"
		}
		a.dataOut("  %s  %s  %-4s  %s %s  %d %s",
			h.CheckedAt.Format("2006-01-02T15:04"),
			h.Commit,
			string(h.Type),
			domain.FormatDivergence(h.Divergence*100),
			delta,
			dmailCount,
			dmailLabel)
	}

	dmails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load dmails: %w", err)
	}

	if len(dmails) > 0 {
		a.dataOut("")
		a.dataOut("D-Mails:")
		for _, d := range dmails {
			var severityTag string
			switch d.Severity {
			case domain.SeverityHigh:
				severityTag = "[HIGH]"
			case domain.SeverityMedium:
				severityTag = "[MED] "
			default:
				severityTag = "[LOW] "
			}
			a.dataOut("  %s  %s %-10s %s",
				d.Name,
				severityTag,
				string(domain.DMailSent),
				d.Description)
		}
	}

	// Convergence alerts from current archive
	convergenceAlerts := a.Config.DetectConvergence(dmails, time.Now().UTC())
	if len(convergenceAlerts) > 0 {
		a.dataOut("")
		a.dataOut("Convergence Alerts:")
		for _, alert := range convergenceAlerts {
			a.dataOut("  [%s] %s — %d hits in %d days (%d D-Mails)",
				strings.ToUpper(string(alert.Severity)),
				alert.Target,
				alert.Count,
				alert.Window,
				len(alert.DMails))
		}
	}

	consumed, err := a.Store.LoadConsumed()
	if err != nil {
		return fmt.Errorf("load consumed: %w", err)
	}
	if len(consumed) > 0 {
		a.dataOut("")
		a.dataOut("Consumed:")
		for _, c := range consumed {
			a.dataOut("  %s  [%s]  %s",
				c.Name,
				string(c.Kind),
				c.ConsumedAt.Format("2006-01-02T15:04"))
		}
	}

	return nil
}

// dmailJSONView is a JSON-specific view of a D-Mail with status.
type dmailJSONView struct {
	Name        string            `json:"name"`
	Kind        domain.DMailKind  `json:"kind"`
	Description string            `json:"description"`
	Issues      []string          `json:"issues,omitempty"`
	Severity    domain.Severity   `json:"severity,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      string            `json:"status"`
}

// PrintLogJSON writes the history and D-Mail log as JSON to DataOut.
func (a *Amadeus) PrintLogJSON() error {
	history, err := a.loadCheckHistory()
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	dmails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load dmails: %w", err)
	}
	consumed, err := a.Store.LoadConsumed()
	if err != nil {
		return fmt.Errorf("load consumed: %w", err)
	}
	if consumed == nil {
		consumed = []domain.ConsumedRecord{}
	}
	if history == nil {
		history = []domain.CheckResult{}
	}

	views := make([]dmailJSONView, len(dmails))
	for i, d := range dmails {
		views[i] = dmailJSONView{
			Name:        d.Name,
			Kind:        d.Kind,
			Description: d.Description,
			Issues:      d.Issues,
			Severity:    d.Severity,
			Metadata:    d.Metadata,
			Status:      string(domain.DMailSent),
		}
	}

	convergenceAlerts := a.Config.DetectConvergence(dmails, time.Now().UTC())
	if convergenceAlerts == nil {
		convergenceAlerts = []domain.ConvergenceAlert{}
	}

	output := struct {
		History           []domain.CheckResult      `json:"history"`
		DMails            []dmailJSONView           `json:"dmails"`
		Consumed          []domain.ConsumedRecord   `json:"consumed"`
		ConvergenceAlerts []domain.ConvergenceAlert `json:"convergence_alerts"`
	}{
		History:           history,
		DMails:            views,
		Consumed:          consumed,
		ConvergenceAlerts: convergenceAlerts,
	}
	return a.writeDataJSON(output)
}

// PrintSync builds and outputs the sync status as JSON to DataOut.
// Lists D-Mail x Issue pairs that have not yet been posted as comments.
func (a *Amadeus) PrintSync() error {
	syncState, err := a.Store.LoadSyncState()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}
	allDMails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load all dmails: %w", err)
	}

	var pendingComments []domain.PendingComment
	for _, d := range allDMails {
		if len(d.Issues) == 0 {
			continue
		}
		for _, issueID := range d.Issues {
			key := d.Name + ":" + issueID
			if _, commented := syncState.CommentedDMails[key]; commented {
				continue
			}
			pendingComments = append(pendingComments, domain.PendingComment{
				DMail:       d.Name,
				IssueID:     issueID,
				Status:      string(domain.DMailSent),
				Description: d.Description,
			})
		}
	}
	if pendingComments == nil {
		pendingComments = []domain.PendingComment{}
	}

	output := domain.SyncOutput{
		PendingComments: pendingComments,
	}
	return a.writeDataJSON(output)
}
