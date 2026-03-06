package platform_test

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
)

func TestOtelEnvContent_Jaeger(t *testing.T) {
	// when
	content, err := platform.OtelEnvContent("jaeger", "", "")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318") {
		t.Errorf("jaeger content missing endpoint: %q", content)
	}
}

func TestOtelEnvContent_Weave_Valid(t *testing.T) {
	// when
	content, err := platform.OtelEnvContent("weave", "my-team", "my-project")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "OTEL_EXPORTER_OTLP_ENDPOINT=https://trace.wandb.ai/otel") {
		t.Errorf("weave content missing endpoint: %q", content)
	}
	if !strings.Contains(content, "OTEL_EXPORTER_OTLP_HEADERS=wandb-api-key=${WANDB_API_KEY}") {
		t.Errorf("weave content missing headers: %q", content)
	}
	if !strings.Contains(content, "wandb.entity=my-team") {
		t.Errorf("weave content missing entity: %q", content)
	}
	if !strings.Contains(content, "wandb.project=my-project") {
		t.Errorf("weave content missing project: %q", content)
	}
}

func TestOtelEnvContent_Weave_MissingEntity(t *testing.T) {
	// when
	_, err := platform.OtelEnvContent("weave", "", "my-project")

	// then
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
	if !strings.Contains(err.Error(), "otel-entity") {
		t.Errorf("error should mention otel-entity: %v", err)
	}
}

func TestOtelEnvContent_Unknown(t *testing.T) {
	// when
	_, err := platform.OtelEnvContent("datadog", "", "")

	// then
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
	if !strings.Contains(err.Error(), "datadog") {
		t.Errorf("error should mention the unknown backend: %v", err)
	}
}

func TestOtelEnvContent_Empty(t *testing.T) {
	// when
	content, err := platform.OtelEnvContent("", "", "")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("empty backend should return empty content, got %q", content)
	}
}
