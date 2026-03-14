package cmd_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestRebuildCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// then
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "rebuild" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("rebuild subcommand not found")
	}
}
