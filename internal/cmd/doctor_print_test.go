package cmd

// white-box-reason: cobra command construction: printDoctorJSON is unexported

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestPrintDoctorJSON_StatusLabelsAreKnown(t *testing.T) {
	// given: all 5 check statuses
	checks := []domain.DoctorCheck{
		{Name: "ok", Status: domain.CheckOK, Message: "ok"},
		{Name: "fail", Status: domain.CheckFail, Message: "fail"},
		{Name: "warn", Status: domain.CheckWarn, Message: "warn"},
		{Name: "skip", Status: domain.CheckSkip, Message: "skip"},
		{Name: "fix", Status: domain.CheckFixed, Message: "fix"},
	}

	// when
	var buf strings.Builder
	_ = printDoctorJSON(&buf, checks)

	// then: all status labels are in the known set
	known := map[string]bool{"OK": true, "FAIL": true, "SKIP": true, "WARN": true, "FIX": true}
	var parsed struct {
		Checks []struct {
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, c := range parsed.Checks {
		if !known[c.Status] {
			t.Errorf("unknown status label in doctor JSON: %q", c.Status)
		}
	}
}
