package main

import (
	"strings"
	"testing"
)

func TestParseResolveArgs_FlagsAfterName(t *testing.T) {
	// given
	args := []string{"feedback-001", "--approve"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.approve {
		t.Error("expected approve=true")
	}
	if result.reject {
		t.Error("expected reject=false")
	}
	if len(result.names) != 1 || result.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", result.names)
	}
}

func TestParseResolveArgs_FlagsBeforeName(t *testing.T) {
	// given
	args := []string{"--approve", "feedback-001"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.approve {
		t.Error("expected approve=true")
	}
	if len(result.names) != 1 || result.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", result.names)
	}
}

func TestParseResolveArgs_RejectWithReason(t *testing.T) {
	// given
	args := []string{"feedback-001", "--reject", "--reason", "not aligned"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.reject {
		t.Error("expected reject=true")
	}
	if result.reason != "not aligned" {
		t.Errorf("expected reason='not aligned', got %q", result.reason)
	}
	if len(result.names) != 1 || result.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", result.names)
	}
}

func TestParseResolveArgs_RejectWithReasonEquals(t *testing.T) {
	// given
	args := []string{"feedback-001", "--reject", "--reason=not aligned"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.reject {
		t.Error("expected reject=true")
	}
	if result.reason != "not aligned" {
		t.Errorf("expected reason='not aligned', got %q", result.reason)
	}
}

func TestParseResolveArgs_MultipleNames(t *testing.T) {
	// given
	args := []string{"feedback-001", "feedback-002", "--approve"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.approve {
		t.Error("expected approve=true")
	}
	if len(result.names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(result.names), result.names)
	}
	if result.names[0] != "feedback-001" || result.names[1] != "feedback-002" {
		t.Errorf("expected [feedback-001, feedback-002], got %v", result.names)
	}
}

func TestParseResolveArgs_NoFlags(t *testing.T) {
	// given
	args := []string{"feedback-001"}

	// when
	result := parseResolveArgs(args)

	// then
	if result.approve || result.reject {
		t.Error("expected both approve and reject to be false")
	}
	if len(result.names) != 1 || result.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", result.names)
	}
}

func TestParseResolveArgs_Empty(t *testing.T) {
	// given
	args := []string{}

	// when
	result := parseResolveArgs(args)

	// then
	if len(result.names) != 0 {
		t.Errorf("expected empty names, got %v", result.names)
	}
}

func TestParseResolveArgs_CommonFlags(t *testing.T) {
	// given: common flags mixed with resolve-specific flags
	args := []string{"--approve", "-v", "--json", "-c", "custom.yaml", "feedback-001"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.approve {
		t.Error("expected approve=true")
	}
	if !result.verbose {
		t.Error("expected verbose=true")
	}
	if !result.jsonOut {
		t.Error("expected jsonOut=true")
	}
	if result.configPath != "custom.yaml" {
		t.Errorf("expected configPath='custom.yaml', got %q", result.configPath)
	}
	if len(result.names) != 1 || result.names[0] != "feedback-001" {
		t.Errorf("expected names=[feedback-001], got %v", result.names)
	}
}

func TestRunArchivePrune_NegativeDays(t *testing.T) {
	// when: --days with negative value
	err := runArchivePrune([]string{"--days", "-5"})

	// then: should reject
	if err == nil {
		t.Fatal("expected error for negative --days")
	}
	if !strings.Contains(err.Error(), "--days must be >= 1") {
		t.Errorf("expected '--days must be >= 1' in error, got: %v", err)
	}
}

func TestRunArchivePrune_ZeroDays(t *testing.T) {
	// when: --days 0
	err := runArchivePrune([]string{"--days", "0"})

	// then: should reject
	if err == nil {
		t.Fatal("expected error for --days 0")
	}
	if !strings.Contains(err.Error(), "--days must be >= 1") {
		t.Errorf("expected '--days must be >= 1' in error, got: %v", err)
	}
}

func TestParseResolveArgs_CommonFlagsLongForm(t *testing.T) {
	// given: long-form common flags
	args := []string{"--verbose", "--config", "path.yaml", "feedback-001", "--reject", "--reason", "bad"}

	// when
	result := parseResolveArgs(args)

	// then
	if !result.reject {
		t.Error("expected reject=true")
	}
	if !result.verbose {
		t.Error("expected verbose=true")
	}
	if result.configPath != "path.yaml" {
		t.Errorf("expected configPath='path.yaml', got %q", result.configPath)
	}
	if result.reason != "bad" {
		t.Errorf("expected reason='bad', got %q", result.reason)
	}
}
