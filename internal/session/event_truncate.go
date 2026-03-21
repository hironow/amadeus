package session

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/eventsource"
)

// TruncateOversizedEventFiles truncates event files exceeding the size threshold,
// keeping only the most recent lines in each.
func TruncateOversizedEventFiles(stateDir string, logger domain.Logger) {
	oversized, err := eventsource.ListOversizedEventFiles(stateDir)
	if err != nil {
		logger.Warn("list oversized event files: %v", err)
		return
	}
	for _, name := range oversized {
		if truncErr := eventsource.TruncateEventFile(stateDir, name, eventsource.EventFileTruncateKeepLines); truncErr != nil {
			logger.Warn("truncate event file %s: %v", name, truncErr)
		} else {
			logger.Info("Truncated oversized event file: %s", name)
		}
	}
}
