package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHook_NewHook(t *testing.T) {
	// given
	gitDir := t.TempDir()
	os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755)

	// when
	err := InstallHook(gitDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hookPath := filepath.Join(gitDir, "hooks", "post-merge")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "#!/bin/sh\n") {
		t.Errorf("expected shebang, got: %q", content[:20])
	}
	if !strings.Contains(content, hookMarkerBegin) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, hookMarkerEnd) {
		t.Error("missing end marker")
	}
	if !strings.Contains(content, "amadeus check --quiet") {
		t.Error("missing amadeus check command")
	}
	// Verify executable permissions
	info, _ := os.Stat(hookPath)
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("hook file is not executable")
	}
}

func TestInstallHook_AppendToExisting(t *testing.T) {
	// given
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	existing := "#!/bin/sh\necho 'existing hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "post-merge"), []byte(existing), 0o755)

	// when
	err := InstallHook(gitDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-merge"))
	content := string(data)
	if !strings.HasPrefix(content, "#!/bin/sh\necho 'existing hook'\n") {
		t.Error("existing content was modified")
	}
	if !strings.Contains(content, hookMarkerBegin) {
		t.Error("missing amadeus section")
	}
}

func TestInstallHook_AlreadyInstalled(t *testing.T) {
	// given
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	InstallHook(gitDir)

	// when
	err := InstallHook(gitDir)

	// then
	if err == nil {
		t.Fatal("expected error for double install")
	}
	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallHook_CreatesHooksDir(t *testing.T) {
	// given
	gitDir := t.TempDir()
	// hooks/ dir does not exist

	// when
	err := InstallHook(gitDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "hooks", "post-merge")); err != nil {
		t.Error("hook file was not created")
	}
}

func TestUninstallHook_RemovesAmadeusOnly(t *testing.T) {
	// given
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	existing := "#!/bin/sh\necho 'existing hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "post-merge"), []byte(existing), 0o755)
	InstallHook(gitDir)

	// when
	err := UninstallHook(gitDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-merge"))
	content := string(data)
	if strings.Contains(content, hookMarkerBegin) {
		t.Error("amadeus section was not removed")
	}
	if !strings.Contains(content, "echo 'existing hook'") {
		t.Error("existing content was removed")
	}
}

func TestUninstallHook_RemovesFileWhenAmadeusOnly(t *testing.T) {
	// given
	gitDir := t.TempDir()
	os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755)
	InstallHook(gitDir)

	// when
	err := UninstallHook(gitDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "hooks", "post-merge")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("hook file should have been removed")
	}
}

func TestUninstallHook_NoHookFile(t *testing.T) {
	// given
	gitDir := t.TempDir()

	// when
	err := UninstallHook(gitDir)

	// then
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no post-merge hook found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUninstallHook_NoAmadeusSection(t *testing.T) {
	// given
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	os.WriteFile(filepath.Join(hooksDir, "post-merge"), []byte("#!/bin/sh\necho hi\n"), 0o755)

	// when
	err := UninstallHook(gitDir)

	// then
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUninstallHook_EndMarkerBeforeBegin(t *testing.T) {
	// given: a malformed hook where end marker appears before begin marker
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	malformed := "#!/bin/sh\n" + hookMarkerEnd + "\nsome stuff\n" + hookMarkerBegin + "\namadeus check\n"
	os.WriteFile(filepath.Join(hooksDir, "post-merge"), []byte(malformed), 0o755)

	// when
	err := UninstallHook(gitDir)

	// then: should report malformed, not silently produce garbled output
	if err == nil {
		t.Fatal("expected error for malformed hook with end marker before begin")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("expected 'malformed' in error, got: %v", err)
	}
}
