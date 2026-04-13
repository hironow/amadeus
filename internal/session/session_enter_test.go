package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func TestEnterSession_WorkDirRequired(t *testing.T) {
	err := session.EnterSession(t.Context(), session.EnterConfig{
		ProviderCmd:       "echo",
		ProviderSessionID: "test-session",
		WorkDir:           "",
	})
	if err == nil {
		t.Fatal("expected error for empty WorkDir")
	}
	if !strings.Contains(err.Error(), "WorkDir") {
		t.Errorf("error should mention WorkDir: %v", err)
	}
}

func TestEnterSession_ProviderSessionIDRequired(t *testing.T) {
	err := session.EnterSession(t.Context(), session.EnterConfig{
		ProviderCmd:       "echo",
		ProviderSessionID: "",
		WorkDir:           t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for empty ProviderSessionID")
	}
	if !strings.Contains(err.Error(), "ProviderSessionID") {
		t.Errorf("error should mention ProviderSessionID: %v", err)
	}
}

func TestBuildIsolationFlags_AlwaysIncludesBase(t *testing.T) {
	cfg := session.EnterConfig{
		ProviderSessionID: "sess-001",
		WorkDir:           t.TempDir(),
		ConfigBase:        t.TempDir(),
	}
	args := session.ExportBuildIsolationFlags(cfg.ConfigBase)

	// --setting-sources "" and --disable-slash-commands are always present
	if !containsFlag(args, "--setting-sources") {
		t.Error("missing --setting-sources flag")
	}
	if !containsFlag(args, "--disable-slash-commands") {
		t.Error("missing --disable-slash-commands flag")
	}
}

func TestBuildIsolationFlags_SettingsPathWhenExists(t *testing.T) {
	base := t.TempDir()
	settingsDir := filepath.Join(base, domain.StateDir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("{}"), 0644)

	cfg := session.EnterConfig{ConfigBase: base}
	args := session.ExportBuildIsolationFlags(cfg.ConfigBase)

	if !containsFlag(args, "--settings") {
		t.Error("expected --settings flag when settings.json exists")
	}
	// Verify the path does NOT double-nest state dir
	for i, a := range args {
		if a == "--settings" && i+1 < len(args) {
			path := args[i+1]
			doubleNest := filepath.Join(domain.StateDir, domain.StateDir)
			if strings.Contains(path, doubleNest) {
				t.Errorf("settings path double-nested: %s", path)
			}
		}
	}
}

func TestBuildIsolationFlags_MCPConfigPathWhenExists(t *testing.T) {
	base := t.TempDir()
	mcpDir := filepath.Join(base, domain.StateDir)
	os.MkdirAll(mcpDir, 0755)
	os.WriteFile(filepath.Join(mcpDir, ".mcp.json"), []byte("{}"), 0644)

	cfg := session.EnterConfig{ConfigBase: base}
	args := session.ExportBuildIsolationFlags(cfg.ConfigBase)

	if !containsFlag(args, "--strict-mcp-config") {
		t.Error("expected --strict-mcp-config flag when .mcp.json exists")
	}
	if !containsFlag(args, "--mcp-config") {
		t.Error("expected --mcp-config flag when .mcp.json exists")
	}
}

func TestBuildIsolationFlags_ResumeFlag(t *testing.T) {
	// EnterSession appends --resume after isolation flags
	// buildIsolationFlags itself does not add --resume
	cfg := session.EnterConfig{
		ProviderSessionID: "sess-resume-001",
		ConfigBase:        t.TempDir(),
	}
	args := session.ExportBuildIsolationFlags(cfg.ConfigBase)

	// --resume is NOT in isolation flags (added by EnterSession)
	if containsFlag(args, "--resume") {
		t.Error("--resume should not be in isolation flags")
	}
}

// --- Execution contract tests (fake provider) ---

func TestEnterSession_ResumeArgPassedToProvider(t *testing.T) {
	// given: "echo" as fake provider — prints all args to stdout
	var stdout strings.Builder
	workDir := t.TempDir()
	configBase := t.TempDir()
	cfg := session.EnterConfig{
		ProviderCmd:       "echo",
		ProviderSessionID: "sess-exec-001",
		WorkDir:           workDir,
		ConfigBase:        configBase,
		IsolationFlags:    session.BuildClaudeIsolationFlags(configBase),
		Stdout:            &stdout,
		Stderr:            &strings.Builder{},
	}

	// when
	err := session.EnterSession(t.Context(), cfg)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "--resume sess-exec-001") {
		t.Errorf("expected --resume sess-exec-001 in output: %q", output)
	}
	if !strings.Contains(output, "--disable-slash-commands") {
		t.Errorf("expected --disable-slash-commands in output: %q", output)
	}
}

func TestEnterSession_WorkDirSetAsCmdDir(t *testing.T) {
	// given: fake provider script that prints working directory
	workDir := t.TempDir()
	fakeProvider := filepath.Join(workDir, "fake-pwd.sh")
	os.WriteFile(fakeProvider, []byte("#!/bin/sh\npwd\n"), 0755)

	var stdout strings.Builder
	cfg := session.EnterConfig{
		ProviderCmd:       fakeProvider,
		ProviderSessionID: "sess-pwd-001",
		WorkDir:           workDir,
		ConfigBase:        t.TempDir(),
		Stdout:            &stdout,
		Stderr:            &strings.Builder{},
	}

	// when
	err := session.EnterSession(t.Context(), cfg)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks for macOS /var -> /private/var
	resolved, _ := filepath.EvalSymlinks(workDir)
	if !strings.Contains(stdout.String(), resolved) {
		t.Errorf("expected WorkDir %q in pwd output: %q", resolved, stdout.String())
	}
}

func TestEnterSession_StdinStdoutStderrPassthrough(t *testing.T) {
	// given: a fake provider script that reads stdin, writes to stdout and stderr
	workDir := t.TempDir()
	fakeProvider := filepath.Join(workDir, "fake-provider.sh")
	os.WriteFile(fakeProvider, []byte("#!/bin/sh\nread line; echo \"out:$line\"; echo \"err:$line\" >&2\n"), 0755)

	input := "passthrough-test\n"
	var stdout, stderr strings.Builder
	cfg := session.EnterConfig{
		ProviderCmd:       fakeProvider,
		ProviderSessionID: "sess-passthrough",
		WorkDir:           workDir,
		ConfigBase:        t.TempDir(),
		Stdin:             strings.NewReader(input),
		Stdout:            &stdout,
		Stderr:            &stderr,
	}

	// when
	err := session.EnterSession(t.Context(), cfg)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "out:passthrough-test") {
		t.Errorf("expected stdout passthrough, got: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "err:passthrough-test") {
		t.Errorf("expected stderr passthrough, got: %q", stderr.String())
	}
}

func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
