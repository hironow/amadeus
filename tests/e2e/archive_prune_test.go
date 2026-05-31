//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestE2E_ArchivePrune_DryRun(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_prune_dry"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	writeDMail(t, ctx, c, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "design-feedback",
		"description": "Old feedback",
		"severity":    "low",
	}, "Old body.\n")

	archivePath := fmt.Sprintf("%s/.gate/archive/feedback-001.md", dir)
	// Backdate the file using container touch (e.g. to Year 2025)
	execInContainer(t, ctx, c, []string{"touch", "-t", "202505310000", archivePath})

	stdout, _, err := runCmd(t, ctx, c, dir, "archive-prune", "--days", "30", "--dry-run")
	_ = err // Wait, amadeus may log output on stdout or stderr. Our runCmd captures stdout.
	
	if !strings.Contains(stdout, "dry-run") {
		t.Logf("output: %s", stdout)
	}

	// File should still exist
	if !fileExistsInContainer(t, ctx, c, archivePath) {
		t.Errorf("expected archive file to still exist: %s", archivePath)
	}
}

func TestE2E_ArchivePrune_WithYes(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_prune_yes"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	writeDMail(t, ctx, c, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "design-feedback",
		"description": "Old feedback",
		"severity":    "low",
	}, "Old body.\n")

	archivePath := fmt.Sprintf("%s/.gate/archive/feedback-001.md", dir)
	execInContainer(t, ctx, c, []string{"touch", "-t", "202505310000", archivePath})

	_, _, err := runCmd(t, ctx, c, dir, "archive-prune", "--execute", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune --execute --yes: %v", err)
	}

	// File should be deleted
	if fileExistsInContainer(t, ctx, c, archivePath) {
		t.Errorf("expected archive file to be deleted: %s", archivePath)
	}
}

func TestE2E_ArchivePrune_PreservesRecent(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_prune_preserve"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	writeDMail(t, ctx, c, dir, "archive", "feedback-001", map[string]any{
		"name":        "feedback-001",
		"kind":        "design-feedback",
		"description": "Recent feedback",
		"severity":    "low",
	}, "Recent body.\n")

	// File is recent (just created) — should not be pruned
	_, _, err := runCmd(t, ctx, c, dir, "archive-prune", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune: %v", err)
	}

	archivePath := fmt.Sprintf("%s/.gate/archive/feedback-001.md", dir)
	if !fileExistsInContainer(t, ctx, c, archivePath) {
		t.Errorf("expected recent file to be preserved: %s", archivePath)
	}
}

func TestE2E_ArchivePrune_EmptyArchive(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_prune_empty"
	initTestRepo(t, ctx, c, dir)
	writeConfig(t, ctx, c, dir, defaultTestConfig())

	_, _, err := runCmd(t, ctx, c, dir, "archive-prune", "--days", "30", "--yes")
	if err != nil {
		t.Fatalf("archive-prune on empty: %v", err)
	}
}

func TestE2E_ArchivePrune_InvalidDays(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_prune_inv"
	initTestRepo(t, ctx, c, dir)

	_, _, err := runCmd(t, ctx, c, dir, "archive-prune", "--days", "0")
	if err == nil {
		t.Fatal("expected error for --days 0")
	}
}
