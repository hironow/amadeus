package domain_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestCheckStatus_StatusLabel_AllKnown(t *testing.T) {
	known := map[domain.CheckStatus]string{
		domain.CheckOK:    "OK",
		domain.CheckFail:  "FAIL",
		domain.CheckSkip:  "SKIP",
		domain.CheckWarn:  "WARN",
		domain.CheckFixed: "FIX",
	}
	for status, want := range known {
		if got := status.StatusLabel(); got != want {
			t.Errorf("StatusLabel(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestCheckStatus_StatusLabel_Unknown_IsNotOK(t *testing.T) {
	unknown := domain.CheckStatus(99)
	label := unknown.StatusLabel()
	if label == "OK" {
		t.Error("unknown status must not map to OK (fail-open)")
	}
	if label != "????" {
		t.Errorf("unknown status label = %q, want %q", label, "????")
	}
}
