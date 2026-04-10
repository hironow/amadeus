package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
)

// --- STRUCTURAL tests: Hint field rendering ---
// NOTE: Behavioral hint tests for check functions moved to internal/session/doctor_hint_test.go

func TestPrintDoctorJSON_IncludesHint(t *testing.T) {
	// given
	results := []domain.DoctorCheck{
		{Name: "test", Status: domain.CheckFail, Message: "failed", Hint: "fix it"},
	}

	// when
	var buf bytes.Buffer
	_ = printDoctorJSON(&buf, results)

	// then
	var parsed struct {
		Checks []jsonCheck `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Checks[0].Hint != "fix it" {
		t.Errorf("hint = %q, want 'fix it'", parsed.Checks[0].Hint)
	}
}

func TestPrintDoctorJSON_OmitsEmptyHint(t *testing.T) {
	// given
	results := []domain.DoctorCheck{
		{Name: "test", Status: domain.CheckOK, Message: "ok"},
	}

	// when
	var buf bytes.Buffer
	_ = printDoctorJSON(&buf, results)

	// then
	if strings.Contains(buf.String(), "hint") {
		t.Error("hint should be omitted when empty")
	}
}

func TestPrintDoctorText_ShowsHint(t *testing.T) {
	// given
	results := []domain.DoctorCheck{
		{Name: "test", Status: domain.CheckFail, Message: "failed", Hint: "run init"},
	}

	// when
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	_ = printDoctorText(&buf, logger, results)

	// then
	if !strings.Contains(buf.String(), "hint: run init") {
		t.Errorf("expected hint in text output, got: %s", buf.String())
	}
}
