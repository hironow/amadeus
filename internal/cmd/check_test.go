package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// === P1-7: Approval gate flags ===

func TestCheckCmd_AutoApproveFlag(t *testing.T) {
	// given
	root := NewRootCommand()
	checkCmd, _, _ := root.Find([]string{"check"})

	// then: --auto-approve flag should exist
	flag := checkCmd.Flags().Lookup("auto-approve")
	if flag == nil {
		t.Fatal("expected --auto-approve flag on check command")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected default false, got %q", flag.DefValue)
	}
}

func TestCheckCmd_ApproveCmdFlag(t *testing.T) {
	// given
	root := NewRootCommand()
	checkCmd, _, _ := root.Find([]string{"check"})

	// then: --approve-cmd flag should exist
	flag := checkCmd.Flags().Lookup("approve-cmd")
	if flag == nil {
		t.Fatal("expected --approve-cmd flag on check command")
	}
}

func TestCheckCmd_NotifyCmdFlag(t *testing.T) {
	// given
	root := NewRootCommand()
	checkCmd, _, _ := root.Find([]string{"check"})

	// then: --notify-cmd flag should exist
	flag := checkCmd.Flags().Lookup("notify-cmd")
	if flag == nil {
		t.Fatal("expected --notify-cmd flag on check command")
	}
}

func TestCheckCmd_FailsWithoutInit(t *testing.T) {
	// given: empty directory with no .gate/
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"check"})

	// when
	execErr := cmd.Execute()

	// then: should fail with init guidance
	if execErr == nil {
		t.Fatal("expected error for uninitialized state, got nil")
	}
	got := execErr.Error()
	if !strings.Contains(got, "init") {
		t.Errorf("expected error to mention 'init', got: %s", got)
	}
}
