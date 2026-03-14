package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
)

func TestIndexEntry_MarshalJSON(t *testing.T) {
	entry := domain.IndexEntry{
		Timestamp: "2026-03-10T14:30:00Z",
		Operation: "divergence",
		Issue:     "ENG-123",
		Status:    "success",
		Tool:      "amadeus",
		Path:      "archive/report-auth.md",
		Summary:   "PR #42 merged: add auth middleware",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got domain.IndexEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != entry {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", got, entry)
	}
}

func TestIndexEntry_JSONFieldNames(t *testing.T) {
	entry := domain.IndexEntry{
		Timestamp: "2026-03-10T14:30:00Z",
		Operation: "divergence",
		Issue:     "ENG-123",
		Status:    "success",
		Tool:      "amadeus",
		Path:      "archive/report.md",
		Summary:   "test summary",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	for _, key := range []string{"ts", "op", "issue", "status", "tool", "path", "summary"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}
