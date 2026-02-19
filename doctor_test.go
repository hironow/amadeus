package amadeus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCheckStatusLabel(t *testing.T) {
	tests := []struct {
		status CheckStatus
		want   string
	}{
		{CheckOK, "OK"},
		{CheckFail, "FAIL"},
		{CheckSkip, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.status.StatusLabel(); got != tt.want {
			t.Errorf("StatusLabel(%d): expected %q, got %q", tt.status, tt.want, got)
		}
	}
}

func TestCheckTool_Exists(t *testing.T) {
	ctx := context.Background()
	result := checkTool(ctx, "git")
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK for 'git', got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git") {
		t.Errorf("expected message to contain path, got: %s", result.Message)
	}
}

func TestCheckTool_NotFound(t *testing.T) {
	ctx := context.Background()
	result := checkTool(ctx, "nonexistent-tool-xyz-12345")
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "command not found" {
		t.Errorf("expected 'command not found', got: %s", result.Message)
	}
}

func TestCheckGitRepo_InRepo(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	result := checkGitRepo(dir)
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGitRepo_NotRepo(t *testing.T) {
	dir := t.TempDir()
	result := checkGitRepo(dir)
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDivergenceDir_Exists(t *testing.T) {
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".divergence")
	os.MkdirAll(divRoot, 0o755)
	result := checkDivergenceDir(dir)
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDivergenceDir_NotExist(t *testing.T) {
	dir := t.TempDir()
	result := checkDivergenceDir(dir)
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(path, data, 0o644)
	result := checkConfig(path)
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_NotFound(t *testing.T) {
	result := checkConfig("/nonexistent/config.yaml")
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`{{{invalid`), 0o644)
	result := checkConfig(path)
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}
