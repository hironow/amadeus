package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseShellCommand_SimpleCommand(t *testing.T) {
	env, bin, args := ParseShellCommand("claude")
	if len(env) != 0 {
		t.Errorf("env: want empty, got %v", env)
	}
	if bin != "claude" {
		t.Errorf("bin: want claude, got %q", bin)
	}
	if len(args) != 0 {
		t.Errorf("args: want empty, got %v", args)
	}
}

func TestParseShellCommand_AbsolutePath(t *testing.T) {
	env, bin, args := ParseShellCommand("/usr/local/bin/claude")
	if len(env) != 0 {
		t.Errorf("env: want empty, got %v", env)
	}
	if bin != "/usr/local/bin/claude" {
		t.Errorf("bin: want /usr/local/bin/claude, got %q", bin)
	}
	if len(args) != 0 {
		t.Errorf("args: want empty, got %v", args)
	}
}

func TestParseShellCommand_TildePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	env, bin, _ := ParseShellCommand("~/.local/bin/claude")
	if len(env) != 0 {
		t.Errorf("env: want empty, got %v", env)
	}
	want := filepath.Join(home, ".local/bin/claude")
	if bin != want {
		t.Errorf("bin: want %q, got %q", want, bin)
	}
}

func TestParseShellCommand_EnvVarPrefix(t *testing.T) {
	home, _ := os.UserHomeDir()
	env, bin, args := ParseShellCommand("CLAUDE_CONFIG_DIR=~/.claude-work-b ~/.local/bin/claude")

	if len(env) != 1 {
		t.Fatalf("env: want 1, got %d: %v", len(env), env)
	}
	wantEnv := "CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude-work-b")
	if env[0] != wantEnv {
		t.Errorf("env[0]: want %q, got %q", wantEnv, env[0])
	}
	wantBin := filepath.Join(home, ".local/bin/claude")
	if bin != wantBin {
		t.Errorf("bin: want %q, got %q", wantBin, bin)
	}
	if len(args) != 0 {
		t.Errorf("args: want empty, got %v", args)
	}
}

func TestParseShellCommand_MultipleEnvVars(t *testing.T) {
	env, bin, _ := ParseShellCommand("FOO=bar BAZ=qux mycommand")
	if len(env) != 2 {
		t.Fatalf("env: want 2, got %d", len(env))
	}
	if env[0] != "FOO=bar" || env[1] != "BAZ=qux" {
		t.Errorf("env: want [FOO=bar BAZ=qux], got %v", env)
	}
	if bin != "mycommand" {
		t.Errorf("bin: want mycommand, got %q", bin)
	}
}

func TestParseShellCommand_Empty(t *testing.T) {
	env, bin, args := ParseShellCommand("")
	if len(env) != 0 || bin != "" || len(args) != 0 {
		t.Errorf("want all empty, got env=%v bin=%q args=%v", env, bin, args)
	}
}

func TestExpandTilde_NoTilde(t *testing.T) {
	if got := ExpandTilde("/usr/bin/foo"); got != "/usr/bin/foo" {
		t.Errorf("want /usr/bin/foo, got %q", got)
	}
}

func TestExpandTilde_EnvValue(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := ExpandTilde("KEY=~/value")
	want := "KEY=" + filepath.Join(home, "value")
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestIsEnvKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"FOO", true},
		{"_BAR", true},
		{"foo123", true},
		{"CLAUDE_CONFIG_DIR", true},
		{"123BAD", false},
		{"-nope", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isEnvKey(tt.key); got != tt.want {
			t.Errorf("isEnvKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestLookPathShell_SimpleCommand(t *testing.T) {
	// "echo" should be found on any system
	path, err := LookPathShell("echo")
	if err != nil {
		t.Fatalf("LookPathShell(echo): %v", err)
	}
	if path == "" {
		t.Error("want non-empty path")
	}
}

func TestLookPathShell_WithEnvPrefix(t *testing.T) {
	// Even with env prefix, should find the binary
	path, err := LookPathShell("FOO=bar echo")
	if err != nil {
		t.Fatalf("LookPathShell(FOO=bar echo): %v", err)
	}
	if path == "" {
		t.Error("want non-empty path")
	}
}
