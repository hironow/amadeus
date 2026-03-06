package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gopkg.in/yaml.v3"
)

// setupTestTracer configures an in-memory tracer for testing doctor spans.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	platform.Tracer = tp.Tracer("amadeus-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		platform.Tracer = prev.Tracer("amadeus")
	})
	return exp
}

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
		content, readErr := platform.SkillTemplateFS.ReadFile(tmplPath)
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
	result := checkTool(ctx, "git")
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK for 'git', got %v: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "git") {
		t.Errorf("expected message to contain path, got: %s", result.Message)
	}
}

func TestCheckTool_NotFound(t *testing.T) {
	ctx := context.Background()
	result := checkTool(ctx, "nonexistent-tool-xyz-12345")
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
	result := checkGitRepo(dir)
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGitRepo_NotRepo(t *testing.T) {
	dir := t.TempDir()
	result := checkGitRepo(dir)
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGateDir_Exists(t *testing.T) {
	dir := t.TempDir()
	divRoot := filepath.Join(dir, ".gate")
	os.MkdirAll(divRoot, 0o755)
	result := checkGateDir(dir)
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckGateDir_NotExist(t *testing.T) {
	dir := t.TempDir()
	result := checkGateDir(dir)
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
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
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
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
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
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
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
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
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail for disconnected, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(path, data, 0o644)
	result := checkConfig(path)
	if result.Status != domain.CheckOK {
		t.Errorf("expected domain.CheckOK, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_NotFound(t *testing.T) {
	result := checkConfig("/nonexistent/config.yaml")
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(`{{{invalid`), 0o644)
	result := checkConfig(path)
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
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
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)

	// Init git repo
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := runDoctor(ctx, configPath, dir, &domain.NopLogger{})

	// then: should have 10 results
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	// Verify names in order
	expectedNames := []string{"git", "Git Repository", "claude", ".gate/", "Config", "SKILL.md", "Event Store", "D-Mail Schema", "success-rate", "Linear MCP"}
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
	cfg := domain.DefaultConfig()
	data, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(divRoot, "config.yaml"), data, 0o644)

	ctx := context.Background()

	// when
	runDoctor(ctx, filepath.Join(divRoot, "config.yaml"), dir, &domain.NopLogger{})

	// then: domain.doctor span should exist
	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "domain.doctor" {
			found = true
			// Should have 10 doctor.check events (one per check)
			eventCount := 0
			for _, event := range s.Events {
				if event.Name == "doctor.check" {
					eventCount++
				}
			}
			if eventCount != 10 {
				t.Errorf("expected 10 doctor.check events, got %d", eventCount)
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
	result := checkSkillMD(dir)

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
	result := checkSkillMD(dir)

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
	result := checkSkillMD(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail, got %v: %s", result.Status, result.Message)
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
	initGateDirForTest(t, divRoot)
	exec.Command("git", "init", dir).Run()

	ctx := context.Background()
	configPath := filepath.Join(divRoot, "config.yaml")

	// when
	results := runDoctor(ctx, configPath, dir, &domain.NopLogger{})

	// then: should have 10 results
	if len(results) != 10 {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Name
		}
		t.Fatalf("expected 10 results, got %d: %v", len(results), names)
	}

	// then: SKILL.md check should be present and OK
	var skillResult domain.DoctorCheckResult
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

func TestRunDoctor_ClaudeUnavailable_MCPSkipped(t *testing.T) {
	// given: no need to mock execCommand for this test
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
	results := runDoctorWithClaudeCmd(ctx, configPath, dir, "nonexistent-claude-xyz", &domain.NopLogger{})

	// then
	var mcpResult domain.DoctorCheckResult
	for _, r := range results {
		if r.Name == "Linear MCP" {
			mcpResult = r
			break
		}
	}
	if mcpResult.Status != domain.CheckSkip {
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
	initGateDirForTest(t, root)

	// when
	result := checkDMailSchema(root)

	// then: skip — no D-Mails to validate
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
		Kind:          domain.KindFeedback,
		Description:   "test",
		Severity:      domain.SeverityHigh,
	}
	data, err := domain.MarshalDMail(dmail)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "archive", "feedback-001.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	result := checkDMailSchema(root)

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
	result := checkDMailSchema(root)

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
	result := checkDMailSchema(root)

	// then: skip — archive doesn't exist yet
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
	result := checkDMailSchema(root)

	// then: FAIL — permission error should not be masked
	if result.Status != domain.CheckFail {
		t.Errorf("expected domain.CheckFail for permission error, got %v: %s", result.Status, result.Message)
	}
}

func TestRunDoctor_IncludesSuccessRate(t *testing.T) {
	// given: mock commands succeed
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "plugin:linear:linear: - ✓ Connected")
	}
	defer func() { execCommand = exec.CommandContext }()

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

	// when
	results := runDoctor(ctx, configPath, repoRoot, &domain.NopLogger{})

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
