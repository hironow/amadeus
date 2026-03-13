package platform_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/platform"
)

func TestIsNDJSON_EmptyString(t *testing.T) {
	if platform.IsNDJSON("") {
		t.Error("expected empty string to not be NDJSON")
	}
}

func TestIsNDJSON_RegularText(t *testing.T) {
	if platform.IsNDJSON("some regular error message") {
		t.Error("expected regular text to not be NDJSON")
	}
}

func TestIsNDJSON_MultiLineRegularText(t *testing.T) {
	if platform.IsNDJSON("error line 1\nerror line 2\n") {
		t.Error("expected multi-line regular text to not be NDJSON")
	}
}

func TestIsNDJSON_SingleJSONLine(t *testing.T) {
	if !platform.IsNDJSON(`{"type":"result"}`) {
		t.Error("expected single JSON line to be NDJSON")
	}
}

func TestIsNDJSON_MultipleJSONLines(t *testing.T) {
	input := "{\"type\":\"result\"}\n{\"type\":\"assistant\"}"
	if !platform.IsNDJSON(input) {
		t.Error("expected multiple JSON lines to be NDJSON")
	}
}

func TestIsNDJSON_LeadingEmptyLines(t *testing.T) {
	input := "\n\n{\"type\":\"result\"}\n"
	if !platform.IsNDJSON(input) {
		t.Error("expected NDJSON with leading empty lines to be detected")
	}
}

func TestIsNDJSON_WhitespaceOnlyLines(t *testing.T) {
	if platform.IsNDJSON("   \n  \n") {
		t.Error("expected whitespace-only to not be NDJSON")
	}
}

func TestSummarizeNDJSON_CountsLines(t *testing.T) {
	input := "{\"type\":\"result\"}\n{\"type\":\"assistant\"}\n{\"type\":\"system\"}"
	got := platform.SummarizeNDJSON(input)
	want := "(3 lines of stream-json output, use --verbose to see)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSummarizeNDJSON_SingleLine(t *testing.T) {
	got := platform.SummarizeNDJSON(`{"type":"result"}`)
	want := "(1 lines of stream-json output, use --verbose to see)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSummarizeNDJSON_IgnoresEmptyLines(t *testing.T) {
	input := "{\"type\":\"result\"}\n\n{\"type\":\"assistant\"}\n"
	got := platform.SummarizeNDJSON(input)
	want := "(2 lines of stream-json output, use --verbose to see)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
