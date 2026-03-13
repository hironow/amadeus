package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// tryWriteHandover writes a handover document if the error is due to context
// cancellation and the outer (shutdown) context is still alive. Returns the
// original error unchanged.
func tryWriteHandover(ctx context.Context, err error, repoRoot string, inProgress string, logger domain.Logger) error {
	if err == nil || !errors.Is(err, context.Canceled) {
		return err
	}
	outerCtx, ok := ctx.Value(domain.ShutdownKey).(context.Context)
	if !ok || outerCtx.Err() != nil {
		return err
	}

	hw := &session.FileHandoverWriter{}
	state := domain.HandoverState{
		Tool:       "amadeus",
		Operation:  "divergence",
		Timestamp:  time.Now(),
		InProgress: inProgress,
	}
	stateDir := filepath.Join(repoRoot, domain.StateDir)
	if hwErr := hw.WriteHandover(outerCtx, stateDir, state); hwErr != nil {
		if logger != nil {
			logger.Warn("handover write failed: %v", hwErr)
		}
	} else if logger != nil {
		logger.Info("Handover written to %s/handover.md", stateDir)
	}
	return err
}
