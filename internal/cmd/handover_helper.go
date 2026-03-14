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
func tryWriteHandover(ctx context.Context, err error, repoDir string, state domain.HandoverState, logger domain.Logger) error {
	if err == nil || !errors.Is(err, context.Canceled) {
		return err
	}
	outerCtx, ok := ctx.Value(domain.ShutdownKey).(context.Context)
	if !ok || outerCtx.Err() != nil {
		return err
	}

	hw := &session.FileHandoverWriter{}
	state.Timestamp = time.Now()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if hwErr := hw.WriteHandover(outerCtx, stateDir, state); hwErr != nil {
		if logger != nil {
			logger.Warn("handover write failed: %v", hwErr)
		}
	} else if logger != nil {
		logger.Info("Handover written to %s/handover.md", stateDir)
	}
	return err
}

// writeHandoverOnCancel writes a handover document when ctx is cancelled
// but the error is nil (e.g. WaitForDMail returns (false, nil) on cancel).
func writeHandoverOnCancel(ctx context.Context, repoDir string, state domain.HandoverState, logger domain.Logger) {
	if ctx.Err() != context.Canceled {
		return
	}
	outerCtx, ok := ctx.Value(domain.ShutdownKey).(context.Context)
	if !ok || outerCtx.Err() != nil {
		return
	}
	hw := &session.FileHandoverWriter{}
	state.Timestamp = time.Now()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if hwErr := hw.WriteHandover(outerCtx, stateDir, state); hwErr != nil {
		if logger != nil {
			logger.Warn("handover write failed: %v", hwErr)
		}
	} else if logger != nil {
		logger.Info("Handover written to %s/handover.md", stateDir)
	}
}
