package eventsource_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hironow/amadeus/internal/eventsource"
)

func TestSeqCounter_AllocSeqNr_Monotonic(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), "seq.db")
	counter, err := eventsource.NewSeqCounter(dbPath)
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer counter.Close()
	ctx := context.Background()

	// when — allocate 5 sequence numbers
	var seqs []uint64
	for i := 0; i < 5; i++ {
		seq, err := counter.AllocSeqNr(ctx)
		if err != nil {
			t.Fatalf("alloc %d: %v", i, err)
		}
		seqs = append(seqs, seq)
	}

	// then — strictly monotonic starting at 1
	for i, seq := range seqs {
		expected := uint64(i + 1)
		if seq != expected {
			t.Errorf("seqs[%d] = %d, want %d", i, seq, expected)
		}
	}
}

func TestSeqCounter_LatestSeqNr_ReturnsHighest(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), "seq.db")
	counter, err := eventsource.NewSeqCounter(dbPath)
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer counter.Close()
	ctx := context.Background()

	// when — allocate 3
	for i := 0; i < 3; i++ {
		if _, err := counter.AllocSeqNr(ctx); err != nil {
			t.Fatalf("alloc: %v", err)
		}
	}
	latest, err := counter.LatestSeqNr(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}

	// then
	if latest != 3 {
		t.Errorf("expected 3, got %d", latest)
	}
}

func TestSeqCounter_LatestSeqNr_ZeroWhenEmpty(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), "seq.db")
	counter, err := eventsource.NewSeqCounter(dbPath)
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer counter.Close()

	// when
	latest, err := counter.LatestSeqNr(context.Background())
	if err != nil {
		t.Fatalf("latest: %v", err)
	}

	// then
	if latest != 0 {
		t.Errorf("expected 0, got %d", latest)
	}
}

func TestSeqCounter_ConcurrentAlloc(t *testing.T) {
	// given — concurrent allocations within same process
	dbPath := filepath.Join(t.TempDir(), "seq.db")
	counter, err := eventsource.NewSeqCounter(dbPath)
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer counter.Close()
	ctx := context.Background()

	const goroutines = 10
	const allocsPerGoroutine = 10
	results := make(chan uint64, goroutines*allocsPerGoroutine)

	// when
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < allocsPerGoroutine; i++ {
				seq, err := counter.AllocSeqNr(ctx)
				if err != nil {
					t.Errorf("alloc: %v", err)
					return
				}
				results <- seq
			}
		}()
	}
	wg.Wait()
	close(results)

	// then — all values unique, total = goroutines * allocsPerGoroutine
	seen := make(map[uint64]bool)
	for seq := range results {
		if seen[seq] {
			t.Errorf("duplicate SeqNr: %d", seq)
		}
		seen[seq] = true
	}
	if len(seen) != goroutines*allocsPerGoroutine {
		t.Errorf("expected %d unique SeqNrs, got %d", goroutines*allocsPerGoroutine, len(seen))
	}
}

func TestSeqCounter_InitializeAt(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), "seq.db")
	counter, err := eventsource.NewSeqCounter(dbPath)
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer counter.Close()
	ctx := context.Background()

	// when — initialize at a specific value (for cutover)
	if err := counter.InitializeAt(ctx, 100); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	seq, err := counter.AllocSeqNr(ctx)
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}

	// then
	if seq != 101 {
		t.Errorf("expected 101 after initializing at 100, got %d", seq)
	}
}

func TestSeqCounter_InitializeAt_Idempotent(t *testing.T) {
	// given — already initialized
	dbPath := filepath.Join(t.TempDir(), "seq.db")
	counter, err := eventsource.NewSeqCounter(dbPath)
	if err != nil {
		t.Fatalf("new seq counter: %v", err)
	}
	defer counter.Close()
	ctx := context.Background()

	// when — initialize, allocate, then try to re-initialize
	if err := counter.InitializeAt(ctx, 50); err != nil {
		t.Fatalf("init: %v", err)
	}
	seq1, _ := counter.AllocSeqNr(ctx)
	// re-initialize should be a no-op if counter already advanced
	if err := counter.InitializeAt(ctx, 50); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	seq2, _ := counter.AllocSeqNr(ctx)

	// then — seq2 should follow seq1, not reset
	if seq1 != 51 {
		t.Errorf("seq1 = %d, want 51", seq1)
	}
	if seq2 != 52 {
		t.Errorf("seq2 = %d, want 52", seq2)
	}
}
