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
