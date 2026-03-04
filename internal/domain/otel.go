package domain

import "fmt"

// OtelEnvContent returns the .otel.env file content for the given backend.
// Supported backends: "jaeger" (local OTLP HTTP), "weave" (Weights & Biases).
// Empty backend returns empty content (no-op). Unknown backends return an error.
func OtelEnvContent(backend, entity, project string) (string, error) {
	switch backend {
	case "jaeger":
		return "OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318\n", nil
	case "weave":
		if entity == "" || project == "" {
			return "", fmt.Errorf("weave requires --otel-entity and --otel-project")
		}
		return fmt.Sprintf(
			"OTEL_EXPORTER_OTLP_ENDPOINT=https://trace.wandb.ai\n"+
				"OTEL_EXPORTER_OTLP_HEADERS=wandb-api-key=${WANDB_API_KEY}\n"+
				"OTEL_RESOURCE_ATTRIBUTES=wandb.entity=%s,wandb.project=%s\n",
			entity, project), nil
	case "":
		return "", nil
	default:
		return "", fmt.Errorf("unknown otel backend: %q (supported: jaeger, weave)", backend)
	}
}
