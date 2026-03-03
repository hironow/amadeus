package usecase

import (
	"fmt"
	"io"

	"github.com/hironow/amadeus/internal/domain"
)

// RunLog prints the divergence log in human-readable format.
func RunLog(gateDir string, cfg domain.Config, dataOut io.Writer, logger *domain.Logger) error {
	result, err := buildAmadeus(AmadeusParams{
		GateDir: gateDir,
		Config:  cfg,
		DataOut: dataOut,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("build amadeus: %w", err)
	}
	defer result.Cleanup()

	return result.Amadeus.PrintLog()
}

// RunLogJSON prints the divergence log in JSON format.
func RunLogJSON(gateDir string, cfg domain.Config, dataOut io.Writer, logger *domain.Logger) error {
	result, err := buildAmadeus(AmadeusParams{
		GateDir: gateDir,
		Config:  cfg,
		DataOut: dataOut,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("build amadeus: %w", err)
	}
	defer result.Cleanup()

	return result.Amadeus.PrintLogJSON()
}
