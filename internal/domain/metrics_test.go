package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func makeCheckEvent(dmails []string) domain.Event {
	data, _ := json.Marshal(domain.CheckCompletedData{
		Result: domain.CheckResult{DMails: dmails},
	})
	return domain.Event{ID: "test", Type: domain.EventCheckCompleted, Timestamp: time.Now(), Data: data}
}

func TestSuccessRate_AllClean(t *testing.T) {
	events := []domain.Event{
		makeCheckEvent(nil),
		makeCheckEvent(nil),
	}

	rate := domain.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestSuccessRate_AllDrift(t *testing.T) {
	events := []domain.Event{
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent([]string{"feedback-002"}),
	}

	rate := domain.SuccessRate(events)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_Mixed(t *testing.T) {
	events := []domain.Event{
		makeCheckEvent(nil),
		makeCheckEvent([]string{"feedback-001"}),
		makeCheckEvent(nil),
	}

	rate := domain.SuccessRate(events)

	if rate < 0.66 || rate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", rate)
	}
}

func TestSuccessRate_NoEvents(t *testing.T) {
	rate := domain.SuccessRate(nil)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_IgnoresOtherEvents(t *testing.T) {
	events := []domain.Event{
		{ID: "1", Type: domain.EventBaselineUpdated, Timestamp: time.Now()},
		makeCheckEvent(nil),
		{ID: "3", Type: domain.EventDMailGenerated, Timestamp: time.Now()},
	}

	rate := domain.SuccessRate(events)

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
	msg := domain.FormatSuccessRate(rate, success, total)

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
	msg := domain.FormatSuccessRate(rate, success, total)

	// then
	if msg != "no events" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "no events")
	}
}
