package session

import (
	"github.com/hironow/amadeus/internal/domain"
)

// ExtractStallEscalations filters stall-escalation D-Mails from a batch.
// Returns only D-Mails with KindStallEscalation for dedicated handling.
func ExtractStallEscalations(dmails []domain.DMail) []domain.DMail {
	var stalls []domain.DMail
	for _, d := range dmails {
		if d.Kind == domain.KindStallEscalation {
			stalls = append(stalls, d)
		}
	}
	return stalls
}

// HandleStallEscalations processes stall-escalation D-Mails received from sightjack.
// Logs each stall event with metadata for operator visibility.
// Returns the count of stalls handled.
func HandleStallEscalations(stalls []domain.DMail, logger domain.Logger) int {
	for _, s := range stalls {
		waveID := s.Metadata["wave_id"]
		cluster := s.Metadata["cluster_name"]
		fp := s.Metadata["error_fingerprint"]
		count := s.Metadata["failure_count"]
		logger.Warn("[STALL] Wave %s:%s stalled (fingerprint=%s, failures=%s): %s",
			cluster, waveID, fp, count, s.Description)
	}
	return len(stalls)
}
