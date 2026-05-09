package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform/actortype"
)

func TestProjector_DMailGenerated_InjectsActorTypeFromEnv(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "ai-agent")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "")
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-actortype-env_00000000",
		Kind:          "design-feedback",
		Description:   "with actor type",
		SchemaVersion: domain.DMailSchemaVersion,
		Body:          "# Hello\n",
	}
	payload, err := json.Marshal(domain.DMailGeneratedData{DMail: dm})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := domain.Event{
		Type:      domain.EventDMailGenerated,
		Data:      payload,
		Timestamp: time.Now(),
	}

	// when
	if err := projector.Apply(context.Background(), event); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then
	outboxPath := filepath.Join(root, domain.StateDir, "outbox", dm.Name+".md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	if got := parsed.Metadata["requester_actor_type"]; got != "ai-agent" {
		t.Errorf("requester_actor_type: got %q, want ai-agent", got)
	}
	if got := parsed.Metadata["requester_actor_source"]; got != "env" {
		t.Errorf("requester_actor_source: got %q, want env", got)
	}
	if _, ok := parsed.Metadata["initiating_actor_type"]; ok {
		t.Errorf("non-daemon actor must not carry initiating_actor_type")
	}
}

func TestProjector_DMailGenerated_InjectsActorTypeWithInitiating(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "human-operator")
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-actortype-daemon_00000000",
		Kind:          "design-feedback",
		Description:   "daemon-driven",
		SchemaVersion: domain.DMailSchemaVersion,
		Body:          "# Hello\n",
	}
	payload, err := json.Marshal(domain.DMailGeneratedData{DMail: dm})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := domain.Event{
		Type:      domain.EventDMailGenerated,
		Data:      payload,
		Timestamp: time.Now(),
	}

	// when
	if err := projector.Apply(context.Background(), event); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then
	outboxPath := filepath.Join(root, domain.StateDir, "outbox", dm.Name+".md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	if got := parsed.Metadata["requester_actor_type"]; got != "workspace-daemon" {
		t.Errorf("requester_actor_type: got %q, want workspace-daemon", got)
	}
	if got := parsed.Metadata["initiating_actor_type"]; got != "human-operator" {
		t.Errorf("initiating_actor_type: got %q, want human-operator", got)
	}
}

func TestProjector_DMailGenerated_OmitsActorTypeWhenUnresolved(t *testing.T) {
	// given — env unset (legacy compat path)
	t.Setenv("RUNOPS_ACTOR_TYPE", "")
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-actortype-legacy_00000000",
		Kind:          "design-feedback",
		Description:   "legacy compat",
		SchemaVersion: domain.DMailSchemaVersion,
		Body:          "# Hello\n",
	}
	payload, err := json.Marshal(domain.DMailGeneratedData{DMail: dm})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := domain.Event{
		Type:      domain.EventDMailGenerated,
		Data:      payload,
		Timestamp: time.Now(),
	}

	// when
	if err := projector.Apply(context.Background(), event); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// then — frontmatter must not carry actor type lines (byte-identical legacy)
	outboxPath := filepath.Join(root, domain.StateDir, "outbox", dm.Name+".md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	if v, ok := parsed.Metadata["requester_actor_type"]; ok {
		t.Errorf("requester_actor_type must be absent in legacy compat path, got %q", v)
	}
	if v, ok := parsed.Metadata["requester_actor_source"]; ok {
		t.Errorf("requester_actor_source must be absent in legacy compat path, got %q", v)
	}
}

func TestProjector_DMailGenerated_InvalidActorTypeReturnsError(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "robot")
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-actortype-invalid_00000000",
		Kind:          "design-feedback",
		Description:   "invalid env",
		SchemaVersion: domain.DMailSchemaVersion,
		Body:          "# Hello\n",
	}
	payload, err := json.Marshal(domain.DMailGeneratedData{DMail: dm})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := domain.Event{
		Type:      domain.EventDMailGenerated,
		Data:      payload,
		Timestamp: time.Now(),
	}

	// when
	applyErr := projector.Apply(context.Background(), event)

	// then
	if applyErr == nil {
		t.Fatal("expected error for invalid RUNOPS_ACTOR_TYPE env, got nil")
	}
	if !errors.Is(applyErr, actortype.ErrInvalidActorType) {
		t.Errorf("expected ErrInvalidActorType wrapped, got %v", applyErr)
	}
	// outbox MUST NOT have the file (silent escalation prevention)
	outboxPath := filepath.Join(root, domain.StateDir, "outbox", dm.Name+".md")
	if _, statErr := os.Stat(outboxPath); statErr == nil {
		t.Errorf("outbox file must not exist after apply fail, but %s does", outboxPath)
	}
}

func TestProjector_DMailGenerated_DaemonInvalidInitiatingReturnsError(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "robot")
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-actortype-daemoninvalid_00000000",
		Kind:          "design-feedback",
		Description:   "daemon with invalid initiating",
		SchemaVersion: domain.DMailSchemaVersion,
		Body:          "# Hello\n",
	}
	payload, err := json.Marshal(domain.DMailGeneratedData{DMail: dm})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := domain.Event{
		Type:      domain.EventDMailGenerated,
		Data:      payload,
		Timestamp: time.Now(),
	}

	// when
	applyErr := projector.Apply(context.Background(), event)

	// then
	if applyErr == nil {
		t.Fatal("expected error for invalid RUNOPS_INITIATING_ACTOR_TYPE env, got nil")
	}
	if !errors.Is(applyErr, actortype.ErrInvalidInitiatingActorType) {
		t.Errorf("expected ErrInvalidInitiatingActorType wrapped, got %v", applyErr)
	}
	outboxPath := filepath.Join(root, domain.StateDir, "outbox", dm.Name+".md")
	if _, statErr := os.Stat(outboxPath); statErr == nil {
		t.Errorf("outbox file must not exist after apply fail, but %s does", outboxPath)
	}
}
