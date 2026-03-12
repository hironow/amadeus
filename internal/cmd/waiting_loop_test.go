package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestRunWaitingLoop_SkipsCheckOnConsecutiveNoDrift(t *testing.T) {
	// given: checkFn always returns nil (no drift), waitFn delivers 10 times then stops
	waitCalls := 0
	waitFn := func(_ context.Context) (bool, error) {
		waitCalls++
		if waitCalls > 10 {
			return false, nil
		}
		return true, nil
	}
	checkCalls := 0
	checkFn := func(_ context.Context) error {
		checkCalls++
		return nil // no drift
	}

	// when
	err := runWaitingLoop(context.Background(), checkFn, waitFn, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checkCalls != maxConsecutiveNoDrift {
		t.Errorf("expected %d checks (maxConsecutiveNoDrift cap), got %d", maxConsecutiveNoDrift, checkCalls)
	}
	// 10 arrived + 1 final "not arrived" = 11 total calls
	if waitCalls != 11 {
		t.Errorf("expected 11 wait calls, got %d", waitCalls)
	}
}

func TestRunWaitingLoop_DriftResetsCounter(t *testing.T) {
	// given: checkFn returns drift on 2nd call, waitFn delivers 8 times
	// Sequence of check calls:
	//   call 1: no drift (counter=1)
	//   call 2: drift!  (counter reset to 0)
	//   call 3: no drift (counter=1)
	//   call 4: no drift (counter=2)
	//   call 5: no drift (counter=3)
	//   arrivals 6-8: skipped (counter >= 3)
	// Total: 5 checks, 9 wait calls (8 arrived + 1 not-arrived)
	waitCalls := 0
	waitFn := func(_ context.Context) (bool, error) {
		waitCalls++
		if waitCalls > 8 {
			return false, nil
		}
		return true, nil
	}
	checkCalls := 0
	checkFn := func(_ context.Context) error {
		checkCalls++
		if checkCalls == 2 {
			return &domain.DriftError{Divergence: 0.5, DMails: 1}
		}
		return nil
	}

	// when
	err := runWaitingLoop(context.Background(), checkFn, waitFn, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 no-drift + 1 drift (reset) + 3 no-drift = 5 checks
	if checkCalls != 5 {
		t.Errorf("expected 5 checks, got %d", checkCalls)
	}
	if waitCalls != 9 {
		t.Errorf("expected 9 wait calls, got %d", waitCalls)
	}
}

func TestRunWaitingLoop_FatalErrorPropagates(t *testing.T) {
	// given: checkFn returns a non-drift error
	waitFn := func(_ context.Context) (bool, error) {
		return true, nil
	}
	fatalErr := fmt.Errorf("connection lost")
	checkFn := func(_ context.Context) error {
		return fatalErr
	}

	// when
	err := runWaitingLoop(context.Background(), checkFn, waitFn, &domain.NopLogger{})

	// then
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != fatalErr {
		t.Errorf("expected fatal error %q, got %q", fatalErr, err)
	}
}

func TestRunWaitingLoop_WaitErrorPropagates(t *testing.T) {
	// given: waitFn returns an error
	waitErr := fmt.Errorf("watch failed")
	waitFn := func(_ context.Context) (bool, error) {
		return false, waitErr
	}
	checkFn := func(_ context.Context) error {
		t.Fatal("checkFn should not be called when waitFn errors")
		return nil
	}

	// when
	err := runWaitingLoop(context.Background(), checkFn, waitFn, &domain.NopLogger{})

	// then
	if err != waitErr {
		t.Errorf("expected wait error %q, got %q", waitErr, err)
	}
}

func TestRunWaitingLoop_ImmediateNotArrived(t *testing.T) {
	// given: waitFn immediately returns not-arrived
	waitFn := func(_ context.Context) (bool, error) {
		return false, nil
	}
	checkFn := func(_ context.Context) error {
		t.Fatal("checkFn should not be called when no D-Mail arrives")
		return nil
	}

	// when
	err := runWaitingLoop(context.Background(), checkFn, waitFn, &domain.NopLogger{})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
