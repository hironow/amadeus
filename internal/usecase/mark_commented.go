package usecase

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
)

// MarkCommented records that a D-Mail x Issue pair has been posted as a comment.
// Constructs an Amadeus orchestrator internally with minimal wiring.
func MarkCommented(gateDir string, logger *domain.Logger, dmailName, issueID string) error {
	result, err := buildAmadeus(AmadeusParams{
		GateDir: gateDir,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("build amadeus: %w", err)
	}
	defer result.Cleanup()

	return result.Amadeus.MarkCommented(dmailName, issueID)
}
