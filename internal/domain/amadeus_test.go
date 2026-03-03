package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestDriftError_IsDrift(t *testing.T) {
	// given: a DriftError
	err := &domain.DriftError{Divergence: 0.35, DMails: 2}

	// then: it implements error and is detectable via errors.As
	var de *domain.DriftError
	if !errors.As(err, &de) {
		t.Fatal("expected errors.As to match DriftError")
	}
	if de.Divergence != 0.35 {
		t.Errorf("expected divergence 0.35, got %f", de.Divergence)
	}
	if de.DMails != 2 {
		t.Errorf("expected 2 dmails, got %d", de.DMails)
	}
}

func TestDriftError_Message(t *testing.T) {
	err := &domain.DriftError{Divergence: 0.35, DMails: 1}
	msg := err.Error()
	if !strings.Contains(msg, "drift") {
		t.Errorf("expected 'drift' in error message, got: %s", msg)
	}
}

func TestDriftError_NotConfusedWithRegularError(t *testing.T) {
	// given: a regular error
	err := fmt.Errorf("something broke")

	// then: errors.As should NOT match DriftError
	var de *domain.DriftError
	if errors.As(err, &de) {
		t.Fatal("regular error should not match DriftError")
	}
}

func TestExitCode_Nil(t *testing.T) {
	if code := domain.ExitCode(nil); code != 0 {
		t.Errorf("expected 0 for nil, got %d", code)
	}
}

func TestExitCode_DriftError(t *testing.T) {
	err := &domain.DriftError{Divergence: 0.35, DMails: 2}
	if code := domain.ExitCode(err); code != 2 {
		t.Errorf("expected 2 for DriftError, got %d", code)
	}
}

func TestExitCode_RegularError(t *testing.T) {
	err := fmt.Errorf("something broke")
	if code := domain.ExitCode(err); code != 1 {
		t.Errorf("expected 1 for regular error, got %d", code)
	}
}

func TestExitCode_WrappedDriftError(t *testing.T) {
	// given: DriftError wrapped in fmt.Errorf
	inner := &domain.DriftError{Divergence: 0.50, DMails: 3}
	err := fmt.Errorf("check failed: %w", inner)

	// then: should still detect drift via errors.As
	if code := domain.ExitCode(err); code != 2 {
		t.Errorf("expected 2 for wrapped DriftError, got %d", code)
	}
}
