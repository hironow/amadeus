package session_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
)

// TestRace_OutboxStore_ConcurrentStageAndFlush verifies that concurrent
// Stage and Flush operations do not trigger the race detector.
func TestRace_OutboxStore_ConcurrentStageAndFlush(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)
	store := testOutboxStore(t, root)

	var wg sync.WaitGroup
	const workers = 10

	for i := range workers {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("race-%03d.md", id)
			store.Stage(name, []byte("data"))
		}(i)
		go func() {
			defer wg.Done()
			store.Flush()
		}()
	}
	wg.Wait()
}

// TestRace_OutboxStore_ConcurrentMultiStore verifies that two store
// connections to the same DB do not trigger the race detector.
func TestRace_OutboxStore_ConcurrentMultiStore(t *testing.T) {
	root := t.TempDir()
	ensureGateDirs(t, root)

	storeA, err := session.NewOutboxStoreForGateDir(root)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := session.NewOutboxStoreForGateDir(root)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	defer storeB.Close()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("a-%03d.md", id)
			storeA.Stage(name, []byte("data-a"))
			storeA.Flush()
		}(i)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("b-%03d.md", id)
			storeB.Stage(name, []byte("data-b"))
			storeB.Flush()
		}(i)
	}
	wg.Wait()
}

// TestRace_Logger_ConcurrentWrite verifies that Logger's mutex protects
// concurrent log writes.
func TestRace_Logger_ConcurrentWrite(t *testing.T) {
	logger := domain.NewLogger(nil, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("concurrent log %d", id)
		}(i)
	}
	wg.Wait()
}
