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

func TestFormatSuccessRate_WithEvents(t *testing.T) {
	// given
	rate := 0.857142
	success := 6
	total := 7

	// when
	msg := amadeus.FormatSuccessRate(rate, success, total)

	// then
	if msg != "85.7% (6/7)" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "85.7% (6/7)")
	}
}

func TestFormatSuccessRate_NoEvents(t *testing.T) {
	// given
	rate := 0.0
	success := 0
	total := 0

	// when
	msg := amadeus.FormatSuccessRate(rate, success, total)

	// then
	if msg != "no events" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "no events")
	}
}
