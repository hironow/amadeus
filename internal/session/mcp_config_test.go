package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestGenerateMCPConfig_WaveMode_EmptyServers(t *testing.T) {
	dir := t.TempDir()
	path, err := session.GenerateMCPConfig(dir, domain.ModeWave, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("expected non-empty file")
	}
	// Should contain empty mcpServers
	if got := string(data); got == "" || !contains(got, `"mcpServers"`) {
		t.Errorf("expected mcpServers key, got: %s", got)
	}
	if contains(string(data), "linear") {
		t.Error("wave mode should not include linear MCP")
	}
}

func TestGenerateMCPConfig_LinearMode_IncludesLinear(t *testing.T) {
	dir := t.TempDir()
	path, err := session.GenerateMCPConfig(dir, domain.ModeLinear, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !contains(string(data), "linear") {
		t.Error("linear mode should include linear MCP server")
	}
	if !contains(string(data), "mcp.linear.app") {
		t.Error("expected Linear MCP URL")
	}
}

func TestGenerateMCPConfig_NoOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	// Generate first time
	session.GenerateMCPConfig(dir, domain.ModeWave, false)
	// Second time without force should fail
	_, err := session.GenerateMCPConfig(dir, domain.ModeWave, false)
	if err == nil {
		t.Error("expected error when file exists without --force")
	}
}

func TestGenerateMCPConfig_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	session.GenerateMCPConfig(dir, domain.ModeWave, false)
	// Force overwrite
	_, err := session.GenerateMCPConfig(dir, domain.ModeLinear, true)
	if err != nil {
		t.Fatalf("force overwrite failed: %v", err)
	}
	data, _ := os.ReadFile(session.MCPConfigPath(dir))
	if !contains(string(data), "linear") {
		t.Error("force overwrite should produce linear config")
	}
}

func TestMCPConfigExists_Missing(t *testing.T) {
	exists, err := session.MCPConfigExists(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for missing file")
	}
}

func TestMCPConfigExists_Valid(t *testing.T) {
	dir := t.TempDir()
	session.GenerateMCPConfig(dir, domain.ModeWave, false)
	exists, err := session.MCPConfigExists(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected true for valid file")
	}
}

func TestMCPConfigExists_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := session.MCPConfigPath(dir)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("not json"), 0o644)
	_, err := session.MCPConfigExists(dir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
