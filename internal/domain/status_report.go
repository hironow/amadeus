package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StatusReport holds operational status information for the amadeus tool.
type StatusReport struct {
	LastCheck    time.Time `json:"last_check"`
	Divergence   float64   `json:"divergence"`
	CheckCount   int       `json:"check_count"`
	InboxCount   int       `json:"inbox_count"`
	ArchiveCount int       `json:"archive_count"`
	SuccessRate  float64   `json:"success_rate"`
	Convergences int       `json:"convergences"`
}

// FormatText returns a human-readable status report string suitable for stderr.
func (r StatusReport) FormatText() string {
	var b strings.Builder
	b.WriteString("amadeus status:\n")

	// Last check
	if r.LastCheck.IsZero() {
		b.WriteString("  Last check:    no checks yet\n")
	} else {
		b.WriteString(fmt.Sprintf("  Last check:    %s\n", r.LastCheck.Format(time.RFC3339)))
	}

	// Divergence
	b.WriteString(fmt.Sprintf("  Divergence:    %.2f\n", r.Divergence))

	// Checks
	b.WriteString(fmt.Sprintf("  Checks:        %d total\n", r.CheckCount))

	// Success rate
	if r.CheckCount == 0 {
		b.WriteString("  Success rate:  no events\n")
	} else {
		b.WriteString(fmt.Sprintf("  Success rate:  %.1f%%\n", r.SuccessRate*100))
	}

	// Inbox
	b.WriteString(fmt.Sprintf("  Inbox:         %d pending\n", r.InboxCount))

	// Archive
	b.WriteString(fmt.Sprintf("  Archive:       %d processed\n", r.ArchiveCount))

	// Convergences
	b.WriteString(fmt.Sprintf("  Convergences:  %d active\n", r.Convergences))

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
