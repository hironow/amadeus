//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestE2E_Version(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_version"
	initTestRepo(t, ctx, c, dir)

	stdout, _, err := runCmd(t, ctx, c, dir, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(stdout, "amadeus") {
		t.Errorf("expected 'amadeus' in version output, got: %s", stdout)
	}
}

func TestE2E_VersionJSON(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_version_json"
	initTestRepo(t, ctx, c, dir)

	stdout, _, err := runCmd(t, ctx, c, dir, "version", "--json")
	if err != nil {
		t.Fatalf("version --json: %v", err)
	}
	var v map[string]string
	parseJSONOutput(t, stdout, &v)
	for _, key := range []string{"version", "commit", "date"} {
		if _, ok := v[key]; !ok {
			t.Errorf("missing key %q in JSON output", key)
		}
	}
}

func TestE2E_Help(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_help"
	initTestRepo(t, ctx, c, dir)

	stdout, _, err := runCmd(t, ctx, c, dir, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	for _, sub := range []string{"init", "sync", "doctor", "log", "validate", "mark-commented", "archive-prune", "version", "mcp", "sessions"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("expected %q in help output", sub)
		}
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_unknown"
	initTestRepo(t, ctx, c, dir)

	_, _, err := runCmd(t, ctx, c, dir, "nonexistent-cmd")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestE2E_NoSubcommand(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_no_subcmd"
	initTestRepo(t, ctx, c, dir)

	_, _, err := runCmd(t, ctx, c, dir)
	if err == nil {
		t.Fatal("expected error when no subcommand given")
	}
}

func TestE2E_Init(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_init"
	initTestRepo(t, ctx, c, dir)

	// Verify .gate structure
	for _, sub := range []string{".run", "events", "outbox", "inbox", "archive"} {
		path := fmt.Sprintf("%s/.gate/%s", dir, sub)
		if !dirExistsInContainer(t, ctx, c, path) && !fileExistsInContainer(t, ctx, c, path) {
			t.Errorf("expected %s to exist in container", path)
		}
	}
	if !fileExistsInContainer(t, ctx, c, dir+"/.gate/config.yaml") {
		t.Error("expected config.yaml to exist in container")
	}
	if !fileExistsInContainer(t, ctx, c, dir+"/.gate/skills/dmail-sendable/SKILL.md") {
		t.Error("expected SKILL.md to exist in container")
	}
}

func TestE2E_Init_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_init_exist"
	initTestRepo(t, ctx, c, dir)

	// Running init again should fail with "already exists"
	_, _, err := runCmd(t, ctx, c, dir, "init")
	if err == nil {
		t.Fatal("expected error on second init")
	}
}

func TestE2E_Validate_ValidConfig(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_validate_valid"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	_, _, err := runCmd(t, ctx, c, dir, "validate")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestE2E_Validate_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_validate_invalid"
	initTestRepo(t, ctx, c, dir)

	cfg := defaultTestConfig()
	// Break weights sum
	cfg["weights"] = map[string]any{
		"adr_integrity":        0.50,
		"dod_fulfillment":      0.50,
		"dependency_integrity": 0.50,
		"implicit_constraints": 0.50,
	}
	writeConfig(t, ctx, c, dir, cfg)

	_, _, err := runCmd(t, ctx, c, dir, "validate")
	if err == nil {
		t.Fatal("expected validation error for bad weights sum")
	}
}

func TestE2E_Doctor(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_doctor"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	out, _, _ := runCmd(t, ctx, c, dir, "doctor")
	if len(strings.TrimSpace(out)) == 0 {
		t.Error("doctor output is empty")
	}
}

func TestE2E_DoctorJSON(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_doctor_json"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	stdout, _, _ := runCmd(t, ctx, c, dir, "doctor", "--json")
	var result struct {
		Checks []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	parseJSONOutput(t, stdout, &result)
	if len(result.Checks) == 0 {
		t.Error("expected at least one check")
	}
}

func TestE2E_Log_Empty(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_log"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, ctx, c, dir, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if !strings.Contains(stdout, "No history") {
		t.Errorf("expected 'No history' in output, got: %s", stdout)
	}
}

func TestE2E_Log_EmptyJSON(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_log_json"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, ctx, c, dir, "log", "--json")
	if err != nil {
		t.Fatalf("log --json: %v", err)
	}
	var result struct {
		History  []any `json:"history"`
		DMails   []any `json:"dmails"`
		Consumed []any `json:"consumed"`
	}
	parseJSONOutput(t, stdout, &result)
	if len(result.History) != 0 {
		t.Errorf("expected empty history, got %d items", len(result.History))
	}
}

func TestE2E_Sync_Empty(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_sync_empty"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	stdout, _, err := runCmd(t, ctx, c, dir, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	var result struct {
		PendingComments []any `json:"pending_comments"`
	}
	parseJSONOutput(t, stdout, &result)
	if len(result.PendingComments) != 0 {
		t.Errorf("expected no pending comments, got %d", len(result.PendingComments))
	}
}

func TestE2E_MCPServerToolsList(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_mcp"
	initTestRepo(t, ctx, c, dir)

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	stdout, _, err := runCmdStdin(t, ctx, c, dir, input, "mcp")
	if err != nil {
		t.Fatalf("mcp command failed: %v", err)
	}

	idx := strings.Index(stdout, `{"jsonrpc"`)
	if idx < 0 {
		t.Fatalf("no JSON-RPC response found in stdout: %s", stdout)
	}
	jsonStr := stdout[idx:]

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON-RPC response: %v\nraw: %s", err, jsonStr)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}

	expectedTools := map[string]bool{
		"amadeus.ping":        false,
		"amadeus.next_review": false,
		"amadeus.post_comment": false,
		"amadeus.get_pr_status": false,
	}

	for _, tool := range resp.Result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing expected tool in MCP response: %s", name)
		}
	}
}
