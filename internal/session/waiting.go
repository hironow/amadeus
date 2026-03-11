package session

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// WaitForDMail blocks until a D-Mail arrives, the timeout expires, or the context is cancelled.
// Returns (true, nil) if a D-Mail arrived, (false, nil) on timeout or cancellation.
func WaitForDMail(ctx context.Context, inboxCh <-chan domain.DMail, timeout time.Duration, logger domain.Logger) (arrived bool, err error) {
	logger.OK("Check completed. Entering D-Mail waiting mode.")
	if timeout > 0 {
		logger.Info("Waiting for incoming D-Mails... (timeout: %s)", timeout)
	} else {
		logger.Info("Waiting for incoming D-Mails... (no timeout)")
	}
	logger.Info("Press Ctrl+C to exit.")

	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}

	select {
	case <-ctx.Done():
		return false, nil
	case <-timer:
		logger.Info("No D-Mails received for %s. Exiting.", timeout)
		return false, nil
	case _, ok := <-inboxCh:
		if !ok {
			return false, nil
		}
		return true, nil
	}
}
