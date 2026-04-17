package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BaselinePoint records a single baseline update event for historical tracking. //nolint:all // nosemgrep: sql-in-domain-go -- comment text matched pattern; no actual SQL in this file [permanent]
type BaselinePoint struct {
	Commit     string    `json:"commit"`
	Divergence float64   `json:"divergence"`
	At         time.Time `json:"at"`
}

// StatusReport holds operational status information for the amadeus tool.
type StatusReport struct {
	LastCheck           time.Time        `json:"last_check"`
	Divergence          float64          `json:"divergence"`
	CheckCount          int              `json:"check_count"`
	InboxCount          int              `json:"inbox_count"`
	ArchiveCount        int              `json:"archive_count"`
	SuccessRate         float64          `json:"success_rate"`
	Convergences        int              `json:"convergences"`
	ProviderState       string           `json:"provider_state,omitempty"`
	ProviderReason      string           `json:"provider_reason,omitempty"`
	ProviderRetryBudget int              `json:"provider_retry_budget,omitempty"`
	ProviderResumeAt    time.Time        `json:"provider_resume_at,omitempty"`
	ProviderResumeWhen  string           `json:"provider_resume_when,omitempty"`
	BaselineHistory     []BaselinePoint  `json:"baseline_history,omitempty"`
	Trend               *DivergenceTrend `json:"trend,omitempty"`
}

// FormatText returns a human-readable status report string suitable for stdout.
func (r StatusReport) FormatText() string {
	var b strings.Builder
	b.WriteString("amadeus status\n\n")

	// Last check
	if r.LastCheck.IsZero() {
		fmt.Fprintf(&b, "  %-16s %s\n", "Last check:", "no checks yet")
	} else {
		fmt.Fprintf(&b, "  %-16s %s\n", "Last check:", r.LastCheck.Format(time.RFC3339))
	}

	fmt.Fprintf(&b, "  %-16s %.2f\n", "Divergence:", r.Divergence)
	fmt.Fprintf(&b, "  %-16s %d total\n", "Checks:", r.CheckCount)

	// Success rate
	if r.CheckCount == 0 {
		fmt.Fprintf(&b, "  %-16s %s\n", "Success rate:", "no events")
	} else {
		fmt.Fprintf(&b, "  %-16s %.1f%%\n", "Success rate:", r.SuccessRate*100)
	}

	fmt.Fprintf(&b, "  %-16s %d pending\n", "Inbox:", r.InboxCount)
	fmt.Fprintf(&b, "  %-16s %d processed\n", "Archive:", r.ArchiveCount)
	fmt.Fprintf(&b, "  %-16s %d active\n", "Convergences:", r.Convergences)
	if r.ProviderState != "" {
		fmt.Fprintf(&b, "  %-16s %s", "Provider:", r.ProviderState)
		if r.ProviderReason != "" {
			fmt.Fprintf(&b, " (%s)", r.ProviderReason)
		}
		b.WriteByte('\n')
		if r.ProviderRetryBudget > 0 {
			fmt.Fprintf(&b, "  %-16s %d\n", "Retry budget:", r.ProviderRetryBudget)
		}
		if r.ProviderResumeWhen != "" {
			fmt.Fprintf(&b, "  %-16s %s\n", "Resume when:", r.ProviderResumeWhen)
		}
		if !r.ProviderResumeAt.IsZero() {
			fmt.Fprintf(&b, "  %-16s %s\n", "Resume at:", r.ProviderResumeAt.Format(time.RFC3339))
		}
	}

	if r.Trend != nil {
		fmt.Fprintf(&b, "  %-16s %s\n", "Trend:", r.Trend.Message)
	}

	return b.String()
}

// FormatJSON returns the status report as a compact JSON string.
func (r StatusReport) FormatJSON() string {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}
