package cmd

import (
	"strings"
	"testing"
)

func TestArchivePrune_NegativeDays(t *testing.T) {
	// given
	root := NewRootCommand(BuildInfo{Version: "test"})
	root.SetArgs([]string{"archive-prune", "--days", "-5"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for negative --days")
	}
	if !strings.Contains(err.Error(), "--days must be >= 1") {
		t.Errorf("expected '--days must be >= 1' in error, got: %v", err)
	}
}

func TestArchivePrune_ZeroDays(t *testing.T) {
	// given
	root := NewRootCommand(BuildInfo{Version: "test"})
	root.SetArgs([]string{"archive-prune", "--days", "0"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for --days 0")
	}
	if !strings.Contains(err.Error(), "--days must be >= 1") {
		t.Errorf("expected '--days must be >= 1' in error, got: %v", err)
	}
}
