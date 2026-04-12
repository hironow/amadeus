package domain

// white-box-reason: tests unexported seqNr field increment on CheckAggregate

import (
	"testing"
	"time"
)

func TestCheckAggregate_SeqNrIncrements(t *testing.T) {
	agg := NewCheckAggregate(Config{})
	now := time.Now()

	ev1, err := agg.RecordRunStarted(RunStartedData{}, now)
	if err != nil {
		t.Fatal(err)
	}

	if ev1.SeqNr != 1 {
		t.Errorf("ev1.SeqNr = %d, want 1", ev1.SeqNr)
	}
	if ev1.AggregateType != AggregateTypeCheck {
		t.Errorf("ev1.AggregateType = %q, want %q", ev1.AggregateType, AggregateTypeCheck)
	}

	// Second event should increment
	ev2, err := agg.RecordRunStopped(RunStoppedData{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if ev2.SeqNr != 2 {
		t.Errorf("ev2.SeqNr = %d, want 2", ev2.SeqNr)
	}
}
