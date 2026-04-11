package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"gopkg.in/yaml.v3"
)

// buildFakeClaude compiles the fake-claude binary and returns its absolute path.
func buildFakeClaude(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "fake-claude")

	// Locate fake-claude source relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	// thisFile = internal/session/doctor_checks_test.go -> project root = ../../
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	fakeSrc := filepath.Join(projectRoot, "tests", "scenario", "testdata", "fake-claude")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = fakeSrc
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake-claude: %v\n%s", err, out)
	}
	return binPath
}

// setupTestTracer is exposed via session.ExportSetupTestTracer from export_test.go.

// initGateDirForTest creates a minimal .gate/ directory structure for doctor tests.
func initGateDirForTest(t *testing.T, root string) {
	t.Helper()
	dirs := []string{
		filepath.Join(root, ".run"),
		filepath.Join(root, "events"),
		filepath.Join(root, "outbox"),
		filepath.Join(root, "inbox"),
		filepath.Join(root, "archive"),
		filepath.Join(root, "skills", "dmail-sendable"),
		filepath.Join(root, "skills", "dmail-readable"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Write default config
	cfg := domain.DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// Write SKILL.md files from embedded templates
	for _, name := range []string{"dmail-sendable", "dmail-readable"} {
		tmplPath := "templates/skills/" + name + "/SKILL.md"
		content, readErr := platform.SkillsFS.ReadFile(tmplPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		skillPath := filepath.Join(root, "skills", name, "SKILL.md")
		if err := os.WriteFile(skillPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Write .gitignore
	gitignorePath := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".run/\noutbox/\ninbox/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// nopSuccessRateChecker is a no-op SuccessRateChecker for tests that don't
// need real success-rate computation.
func nopSuccessRateChecker(gateDir string, logger domain.Logger) domain.DoctorCheck {
	return domain.DoctorCheck{
		Name:    "success-rate",
		Status:  domain.CheckOK,
		Message: "no events",
	}
}

func TestCheckStatusLabel(t *testing.T) {
	tests := []struct {
		status domain.CheckStatus
		want   string
	}{
		{domain.CheckOK, "OK"},
		{domain.CheckFail, "FAIL"},
		{domain.CheckSkip, "SKIP"},
	}
	for _, tt := range tests {
		if got := tt.status.StatusLabel(); got != tt.want {
			t.Errorf("StatusLabel(%d): expected %q, got %q", tt.status, tt.want, got)
		}
	}
}

func TestCheckTool_Exists(t *testing.T) {
	ctx := context.Background()
	result := session.CheckTool(ctx, "git")
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK for 'git', got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git") {
		t.Errorf("expected message to contain path, got: %s", result.Message)
	}
}

func TestCheckTool_NotFound(t *testing.T) {
	ctx := context.Background()
	result := session.CheckTool(ctx, "nonexistent-tool-xyz-12345")
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
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
	result := session.CheckGitRepo(dir)
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGitRepo_NotRepo(t *testing.T) {
	dir := t.TempDir()
	result := session.CheckGitRepo(dir)
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGateDir_Exists(t *testing.T) {
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	result := session.CheckGateDir(dir, false)
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGateDir_NotExist(t *testing.T) {
	dir := t.TempDir()
	result := session.CheckGateDir(dir, false)
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGateDir_RepairCreatesMissing(t *testing.T) {
	dir := t.TempDir()
	result := session.CheckGateDir(dir, true)
	if result.Status != domain.CheckFixed {
		t.Errorf("expected domain.CheckFixed, got %v: %s", result.Status, result.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gate")); err != nil {
		t.Error(".gate/ should have been created")
	}
}

func TestCheckClaudeAuth_Authenticated(t *testing.T) {
	// given
	mcpOutput := "plugin:linear:linear: https://mcp.linear.app/mcp (HTTP) - \u2713 Connected"

	// when
	result := session.CheckClaudeAuth(mcpOutput, nil, "claude")

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "authenticated" {
		t.Errorf("expected 'authenticated', got: %s", result.Message)
	}
}

func TestCheckClaudeAuth_NotAuthenticated(t *testing.T) {
	// given
	mcpErr := fmt.Errorf("exit status 1")

	// when
	result := session.CheckClaudeAuth("", mcpErr, "claude")

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected domain.CheckWarn, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Hint, "claude login") {
		t.Errorf("expected hint to mention 'claude login', got: %s", result.Hint)
	}
}

func TestCheckClaudeAuth_NotAuthenticated_WithEnvPrefix(t *testing.T) {
	// given
	mcpErr := fmt.Errorf("exit status 1")

	// when
	result := session.CheckClaudeAuth("", mcpErr, "CLAUDE_CONFIG_DIR=/foo claude")

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected domain.CheckWarn, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Hint, "CLAUDE_CONFIG_DIR=/foo") {
		t.Errorf("expected hint to include env prefix, got: %s", result.Hint)
	}
	if !strings.Contains(result.Hint, "login") {
		t.Errorf("expected hint to mention login, got: %s", result.Hint)
	}
}

func TestCheckLinearMCP_Connected(t *testing.T) {
	// given
	mcpOutput := "plugin:linear:linear: https://mcp.linear.app/mcp (HTTP) - \u2713 Connected"

	// when
	result := session.CheckLinearMCP(mcpOutput, nil)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_NotConnected(t *testing.T) {
	// given
	mcpOutput := "some-other-mcp: https://example.com - \u2713 Connected"

	// when
	result := session.CheckLinearMCP(mcpOutput, nil)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected domain.CheckWarn, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_CommandFails(t *testing.T) {
	// given
	mcpErr := fmt.Errorf("exit status 1")

	// when
	result := session.CheckLinearMCP("", mcpErr)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected domain.CheckWarn, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckLinearMCP_Disconnected(t *testing.T) {
	// given
	mcpOutput := "plugin:linear:linear: https://mcp.linear.app/mcp (HTTP) - \u2717 Disconnected"

	// when
	result := session.CheckLinearMCP(mcpOutput, nil)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected domain.CheckWarn for disconnected, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(path, data, 0o644)
	result := session.CheckConfig(path)
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_NotFound(t *testing.T) {
	result := session.CheckConfig("/nonexistent/config.yaml")
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`{{{invalid`), 0o644)
	result := session.CheckConfig(path)
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_ReturnsAllResults(t *testing.T) {
	// given: mock commands succeed
	cleanupCmd := session.OverrideShellCmd(func(ctx context.Context, cmdLine string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - \u2713 Connected")
	})
	defer cleanupCmd()
	cleanupPath := session.OverrideLookPath(func(cmdLine string) (string, error) {
		return "/usr/local/bin/" + cmdLine, nil
	})
	defer cleanupPath()

	dir := t.TempDir()
	// Create .gate/ with config
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)

	// Init git repo
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := session.RunDoctorWithClaudeCmd(ctx, configPath, dir, domain.DefaultClaudeCmd, &domain.NopLogger{}, false, domain.ModeLinear, nopSuccessRateChecker)

	// then: should have 19 results
	if len(results) != 19 {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Name
		}
		t.Fatalf("expected 19 results, got %d: %v", len(results), names)
	}
	// Verify names in order
	expectedNames := []string{"git", "claude", "gh", "gh-auth", "Git Repository", "Git Remote", ".gate/", "Config", "SKILL.md", "Event Store", "D-Mail Schema", "dead-letters", "fsnotify", "claude-auth", "linear-mcp", "claude-inference", "context-budget", "skills-ref", "success-rate"}
	for i, name := range expectedNames {
		if results[i].Name != name {
			t.Errorf("result[%d]: expected name %q, got %q", i, name, results[i].Name)
		}
	}
}

func TestRunDoctor_CreatesSpanWithEvents(t *testing.T) {
	// given: mock commands succeed
	exp := session.ExportSetupTestTracer(t)
	cleanupCmd := session.OverrideShellCmd(func(ctx context.Context, cmdLine string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - \u2713 Connected")
	})
	defer cleanupCmd()
	cleanupPath := session.OverrideLookPath(func(cmdLine string) (string, error) {
		return "/usr/local/bin/" + cmdLine, nil
	})
	defer cleanupPath()

	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)

	ctx := context.Background()

	// when
	session.RunDoctorWithClaudeCmd(ctx, filepath.Join(divRoot, "config.yaml"), dir, domain.DefaultClaudeCmd, &domain.NopLogger{}, false, domain.ModeLinear, nopSuccessRateChecker)

	// then: domain.doctor span should exist
	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "domain.doctor" {
			found = true
			// Should have 17 doctor.check events (one per check)
			eventCount := 0
			for _, event := range s.Events {
				if event.Name == "doctor.check" {
					eventCount++
				}
			}
			if eventCount != 19 {
				t.Errorf("expected 19 doctor.check events, got %d", eventCount)
			}
		}
	}
	if !found {
		t.Errorf("expected 'domain.doctor' span")
	}
}

func TestCheckSkillMD_BothExist(t *testing.T) {
	// given: properly initialized .gate/ with skills
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)

	// when
	result := session.CheckSkillMD(dir)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkillMD_MissingSendable(t *testing.T) {
	// given: .gate/ with only dmail-readable
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)
	os.Remove(filepath.Join(root, "skills", "dmail-sendable", "SKILL.md"))

	// when
	result := session.CheckSkillMD(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "dmail-sendable") {
		t.Errorf("expected message to mention dmail-sendable, got: %s", result.Message)
	}
}

func TestCheckSkillMD_NoGateDir(t *testing.T) {
	// given: no .gate/ at all
	dir := t.TempDir()

	// when
	result := session.CheckSkillMD(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckSkillMD_DeprecatedFeedbackKind(t *testing.T) {
	// given: SKILL.md with deprecated "kind: feedback" (pre-split)
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)
	// Overwrite sendable with old kind
	os.WriteFile(filepath.Join(root, "skills", "dmail-sendable", "SKILL.md"),
		[]byte("---\nname: dmail-sendable\nmetadata:\n  dmail-schema-version: \"1\"\nproduces:\n    - kind: feedback\n---\n"), 0o644)

	// when
	result := session.CheckSkillMD(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail for deprecated kind, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Hint, "init --force") {
		t.Errorf("hint should suggest init --force, got %q", result.Hint)
	}
}

func TestCheckSkillMD_UpdatedFeedbackKind(t *testing.T) {
	// given: SKILL.md with updated kinds (post-split)
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)

	// when
	result := session.CheckSkillMD(dir)

	// then: templates already have updated kinds
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK for updated kind, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_IncludesSkillMDCheck(t *testing.T) {
	// given: mock commands succeed
	cleanupCmd := session.OverrideShellCmd(func(ctx context.Context, cmdLine string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - \u2713 Connected")
	})
	defer cleanupCmd()
	cleanupPath := session.OverrideLookPath(func(cmdLine string) (string, error) {
		return "/usr/local/bin/" + cmdLine, nil
	})
	defer cleanupPath()

	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	initGateDirForTest(t, divRoot)
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := session.RunDoctorWithClaudeCmd(ctx, configPath, dir, domain.DefaultClaudeCmd, &domain.NopLogger{}, false, domain.ModeLinear, nopSuccessRateChecker)

	// then: should have 19 results
	if len(results) != 19 {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Name
		}
		t.Fatalf("expected 19 results, got %d: %v", len(results), names)
	}

	// then: SKILL.md check should be present and OK
	var skillResult domain.DoctorCheck
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
	if skillResult.Status != domain.CheckOK {
		t.Errorf("expected SKILL.md domain.CheckOK, got %v: %s", skillResult.Status, skillResult.Message)
	}
}

func TestRunDoctor_ClaudeUnavailable_AuthAndMCPSkipped(t *testing.T) {
	// given
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when: pass a nonexistent claude command
	results := session.RunDoctorWithClaudeCmd(ctx, configPath, dir, "nonexistent-claude-xyz", &domain.NopLogger{}, false, domain.ModeLinear, nopSuccessRateChecker)

	// then: claude-auth, Linear MCP, and claude-inference should be skipped
	var authResult, mcpResult, inferResult domain.DoctorCheck
	for _, r := range results {
		switch r.Name {
		case "claude-auth":
			authResult = r
		case "linear-mcp":
			mcpResult = r
		case "claude-inference":
			inferResult = r
		}
	}
	if authResult.Status != domain.CheckSkip {
		t.Errorf("expected claude-auth SKIP when claude unavailable, got %v: %s", authResult.Status, authResult.Message)
	}
	if !strings.Contains(authResult.Message, "claude not available") {
		t.Errorf("expected 'claude not available' in auth message, got: %s", authResult.Message)
	}
	if mcpResult.Status != domain.CheckSkip {
		t.Errorf("expected Linear MCP SKIP when claude unavailable, got %v: %s", mcpResult.Status, mcpResult.Message)
	}
	if !strings.Contains(mcpResult.Message, "claude not available") {
		t.Errorf("expected 'claude not available' in MCP message, got: %s", mcpResult.Message)
	}
	if inferResult.Status != domain.CheckSkip {
		t.Errorf("expected claude-inference SKIP when claude unavailable, got %v: %s", inferResult.Status, inferResult.Message)
	}
	if !strings.Contains(inferResult.Message, "claude not available") {
		t.Errorf("expected 'claude not available' in inference message, got: %s", inferResult.Message)
	}
}

func TestRunDoctor_MCPListFails_InferenceStillRuns(t *testing.T) {
	// given: claude binary exists but mcp list fails
	callCount := 0
	cleanupCmd := session.OverrideShellCmd(func(ctx context.Context, cmdLine string, args ...string) *exec.Cmd {
		callCount++
		// First call: mcp list -> fail
		for _, arg := range args {
			if arg == "list" {
				return exec.Command("false")
			}
			if arg == "--print" {
				// inference call -> succeed with "2"
				return exec.Command("echo", `{"type":"result","result":"2"}`)
			}
		}
		// default: version check
		return exec.Command("echo", "1.0.0")
	})
	defer cleanupCmd()
	cleanupPath := session.OverrideLookPath(func(cmdLine string) (string, error) {
		return "/usr/local/bin/" + cmdLine, nil
	})
	defer cleanupPath()

	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := session.RunDoctorWithClaudeCmd(ctx, configPath, dir, domain.DefaultClaudeCmd, &domain.NopLogger{}, false, domain.ModeLinear, nopSuccessRateChecker)

	// then: claude-auth should WARN, linear-mcp should SKIP, but inference should NOT be skipped
	var authResult, mcpResult, inferResult domain.DoctorCheck
	for _, r := range results {
		switch r.Name {
		case "claude-auth":
			authResult = r
		case "linear-mcp":
			mcpResult = r
		case "claude-inference":
			inferResult = r
		}
	}
	if authResult.Status != domain.CheckWarn {
		t.Errorf("claude-auth: expected WARN, got %v: %s", authResult.Status, authResult.Message)
	}
	if mcpResult.Status != domain.CheckSkip {
		t.Errorf("linear-mcp: expected SKIP, got %v: %s", mcpResult.Status, mcpResult.Message)
	}
	if inferResult.Status == domain.CheckSkip {
		t.Errorf("claude-inference: should NOT be skipped when mcp list fails, got SKIP: %s", inferResult.Message)
	}
}

func TestCheckDMailSchema_EmptyArchive(t *testing.T) {
	// given: .gate/ with empty archive
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)

	// when
	result := session.CheckDMailSchema(root)

	// then: skip -- no D-Mails to validate
	if result.Status != domain.CheckSkip {
		t.Errorf("expected domain.CheckSkip for empty archive, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_ValidDMails(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)
	// Write a valid D-Mail directly to archive
	dmail := domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "feedback-001",
		Kind:          domain.KindDesignFeedback,
		Description:   "test",
		Severity:      domain.SeverityHigh,
		Body:          "Content.\n",
	}
	data, err := domain.MarshalDMail(dmail)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "archive", "feedback-001.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result := session.CheckDMailSchema(root)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_InvalidDMail(t *testing.T) {
	// given: a D-Mail missing required kind (schema v1 violation)
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)
	content := []byte("---\nname: feedback-001\ndescription: test\n---\n\nbody\n")
	os.WriteFile(filepath.Join(root, "archive", "feedback-001.md"), content, 0o644)

	// when
	result := session.CheckDMailSchema(root)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail for invalid D-Mail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_NoGateDir(t *testing.T) {
	// given: no .gate/ at all
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")

	// when
	result := session.CheckDMailSchema(root)

	// then: skip -- archive doesn't exist yet
	if result.Status != domain.CheckSkip {
		t.Errorf("expected domain.CheckSkip for missing .gate, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckDMailSchema_ArchivePermissionError(t *testing.T) {
	// given: archive/ exists but is not readable
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	initGateDirForTest(t, root)
	archiveDir := filepath.Join(root, "archive")
	os.Chmod(archiveDir, 0o000)
	defer os.Chmod(archiveDir, 0o755)

	// when
	result := session.CheckDMailSchema(root)

	// then: FAIL -- permission error should not be masked
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail for permission error, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTool_GH(t *testing.T) {
	// given
	ctx := context.Background()

	// when
	result := session.CheckTool(ctx, "gh")

	// then: gh should be available in the test environment
	if result.Status != domain.CheckOK {
		t.Skipf("gh not installed, skipping: %s", result.Message)
	}
	if !strings.Contains(result.Message, "gh") {
		t.Errorf("expected message to contain path, got: %s", result.Message)
	}
}

func TestCheckGitRemote_HasRemote(t *testing.T) {
	// given: a git repo with a remote
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/example/repo.git").Run()

	// when
	result := session.CheckGitRemote(dir)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "origin") {
		t.Errorf("expected message to contain 'origin', got: %s", result.Message)
	}
}

func TestCheckGitRemote_NoRemote(t *testing.T) {
	// given: a git repo without remotes
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()

	// when
	result := session.CheckGitRemote(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "no remote") {
		t.Errorf("expected 'no remote' in message, got: %s", result.Message)
	}
}

func TestCheckGitRemote_NotGitRepo(t *testing.T) {
	// given: a directory that is not a git repo
	dir := t.TempDir()

	// when
	result := session.CheckGitRemote(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckFsnotify_Available(t *testing.T) {
	// when
	result := session.CheckFsnotify()

	// then: should succeed on any normal test environment
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
	if result.Name != "fsnotify" {
		t.Errorf("expected name 'fsnotify', got: %s", result.Name)
	}
}

func TestRunDoctor_IncludesSuccessRate(t *testing.T) {
	// given: mock commands succeed
	cleanupCmd := session.OverrideShellCmd(func(ctx context.Context, cmdLine string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - \u2713 Connected")
	})
	defer cleanupCmd()
	cleanupPath := session.OverrideLookPath(func(cmdLine string) (string, error) {
		return "/usr/local/bin/" + cmdLine, nil
	})
	defer cleanupPath()

	repoRoot := t.TempDir()
	gateDir := filepath.Join(repoRoot, ".gate")
	initGateDirForTest(t, gateDir)

	exec.Command("git", "init", repoRoot).Run()
	exec.Command("git", "-C", repoRoot, "commit", "--allow-empty", "-m", "init").Run()

	// Write 3 check.completed events: 2 clean, 1 with drift
	eventsDir := filepath.Join(gateDir, "events")
	today := time.Now().UTC().Format("2006-01-02")

	cleanData, _ := json.Marshal(domain.CheckCompletedData{
		Result: domain.CheckResult{DMails: nil},
	})
	driftData, _ := json.Marshal(domain.CheckCompletedData{
		Result: domain.CheckResult{DMails: []string{"feedback-001"}},
	})
	now := time.Now().UTC().Format(time.RFC3339)
	lines := []string{
		`{"id":"e1","type":"check.completed","timestamp":"` + now + `","data":` + string(cleanData) + `}`,
		`{"id":"e2","type":"check.completed","timestamp":"` + now + `","data":` + string(cleanData) + `}`,
		`{"id":"e3","type":"check.completed","timestamp":"` + now + `","data":` + string(driftData) + `}`,
	}
	eventContent := strings.Join(lines, "\n") + "\n"
	os.WriteFile(filepath.Join(eventsDir, today+".jsonl"), []byte(eventContent), 0o644)

	ctx := context.Background()
	configPath := filepath.Join(gateDir, "config.yaml")

	// Use a real success-rate checker that reads events
	realSuccessRateChecker := func(gateDir string, logger domain.Logger) domain.DoctorCheck {
		eventStore := session.NewEventStore(gateDir, logger)
		events, _, loadErr := eventStore.LoadAll(context.Background())
		if loadErr != nil || len(events) == 0 {
			return domain.DoctorCheck{
				Name: "success-rate", Status: domain.CheckOK,
				Message: "no events",
			}
		}
		rate := domain.SuccessRate(events)
		var clean, total int
		for _, ev := range events {
			if ev.Type != domain.EventCheckCompleted {
				continue
			}
			var data domain.CheckCompletedData
			if jsonErr := json.Unmarshal(ev.Data, &data); jsonErr != nil {
				continue
			}
			total++
			if len(data.Result.DMails) == 0 {
				clean++
			}
		}
		return domain.DoctorCheck{
			Name: "success-rate", Status: domain.CheckOK,
			Message: domain.FormatSuccessRate(rate, clean, total),
		}
	}

	// when
	results := session.RunDoctorWithClaudeCmd(ctx, configPath, repoRoot, domain.DefaultClaudeCmd, &domain.NopLogger{}, false, domain.ModeLinear, realSuccessRateChecker)

	// then: success-rate check should be present
	var found bool
	for _, r := range results {
		if r.Name == "success-rate" {
			found = true
			if r.Status != domain.CheckOK {
				t.Errorf("expected domain.CheckOK, got %v", r.Status)
			}
			if !strings.Contains(r.Message, "66.7%") || !strings.Contains(r.Message, "(2/3)") {
				t.Errorf("unexpected message: %s", r.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected success-rate check in results")
	}
}

func TestRunDoctor_AllPassWithFakeClaude(t *testing.T) {
	// given: fake-claude binary via RunDoctorWithClaudeCmd
	fakeClaude := buildFakeClaude(t)

	repoRoot := t.TempDir()
	gateDir := filepath.Join(repoRoot, ".gate")
	initGateDirForTest(t, gateDir)
	exec.Command("git", "init", repoRoot).Run()
	exec.Command("git", "-C", repoRoot, "remote", "add", "origin", "https://github.com/example/repo.git").Run()

	ctx := context.Background()
	configPath := filepath.Join(gateDir, "config.yaml")

	// when
	results := session.RunDoctorWithClaudeCmd(ctx, configPath, repoRoot, fakeClaude, &domain.NopLogger{}, false, domain.ModeLinear, nopSuccessRateChecker)

	// then: claude-auth, linear-mcp, and claude-inference should be OK
	var authResult, mcpResult, inferResult domain.DoctorCheck
	for _, r := range results {
		switch r.Name {
		case "claude-auth":
			authResult = r
		case "linear-mcp":
			mcpResult = r
		case "claude-inference":
			inferResult = r
		}
	}
	if authResult.Status != domain.CheckOK {
		t.Errorf("claude-auth: expected OK, got %v: %s", authResult.Status, authResult.Message)
	}
	if mcpResult.Status != domain.CheckOK {
		t.Errorf("linear-mcp: expected OK, got %v: %s", mcpResult.Status, mcpResult.Message)
	}
	if inferResult.Status != domain.CheckOK {
		t.Errorf("claude-inference: expected OK, got %v: %s", inferResult.Status, inferResult.Message)
	}
}

func TestCheckClaudeInference_Success(t *testing.T) {
	// given
	output := "2"

	// when
	result := session.CheckClaudeInference(output, nil)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
	if result.Message != "inference OK" {
		t.Errorf("expected 'inference OK', got: %s", result.Message)
	}
}

func TestCheckClaudeInference_Error(t *testing.T) {
	// given
	err := fmt.Errorf("exit status 1")

	// when
	result := session.CheckClaudeInference("", err)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckClaudeInference_FalsePositive(t *testing.T) {
	// given: "12" contains "2" but is not the expected answer
	output := "12"

	// when
	result := session.CheckClaudeInference(output, nil)

	// then: must WARN -- "12" is not "2"
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN for false positive '12', got %v: %s", result.Status, result.Message)
	}
	if !strings.HasPrefix(result.Message, "unexpected response: ") {
		t.Errorf("expected message starting with 'unexpected response: ', got: %s", result.Message)
	}
}

func TestCheckClaudeInference_UnexpectedResponse(t *testing.T) {
	// given
	output := "I cannot compute that"

	// when
	result := session.CheckClaudeInference(output, nil)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status, result.Message)
	}
	if !strings.HasPrefix(result.Message, "unexpected response: ") {
		t.Errorf("expected message starting with 'unexpected response: ', got: %s", result.Message)
	}
}

func TestFindSkillsRefDir_UsesBaseDir(t *testing.T) {
	t.Parallel()

	// given: skills-ref exists relative to baseDir but NOT relative to CWD
	baseDir := t.TempDir()
	skillsRefDir := filepath.Join(baseDir, "..", "skills-ref")
	if err := os.MkdirAll(skillsRefDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// when: findSkillsRefDir uses baseDir (not CWD)
	result := session.ExportFindSkillsRefDir(baseDir)

	// then: should find the skills-ref directory
	if result == "" {
		t.Error("expected findSkillsRefDir to find skills-ref relative to baseDir, got empty string")
	}
}

func TestFindSkillsRefDir_NotFound(t *testing.T) {
	t.Parallel()

	// given: no skills-ref anywhere near baseDir
	baseDir := t.TempDir()

	// when
	result := session.ExportFindSkillsRefDir(baseDir)

	// then
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCheckContextBudget_LowUsage(t *testing.T) {
	t.Parallel()

	// given: stream-json with small init (2 tools, 1 skill)
	streamJSON := `{"type":"system","subtype":"init","model":"claude-opus-4-6","tools":["Read","Write"],"skills":["commit"],"plugins":[],"mcp_servers":[]}
{"type":"result","result":"2"}`

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then: should be OK (well under threshold)
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckContextBudget_HighUsage(t *testing.T) {
	t.Parallel()

	// given: stream-json with many tools/skills/plugins + large hook output
	var lines []string
	lines = append(lines, `{"type":"system","subtype":"init","model":"claude-opus-4-6","tools":["Read","Write","Bash","Glob","Grep","Agent","mcp__a__t1","mcp__b__t2","mcp__c__t3","mcp__d__t4","mcp__e__t5","mcp__f__t6","mcp__g__t7","mcp__h__t8"],"skills":["s1","s2","s3","s4","s5","s6","s7","s8","s9","s10","s11","s12","s13","s14","s15","s16","s17","s18","s19","s20","s21","s22","s23","s24","s25","s26","s27","s28","s29","s30","s31","s32","s33","s34","s35","s36","s37","s38","s39","s40"],"plugins":[{"name":"p1"},{"name":"p2"},{"name":"p3"},{"name":"p4"},{"name":"p5"},{"name":"p6"},{"name":"p7"},{"name":"p8"}],"mcp_servers":[{"name":"a","status":"connected"},{"name":"b","status":"connected"},{"name":"c","status":"connected"},{"name":"d","status":"connected"},{"name":"e","status":"connected"},{"name":"f","status":"connected"}]}`)
	// Add large hook response (simulate ~80K chars of hook context)
	largeStdout := strings.Repeat("x", 80000)
	lines = append(lines, fmt.Sprintf(`{"type":"system","subtype":"hook_response","hook_id":"h1","stdout":"%s","exit_code":0}`, largeStdout))
	lines = append(lines, `{"type":"result","result":"2"}`)
	streamJSON := strings.Join(lines, "\n")

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then: should be WARN with hint (over threshold)
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v: %s", result.Status.StatusLabel(), result.Message)
	}
	if result.Hint == "" {
		t.Error("expected non-empty hint for high usage")
	}
}

func TestCheckContextBudget_EmptyStream(t *testing.T) {
	t.Parallel()

	// given: empty stream
	streamJSON := ""

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then: should be OK (nothing to measure)
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK for empty stream, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckContextBudget_NoInitMessage(t *testing.T) {
	t.Parallel()

	// given: stream-json with no init message
	streamJSON := `{"type":"result","result":"2"}`

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then: should be OK (no init = no overhead)
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK for no-init stream, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckContextBudget_WarnWithBreakdown(t *testing.T) {
	t.Parallel()

	// given: stream with many skills (exceeds threshold)
	initMsg := `{"type":"system","subtype":"init","tools":["Read","Write"],"skills":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z","aa","ab","ac","ad","ae","af","ag","ah","ai","aj","ak","al","am","an"],"plugins":[{"name":"p1"},{"name":"p2"},{"name":"p3"},{"name":"p4"},{"name":"p5"}],"mcp_servers":[{"name":"linear","status":"connected"}]}`
	streamJSON := initMsg + "\n"

	// when
	result := session.CheckContextBudget(streamJSON, "")

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v", result.Status.StatusLabel())
	}
	if !strings.Contains(result.Message, "skills") {
		t.Errorf("message should contain breakdown with 'skills', got: %s", result.Message)
	}
	if result.Hint == "" {
		t.Error("expected hint for threshold exceeded")
	}
}

func TestCheckContextBudget_WarnHintWithSettingsFile(t *testing.T) {
	t.Parallel()

	// given: project with .claude/settings.json
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(`{}`), 0o644)

	initMsg := `{"type":"system","subtype":"init","skills":["a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z","aa","ab","ac","ad","ae","af","ag","ah","ai","aj","ak","al","am","an","ao","ap"]}`
	streamJSON := initMsg + "\n"

	// when
	result := session.CheckContextBudget(streamJSON, dir)

	// then
	if result.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %v", result.Status.StatusLabel())
	}
	if !strings.Contains(result.Hint, "\u898b\u76f4\u3057\u3066") {
		t.Errorf("hint should say review settings, got: %s", result.Hint)
	}
}

func TestDoctor_WaveMode_SkipsLinearMCP(t *testing.T) {
	// given: wave mode doctor run
	dir := t.TempDir()
	gateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(gateDir, 0755)
	configPath := filepath.Join(gateDir, "config.yaml")
	os.WriteFile(configPath, []byte("lang: en\nclaude_cmd: echo\n"), 0644)

	// when: run doctor in wave mode
	results := session.RunDoctorWithClaudeCmd(context.Background(), configPath, dir, "echo", &domain.NopLogger{}, false, domain.ModeWave, nopSuccessRateChecker)

	// then: linear-mcp should be SKIP (not WARN or FAIL)
	for _, r := range results {
		if r.Name == "linear-mcp" {
			if r.Status != domain.CheckSkip {
				t.Errorf("wave mode: linear-mcp status = %v, want SKIP", r.Status)
			}
			if !strings.Contains(r.Message, "wave mode") {
				t.Errorf("wave mode: expected 'wave mode' in message, got: %s", r.Message)
			}
			return
		}
	}
	// linear-mcp check might be skipped entirely if claude is not available
	// That's acceptable -- the check is conditional on claude being found
}

func TestCheckEventStore_CorruptLines(t *testing.T) {
	// given: event dir with corrupt JSONL
	gateRoot := t.TempDir()
	eventsDir := filepath.Join(gateRoot, "events")
	os.MkdirAll(eventsDir, 0755)

	validEvent := `{"type":"check.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	corruptLine := `{not valid json`
	os.WriteFile(filepath.Join(eventsDir, "2026-04-08.jsonl"),
		[]byte(validEvent+"\n"+corruptLine+"\n"+validEvent+"\n"), 0644)

	// when
	check := session.CheckEventStore(gateRoot)

	// then
	if check.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
	if !strings.Contains(check.Message, "1 corrupt line") {
		t.Errorf("expected '1 corrupt line' in message: %q", check.Message)
	}
}

func TestCheckEventStore_Clean(t *testing.T) {
	// given: clean event dir
	gateRoot := t.TempDir()
	eventsDir := filepath.Join(gateRoot, "events")
	os.MkdirAll(eventsDir, 0755)

	validEvent := `{"type":"check.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	os.WriteFile(filepath.Join(eventsDir, "2026-04-08.jsonl"),
		[]byte(validEvent+"\n"), 0644)

	// when
	check := session.CheckEventStore(gateRoot)

	// then
	if check.Status != domain.CheckOK {
		t.Errorf("expected OK, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
}
