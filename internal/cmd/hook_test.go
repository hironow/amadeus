package cmd_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/cmd"
)

func TestInstallHookCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "install-hook" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Fatal("install-hook subcommand not found")
	}
}

func TestInstallHookCmd_NoArgsAllowed(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install-hook", "extra-arg"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when args are provided (cobra.NoArgs)")
	}
}

func TestInstallHookCmd_FailsOutsideGitRepo(t *testing.T) {
	// given: temp dir that is NOT a git repository
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install-hook"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error outside git repo")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error to mention 'git', got: %s", err.Error())
	}
}

func TestUninstallHookCmd_SubcommandExists(t *testing.T) {
	// given
	root := cmd.NewRootCommand()

	// when
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "uninstall-hook" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Fatal("uninstall-hook subcommand not found")
	}
}

func TestUninstallHookCmd_NoArgsAllowed(t *testing.T) {
	// given
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"uninstall-hook", "extra-arg"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when args are provided (cobra.NoArgs)")
	}
}

func TestUninstallHookCmd_FailsOutsideGitRepo(t *testing.T) {
	// given: temp dir that is NOT a git repository
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"uninstall-hook"})

	// when
	err := root.Execute()

	// then
	if err == nil {
		t.Fatal("expected error outside git repo")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error to mention 'git', got: %s", err.Error())
	}
}
