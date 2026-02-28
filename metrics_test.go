package amadeus_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/amadeus"
)

func makeCheckEvent(dmails []string) amadeus.Event {
	data, _ := json.Marshal(amadeus.CheckCompletedData{
		Result: amadeus.CheckResult{DMails: dmails},
	})
	return amadeus.Event{ID: "test", Type: amadeus.EventCheckCompleted, Timestamp: time.Now(), Data: data}
}

func TestSuccessRate_AllClean(t *testing.T) {
	events := []amadeus.Event{
		makeCheckEvent(nil),
		makeCheckEvent(nil),
	}

	rate := amadeus.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestSuccessRate_AllDrift(t *testing.T) {
	events := []amadeus.Event{
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent([]string{"feedback-002"}),
	}

	rate := amadeus.SuccessRate(events)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_Mixed(t *testing.T) {
	events := []amadeus.Event{
		makeCheckEvent(nil),
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent(nil),
	}

	rate := amadeus.SuccessRate(events)

	if rate < 0.66 || rate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", rate)
	}
}

func TestSuccessRate_NoEvents(t *testing.T) {
	rate := amadeus.SuccessRate(nil)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_IgnoresOtherEvents(t *testing.T) {
	events := []amadeus.Event{
		{ID: "1", Type: amadeus.EventBaselineUpdated, Timestamp: time.Now()},
		makeCheckEvent(nil),
		{ID: "3", Type: amadeus.EventDMailGenerated, Timestamp: time.Now()},
	}

	rate := amadeus.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}
