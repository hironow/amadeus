package main

import (
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
