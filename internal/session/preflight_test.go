package session

import (
	"strings"
	"testing"
)

func TestPreflightCheck_ExistingBinary(t *testing.T) {
	// given: "go" should always exist in test environment
	// when
	err := PreflightCheck("go")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreflightCheck_MissingBinary(t *testing.T) {
	// given: a binary that should not exist
	// when
	err := PreflightCheck("nonexistent-binary-xyz-123")

	// then
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("expected 'not found in PATH' in error, got: %v", err)
	}
}

func TestPreflightCheck_MultipleBinaries(t *testing.T) {
	// given: first binary exists, second does not
	// when
	err := PreflightCheck("go", "nonexistent-binary-xyz-123")

	// then: should fail on the missing binary
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "nonexistent-binary-xyz-123") {
		t.Errorf("expected binary name in error, got: %v", err)
	}
}

func TestPreflightCheck_NoBinaries(t *testing.T) {
	// given: no binaries to check
	// when
	err := PreflightCheck()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
