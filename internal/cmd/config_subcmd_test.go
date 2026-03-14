package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
	"github.com/spf13/cobra"
)

func findSubcommand(root *cobra.Command, name string) *cobra.Command {
	for _, sub := range root.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

func TestConfigCmd_ShowSubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("config subcommand not found")
	}

	// then
	showCmd := findSubcommand(configCmd, "show")
	if showCmd == nil {
		t.Fatal("config show subcommand not found")
	}
}

func TestConfigCmd_SetSubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("config subcommand not found")
	}

	// then
	setCmd := findSubcommand(configCmd, "set")
	if setCmd == nil {
		t.Fatal("config set subcommand not found")
	}
}

func TestConfigCmd_SetWithKeyValue_OnInitializedProject(t *testing.T) {
	// given: initialized .gate/ with config
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte(`lang: "ja"`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "set", "lang", "en", dir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}
}

func TestConfigCmd_ShowDisplaysConfig_OnInitializedProject(t *testing.T) {
	// given: initialized .gate/ with config
	dir := t.TempDir()
	gateDir := filepath.Join(dir, ".gate")
	if err := os.MkdirAll(gateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gateDir, "config.yaml"), []byte("lang: en\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"config", "show", dir})

	// when
	err := root.Execute()

	// then
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "lang: en") {
		t.Errorf("expected 'lang: en' in output, got:\n%s", stdout.String())
	}
}
