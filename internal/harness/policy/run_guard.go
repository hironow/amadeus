package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/usecase/port"
)

// RunGuard prevents duplicate runs using persistent cross-process locking.
// When a run is in progress, other processes attempting the same run key
// are rejected with a descriptive reason.
type RunGuard struct {
	lockStore port.RunLockStore
	holderID  string
}

// NewRunGuard creates a run guard backed by the given lock store.
// holderID identifies this process uniquely (typically a UUID).
func NewRunGuard(lockStore port.RunLockStore, holderID string) *RunGuard {
	return &RunGuard{
		lockStore: lockStore,
		holderID:  holderID,
	}
}

// AllowRun attempts to acquire the run lock for the given key.
// Returns (true, "", nil) if the lock was acquired.
// Returns (false, reason, nil) if the lock is held by another process.
func (g *RunGuard) AllowRun(ctx context.Context, runKey string, ttl time.Duration) (bool, string, error) {
	acquired, holder, err := g.lockStore.TryAcquire(ctx, runKey, ttl)
	if err != nil {
		return false, "", fmt.Errorf("run guard: %w", err)
	}
	if !acquired {
		return false, fmt.Sprintf("run locked by %s", holder), nil
	}
	return true, "", nil
}

// ReleaseRun releases the run lock for the given key.
func (g *RunGuard) ReleaseRun(ctx context.Context, runKey string) error {
	return g.lockStore.Release(ctx, runKey, g.holderID)
}
