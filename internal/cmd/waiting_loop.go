package cmd

import (
	"context"
	"errors"

	"github.com/hironow/amadeus/internal/domain"
)

// maxConsecutiveNoDrift is the number of consecutive no-drift check results
// after which the loop skips the expensive RunCheck call. The channel consumer
// (waitFn) is always called to keep MonitorInbox drained.
const maxConsecutiveNoDrift = 3

// runWaitingLoop drives the D-Mail waiting loop. It always calls waitFn
// (draining the inbox channel promptly) but skips checkFn after
// maxConsecutiveNoDrift consecutive no-drift results. A DriftError from
// checkFn resets the counter (real work happened). Any other error from
// checkFn is treated as fatal and returned immediately.
func runWaitingLoop(
	ctx context.Context,
	checkFn func(context.Context) error,
	waitFn func(context.Context) (bool, error),
	logger domain.Logger,
) error {
	var consecutiveNoDrift int

	for {
		arrived, waitErr := waitFn(ctx)
		if waitErr != nil {
			return waitErr
		}
		if !arrived {
			return nil // timeout or cancel → clean exit
		}

		if consecutiveNoDrift >= maxConsecutiveNoDrift {
			logger.Info("Skipping check — no drift change for %d consecutive cycles", consecutiveNoDrift)
			consecutiveNoDrift++
			continue
		}

		checkErr := checkFn(ctx)
		if checkErr != nil {
			var de *domain.DriftError
			if errors.As(checkErr, &de) {
				// Drift found — real work happened, reset counter
				consecutiveNoDrift = 0
				continue
			}
			// Non-drift error is fatal
			return checkErr
		}

		// No drift — increment counter
		consecutiveNoDrift++
	}
}
