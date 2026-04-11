package cmd_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestExitError_Code(t *testing.T) {
	// given
	err := &cmd.ExitError{Code: 130}

	// then
	if err.Code != 130 {
		t.Errorf("Code = %d, want 130", err.Code)
	}
}

func TestExitError_Error(t *testing.T) {
	// given
	err := &cmd.ExitError{Code: 1, Err: fmt.Errorf("something failed")}

	// then
	if err.Error() != "something failed" {
		t.Errorf("Error() = %q, want %q", err.Error(), "something failed")
	}
}

func TestExitError_Unwrap(t *testing.T) {
	// given
	inner := fmt.Errorf("inner cause")
	err := &cmd.ExitError{Code: 2, Err: inner}

	// then
	if !errors.Is(err, inner) {
		t.Error("errors.Is should find inner error")
	}
}

func TestExitError_ExtractFromChain(t *testing.T) {
	// given
	inner := &cmd.ExitError{Code: 130, Err: fmt.Errorf("interrupted")}
	wrapped := fmt.Errorf("run failed: %w", inner)

	// when
	var exitErr *cmd.ExitError
	found := errors.As(wrapped, &exitErr)

	// then
	if !found {
		t.Fatal("errors.As should find ExitError in chain")
	}
	if exitErr.Code != 130 {
		t.Errorf("Code = %d, want 130", exitErr.Code)
	}
}
