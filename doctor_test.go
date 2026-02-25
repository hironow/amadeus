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

func TestCheckGateDir_Exists(t *testing.T) {
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	result := checkGateDir(dir)
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGateDir_NotExist(t *testing.T) {
	dir := t.TempDir()
	result := checkGateDir(dir)
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

func TestCheckLinearMCP_Disconnected(t *testing.T) {
	// given: mock output showing linear as disconnected
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: https://mcp.linear.app/mcp (HTTP) - ✗ Disconnected")
	}
	defer func() { execCommand = exec.CommandContext }()

	ctx := context.Background()

	// when
	result := checkLinearMCP(ctx, "claude")

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail for disconnected, got %v: %s", result.Status, result.Message)
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
	// Create .gate/ with config
	divRoot := filepath.Join(dir, ".gate")
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

	// then: should have 9 results
	if len(results) != 9 {
		t.Fatalf("expected 9 results, got %d", len(results))
	}
	// Verify names in order
	expectedNames := []string{"git", "Git Repository", "claude", ".gate/", "Config", "SKILL.md", "Event Store", "D-Mail Schema", "Linear MCP"}
	for i, name := range expectedNames {
		if results[i].Name != name {
			t.Errorf("result[%d]: expected name %q, got %q", i, name, results[i].Name)
		}
	}
}

func TestRunDoctor_CreatesSpanWithEvents(t *testing.T) {
	// given: mock commands succeed
	exp := setupTestTracer(t)
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - ✓ Connected")
	}
	defer func() { execCommand = exec.CommandContext }()

	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	cfg := DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)

	ctx := context.Background()

	// when
	RunDoctor(ctx, filepath.Join(divRoot, "config.yaml"), dir)

	// then: amadeus.doctor span should exist
	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "amadeus.doctor" {
			found = true
			// Should have 8 doctor.check events (one per check)
			eventCount := 0
			for _, event := range s.Events {
				if event.Name == "doctor.check" {
					eventCount++
				}
			}
			if eventCount != 9 {
				t.Errorf("expected 9 doctor.check events, got %d", eventCount)
			}
		}
	}
	if !found {
		t.Errorf("expected 'amadeus.doctor' span")
	}
}

func TestCheckSkillMD_BothExist(t *testing.T) {
	// given: properly initialized .gate/ with skills
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	// when
	result := checkSkillMD(dir)

	// then
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkillMD_MissingSendable(t *testing.T) {
	// given: .gate/ with only dmail-readable
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, "skills", "dmail-sendable", "SKILL.md"))

	// when
	result := checkSkillMD(dir)

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "dmail-sendable") {
		t.Errorf("expected message to mention dmail-sendable, got: %s", result.Message)
	}
}

func TestCheckSkillMD_NoGateDir(t *testing.T) {
	// given: no .gate/ at all
	dir := t.TempDir()

	// when
	result := checkSkillMD(dir)

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_IncludesSkillMDCheck(t *testing.T) {
	// given: mock commands succeed
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - ✓ Connected")
	}
	defer func() { execCommand = exec.CommandContext }()

	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := RunDoctor(ctx, configPath, dir)

	// then: should have 9 results
	if len(results) != 9 {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Name
		}
		t.Fatalf("expected 9 results, got %d: %v", len(results), names)
	}

	// then: SKILL.md check should be present and OK
	var skillResult DoctorCheckResult
	found := false
	for _, r := range results {
		if r.Name == "SKILL.md" {
			skillResult = r
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected SKILL.md check in doctor results")
	}
	if skillResult.Status != CheckOK {
		t.Errorf("expected SKILL.md CheckOK, got %v: %s", skillResult.Status, skillResult.Message)
	}
}

func TestRunDoctor_ClaudeUnavailable_MCPSkipped(t *testing.T) {
	// given: no need to mock execCommand for this test
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
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

func TestCheckDMailSchema_EmptyArchive(t *testing.T) {
	// given: .gate/ with empty archive
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	// when
	result := checkDMailSchema(root)

	// then: skip — no D-Mails to validate
	if result.Status != CheckSkip {
		t.Errorf("expected CheckSkip for empty archive, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_ValidDMails(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	store.SaveDMail(DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "test",
		Severity:    SeverityHigh,
	})

	// when
	result := checkDMailSchema(root)

	// then
	if result.Status != CheckOK {
		t.Errorf("expected CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_InvalidDMail(t *testing.T) {
	// given: a D-Mail missing required kind (schema v1 violation)
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	content := []byte("---\nname: feedback-001\ndescription: test\n---\n\nbody\n")
	os.WriteFile(filepath.Join(root, "archive", "feedback-001.md"), content, 0o644)

	// when
	result := checkDMailSchema(root)

	// then
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail for invalid D-Mail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_NoGateDir(t *testing.T) {
	// given: no .gate/ at all
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// when
	result := checkDMailSchema(root)

	// then: skip — archive doesn't exist yet
	if result.Status != CheckSkip {
		t.Errorf("expected CheckSkip for missing .gate, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_ArchivePermissionError(t *testing.T) {
	// given: archive/ exists but is not readable
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	archiveDir := filepath.Join(root, "archive")
	os.Chmod(archiveDir, 0o000)
	defer os.Chmod(archiveDir, 0o755)

	// when
	result := checkDMailSchema(root)

	// then: FAIL — permission error should not be masked
	if result.Status != CheckFail {
		t.Errorf("expected CheckFail for permission error, got %v: %s", result.Status, result.Message)
	}
}
