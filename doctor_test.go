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

func TestCheckLinearMCP_Connected(t *testing.T) {
	// given: mock claude mcp list output showing linear connected
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: https://mcp.linear.app/mcp (HTTP) - ✓ Connected")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, "claude")

	// then
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_NotConnected(t *testing.T) {
	// given: mock output without linear
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "some-other-mcp: https://example.com - ✓ Connected")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, "claude")

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_CommandFails(t *testing.T) {
	// given: claude mcp list fails
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, "claude")

	// then
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

func TestRunDoctor_ReturnsAllResults(t *testing.T) {
	// given: mock commands succeed
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - ✓ Connected")
	}
	defer func() { execCommand = exec.CommandContext }()

	dir := t.TempDir()
	// Create .divergence/ with config
	divRoot := filepath.Join(dir, ".divergence")
	os.MkdirAll(divRoot, 0o755)
	cfg := DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)

	// Init git repo
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := RunDoctor(ctx, configPath, dir)

	// then: should have 6 results
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}
	// Verify names in order
	expectedNames := []string{"git", "Git Repository", "claude", ".divergence/", "Config", "Linear MCP"}
	for i, name := range expectedNames {
		if results[i].Name != name {
			t.Errorf("result[%d]: expected name %q, got %q", i, name, results[i].Name)
		}
	}
}

func TestRunDoctor_ClaudeUnavailable_MCPSkipped(t *testing.T) {
	// given: no need to mock execCommand for this test
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".divergence")
	os.MkdirAll(divRoot, 0o755)
	cfg := DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when: pass a nonexistent claude command
	results := RunDoctorWithClaudeCmd(ctx, configPath, dir, "nonexistent-claude-xyz")

	// then
	var mcpResult DoctorCheckResult
	for _, r := range results {
		if r.Name == "Linear MCP" {
			mcpResult = r
			break
		}
	}
	if mcpResult.Status != CheckSkip {
		t.Errorf("expected Linear MCP SKIP when claude unavailable, got %v: %s", mcpResult.Status, mcpResult.Message)
	}
	if !strings.Contains(mcpResult.Message, "claude not available") {
		t.Errorf("expected 'claude not available' in message, got: %s", mcpResult.Message)
	}
}
