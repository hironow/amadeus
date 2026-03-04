package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyOtelEnv_SetsVars(t *testing.T) {
	// given
	dir := t.TempDir()
	content := "OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318\n"
	if err := os.WriteFile(filepath.Join(dir, ".otel.env"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	applyOtelEnv(dir)

	// then
	got := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got != "http://localhost:4318" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want %q", got, "http://localhost:4318")
	}

	// cleanup
	t.Cleanup(func() { os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT") })
}

func TestApplyOtelEnv_ExistingEnvTakesPrecedence(t *testing.T) {
	// given
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://existing:4318")
	dir := t.TempDir()
	content := "OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318\n"
	if err := os.WriteFile(filepath.Join(dir, ".otel.env"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	applyOtelEnv(dir)

	// then
	got := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got != "http://existing:4318" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want %q (existing should win)", got, "http://existing:4318")
	}
}

func TestApplyOtelEnv_ExpandsVarRef(t *testing.T) {
	// given
	t.Setenv("WANDB_API_KEY", "secret-key-123")
	dir := t.TempDir()
	content := "OTEL_EXPORTER_OTLP_HEADERS=wandb-api-key=${WANDB_API_KEY}\n"
	if err := os.WriteFile(filepath.Join(dir, ".otel.env"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	applyOtelEnv(dir)

	// then
	got := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")
	if got != "wandb-api-key=secret-key-123" {
		t.Errorf("OTEL_EXPORTER_OTLP_HEADERS = %q, want %q", got, "wandb-api-key=secret-key-123")
	}

	// cleanup
	t.Cleanup(func() { os.Unsetenv("OTEL_EXPORTER_OTLP_HEADERS") })
}

func TestApplyOtelEnv_MissingFile_Noop(t *testing.T) {
	// given
	dir := t.TempDir() // no .otel.env file

	// when — should not panic or error
	applyOtelEnv(dir)

	// then — no env vars set
	if got := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); got != "" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT unexpectedly set: %q", got)
	}
}

func TestApplyOtelEnv_SkipsComments(t *testing.T) {
	// given
	dir := t.TempDir()
	content := "# This is a comment\n\nOTEL_RESOURCE_ATTRIBUTES=wandb.entity=team\n# Another comment\n"
	if err := os.WriteFile(filepath.Join(dir, ".otel.env"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	applyOtelEnv(dir)

	// then
	got := os.Getenv("OTEL_RESOURCE_ATTRIBUTES")
	if got != "wandb.entity=team" {
		t.Errorf("OTEL_RESOURCE_ATTRIBUTES = %q, want %q", got, "wandb.entity=team")
	}

	// cleanup
	t.Cleanup(func() { os.Unsetenv("OTEL_RESOURCE_ATTRIBUTES") })
}
