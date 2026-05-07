package session_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

func newProjectionFixture(t *testing.T) (*session.Projector, string) {
	t.Helper()
	root := t.TempDir()
	if _, err := session.EnsureStateDir(root, session.WithMailDirs()); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}
	outboxStore, err := session.NewOutboxStoreForDir(root)
	if err != nil {
		t.Fatalf("NewOutboxStoreForDir: %v", err)
	}
	t.Cleanup(func() { outboxStore.Close() })
	store := session.NewProjectionStore(root)
	return &session.Projector{Store: store, OutboxStore: outboxStore}, root
}

func TestProjector_DMailGenerated_InjectsProjectIDFromEnv(t *testing.T) {
	// given
	t.Setenv("RUNOPS_PROJECT_ID", "alpha-bridge")
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-projectid_00000000",
		Kind:          "design-feedback",
		Description:   "project_id injection",
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
	if got := parsed.Metadata["project_id"]; got != "alpha-bridge" {
		t.Errorf("project_id: got %q, want %q", got, "alpha-bridge")
	}
}

func TestProjector_DMailGenerated_OmitsProjectIDWhenUnresolved(t *testing.T) {
	// given — env unset + tmp HOME
	t.Setenv("RUNOPS_PROJECT_ID", "")
	t.Setenv("HOME", t.TempDir())
	projector, root := newProjectionFixture(t)
	dm := domain.DMail{
		Name:          "am-feedback-projectid-omit_00000000",
		Kind:          "design-feedback",
		Description:   "no project_id",
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

	// then — legacy single-mode: no project_id key
	outboxPath := filepath.Join(root, domain.StateDir, "outbox", dm.Name+".md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := domain.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	if _, present := parsed.Metadata["project_id"]; present {
		t.Errorf("project_id should be omitted when unresolved, got %q", parsed.Metadata["project_id"])
	}
}
