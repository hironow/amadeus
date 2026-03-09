package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"testing"
)

func TestNeedsDefaultRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		// No args → default to run
		{"empty args", nil, true},
		{"empty slice", []string{}, true},

		// Explicit subcommands → no default
		{"explicit run", []string{"run"}, false},
		{"explicit check", []string{"check"}, false},
		{"explicit init", []string{"init"}, false},
		{"explicit validate", []string{"validate"}, false},
		{"explicit install-hook", []string{"install-hook"}, false},
		{"explicit uninstall-hook", []string{"uninstall-hook"}, false},
		{"explicit log", []string{"log"}, false},
		{"explicit doctor", []string{"doctor"}, false},
		{"explicit sync", []string{"sync"}, false},
		{"explicit mark-commented", []string{"mark-commented"}, false},
		{"explicit archive-prune", []string{"archive-prune"}, false},
		{"explicit clean", []string{"clean"}, false},
		{"explicit rebuild", []string{"rebuild"}, false},
		{"explicit version", []string{"version"}, false},
		{"explicit update", []string{"update"}, false},
		{"explicit help", []string{"help"}, false},
		{"explicit completion", []string{"completion"}, false},

		// Root flags that suppress default
		{"--version", []string{"--version"}, false},
		{"--help", []string{"--help"}, false},
		{"-h", []string{"-h"}, false},

		// Persistent flags before subcommand → still finds subcommand
		{"verbose then check", []string{"-v", "check"}, false},
		{"config then check", []string{"-c", "cfg.yaml", "check"}, false},
		{"config=val then check", []string{"-c=cfg.yaml", "check"}, false},
		{"lang then check", []string{"-l", "ja", "check"}, false},
		{"output then check", []string{"-o", "json", "check"}, false},

		// Persistent flags only → default to run
		{"verbose only", []string{"-v"}, true},
		{"config only", []string{"-c", "cfg.yaml"}, true},
		{"long verbose only", []string{"--verbose"}, true},
		{"long config only", []string{"--config", "cfg.yaml"}, true},
		{"lang only", []string{"--lang", "ja"}, true},
		{"output only", []string{"--output", "json"}, true},

		// Unknown flags → default to run
		{"unknown flag", []string{"--some-flag"}, true},

		// -- terminator
		{"double dash only", []string{"--"}, true},
		{"double dash then subcommand", []string{"--", "check"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewRootCommand()
			got := NeedsDefaultRun(rootCmd, tt.args)
			if got != tt.want {
				t.Errorf("NeedsDefaultRun(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
