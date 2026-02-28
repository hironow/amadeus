package amadeus

import (
	"encoding/json"
	"testing"
	"time"
)

func makeCheckEvent(dmails []string) Event {
	data, _ := json.Marshal(CheckCompletedData{
		Result: CheckResult{DMails: dmails},
	})
	return Event{ID: "test", Type: EventCheckCompleted, Timestamp: time.Now(), Data: data}
}

func TestSuccessRate_AllClean(t *testing.T) {
	events := []Event{
		makeCheckEvent(nil),
		makeCheckEvent(nil),
	}

	rate := SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestSuccessRate_AllDrift(t *testing.T) {
	events := []Event{
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent([]string{"feedback-002"}),
	}

	rate := SuccessRate(events)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_Mixed(t *testing.T) {
	events := []Event{
		makeCheckEvent(nil),
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent(nil),
	}

	rate := SuccessRate(events)

	if rate < 0.66 || rate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", rate)
	}
}

func TestSuccessRate_NoEvents(t *testing.T) {
	rate := SuccessRate(nil)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_IgnoresOtherEvents(t *testing.T) {
	events := []Event{
		{ID: "1", Type: EventBaselineUpdated, Timestamp: time.Now()},
		makeCheckEvent(nil),
		{ID: "3", Type: EventDMailGenerated, Timestamp: time.Now()},
	}

	rate := SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}
