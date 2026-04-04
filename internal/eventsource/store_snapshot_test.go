package eventsource_test

import (
	"context"
	"testing"

	"github.com/hironow/amadeus/internal/eventsource"
)

func TestFileSnapshotStore_SaveAndLoad(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileSnapshotStore(dir)
	ctx := context.Background()
	state := []byte(`{"key":"value"}`)

	// when
	if err := store.Save(ctx, "phonewave.state", 42, state); err != nil {
		t.Fatalf("save: %v", err)
	}
	seqNr, loaded, err := store.Load(ctx, "phonewave.state")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// then
	if seqNr != 42 {
		t.Errorf("expected seqNr 42, got %d", seqNr)
	}
	if string(loaded) != string(state) {
		t.Errorf("expected state %s, got %s", state, loaded)
	}
}

func TestFileSnapshotStore_LoadNonexistent(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileSnapshotStore(dir)

	// when
	seqNr, state, err := store.Load(context.Background(), "nonexistent")

	// then — graceful degradation: no error, zero values
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seqNr != 0 {
		t.Errorf("expected seqNr 0, got %d", seqNr)
	}
	if state != nil {
		t.Errorf("expected nil state, got %v", state)
	}
}

func TestFileSnapshotStore_Overwrite(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileSnapshotStore(dir)
	ctx := context.Background()

	// save initial
	if err := store.Save(ctx, "test.proj", 10, []byte(`"old"`)); err != nil {
		t.Fatalf("save: %v", err)
	}

	// when — overwrite with newer snapshot
	if err := store.Save(ctx, "test.proj", 20, []byte(`"new"`)); err != nil {
		t.Fatalf("save: %v", err)
	}
	seqNr, state, err := store.Load(ctx, "test.proj")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// then
	if seqNr != 20 {
		t.Errorf("expected seqNr 20, got %d", seqNr)
	}
	if string(state) != `"new"` {
		t.Errorf("expected state '\"new\"', got %s", state)
	}
}

func TestFileSnapshotStore_MultipleAggregateTypes(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileSnapshotStore(dir)
	ctx := context.Background()

	// when — save two different aggregate types
	if err := store.Save(ctx, "type.a", 5, []byte(`"a-state"`)); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := store.Save(ctx, "type.b", 10, []byte(`"b-state"`)); err != nil {
		t.Fatalf("save b: %v", err)
	}

	// then — each has its own snapshot
	seqA, stateA, _ := store.Load(ctx, "type.a")
	seqB, stateB, _ := store.Load(ctx, "type.b")

	if seqA != 5 || string(stateA) != `"a-state"` {
		t.Errorf("type.a: expected (5, \"a-state\"), got (%d, %s)", seqA, stateA)
	}
	if seqB != 10 || string(stateB) != `"b-state"` {
		t.Errorf("type.b: expected (10, \"b-state\"), got (%d, %s)", seqB, stateB)
	}
}
