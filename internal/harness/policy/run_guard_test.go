package policy_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/harness/policy"
)

// fakeRunLockStore is a test double for policy.RunLockAcquirer.
type fakeRunLockStore struct {
	held      bool
	holder    string
	acquireOK bool
}

func (f *fakeRunLockStore) TryAcquire(_ context.Context, _ string, _ time.Duration) (bool, string, error) {
	if f.acquireOK {
		f.held = true
		f.holder = "test-holder"
		return true, "test-holder", nil
	}
	return false, f.holder, nil
}

func (f *fakeRunLockStore) Release(_ context.Context, _ string, _ string) error {
	f.held = false
	return nil
}

func TestRunGuard_AllowRun_Succeeds(t *testing.T) {
	// given
	store := &fakeRunLockStore{acquireOK: true}
	guard := policy.NewRunGuard(store, "my-holder")

	// when
	allowed, reason, err := guard.AllowRun(context.Background(), "wave-1", 30*time.Minute)

	// then
	if err != nil {
		t.Fatalf("AllowRun: %v", err)
	}
	if !allowed {
		t.Errorf("expected allowed=true, reason=%s", reason)
	}
}

func TestRunGuard_AllowRun_BlockedByOtherHolder(t *testing.T) {
	// given
	store := &fakeRunLockStore{acquireOK: false, holder: "other-process"}
	guard := policy.NewRunGuard(store, "my-holder")

	// when
	allowed, reason, err := guard.AllowRun(context.Background(), "wave-1", 30*time.Minute)

	// then
	if err != nil {
		t.Fatalf("AllowRun: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestRunGuard_ReleaseRun(t *testing.T) {
	// given
	store := &fakeRunLockStore{acquireOK: true}
	guard := policy.NewRunGuard(store, "my-holder")
	guard.AllowRun(context.Background(), "wave-1", 30*time.Minute)

	// when
	err := guard.ReleaseRun(context.Background(), "wave-1")

	// then
	if err != nil {
		t.Fatalf("ReleaseRun: %v", err)
	}
}
