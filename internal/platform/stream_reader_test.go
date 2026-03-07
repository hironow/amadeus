package platform_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform"
)

func TestParseStreamMessage_assistant(t *testing.T) {
	line := `{"type":"assistant","session_id":"sess-1","message":{"id":"msg_1","role":"assistant","content":[{"type":"text","text":"hello"}],"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":50}}}`

	msg, err := platform.ParseStreamMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "assistant" {
		t.Errorf("got type %q, want assistant", msg.Type)
	}
	if msg.SessionID != "sess-1" {
		t.Errorf("got session_id %q, want sess-1", msg.SessionID)
	}
}

func TestParseStreamMessage_result(t *testing.T) {
	line := `{"type":"result","subtype":"success","session_id":"sess-1","result":"done","total_cost_usd":0.15,"num_turns":5,"duration_ms":45000,"usage":{"input_tokens":5000,"output_tokens":2000}}`

	msg, err := platform.ParseStreamMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "result" {
		t.Errorf("got type %q, want result", msg.Type)
	}
	if msg.Result != "done" {
		t.Errorf("got result %q, want done", msg.Result)
	}
	if msg.Usage == nil {
		t.Fatal("usage is nil")
	}
	if msg.Usage.InputTokens != 5000 {
		t.Errorf("got input_tokens %d, want 5000", msg.Usage.InputTokens)
	}
}

func TestParseStreamMessage_unknown_type(t *testing.T) {
	line := `{"type":"future_type","session_id":"sess-1"}`

	msg, err := platform.ParseStreamMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "future_type" {
		t.Errorf("got type %q, want future_type", msg.Type)
	}
}

func TestParseStreamMessage_invalid_json(t *testing.T) {
	_, err := platform.ParseStreamMessage([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStreamReader_reads_all_message_types(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s1","model":"opus"}`,
		`{"type":"assistant","session_id":"s1","message":{"content":[{"type":"text","text":"hi"}]}}`,
		`{"type":"result","subtype":"success","session_id":"s1","result":"done","usage":{"input_tokens":100,"output_tokens":50}}`,
	}, "\n")

	reader := platform.NewStreamReader(strings.NewReader(input))
	types := []string{}
	for {
		msg, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		types = append(types, msg.Type)
	}

	if len(types) != 3 {
		t.Fatalf("got %d messages, want 3", len(types))
	}
	want := []string{"system", "assistant", "result"}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("msg[%d] type = %q, want %q", i, types[i], w)
		}
	}
}

func TestStreamReader_skips_invalid_json_lines(t *testing.T) {
	input := "not json\n" + `{"type":"result","result":"ok"}` + "\n"
	reader := platform.NewStreamReader(strings.NewReader(input))

	msg, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "result" {
		t.Errorf("got type %q, want result", msg.Type)
	}

	_, err = reader.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestStreamReader_skips_empty_lines(t *testing.T) {
	input := "\n\n" + `{"type":"result","result":"ok"}` + "\n\n"
	reader := platform.NewStreamReader(strings.NewReader(input))

	msg, err := reader.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != "result" {
		t.Errorf("got type %q, want result", msg.Type)
	}
}

func TestStreamReader_CollectAll(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"assistant","session_id":"s1","message":{"content":[{"type":"text","text":"hello world"}]}}`,
		`{"type":"result","subtype":"success","session_id":"s1","result":"hello world","usage":{"input_tokens":100,"output_tokens":50},"total_cost_usd":0.01,"num_turns":1}`,
	}, "\n")

	sr := platform.NewStreamReader(strings.NewReader(input))
	result, messages, err := sr.CollectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Result != "hello world" {
		t.Errorf("got result %q, want 'hello world'", result.Result)
	}
	if len(messages) != 3 {
		t.Errorf("got %d messages, want 3", len(messages))
	}
}

func TestExtractText_concatenates_text_blocks(t *testing.T) {
	line := `{"type":"assistant","session_id":"s1","message":{"content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"hello "},{"type":"text","text":"world"},{"type":"tool_use","name":"Read","id":"t1","input":{"file_path":"/tmp/x"}}]}}`

	msg, err := platform.ParseStreamMessage([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	text, err := msg.ExtractText()
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Errorf("got %q, want 'hello world'", text)
	}
}

func TestExtractToolUse_returns_tool_blocks(t *testing.T) {
	line := `{"type":"assistant","session_id":"s1","message":{"content":[{"type":"text","text":"reading"},{"type":"tool_use","name":"Read","id":"t1","input":{"file_path":"/tmp/x"}},{"type":"tool_use","name":"Write","id":"t2","input":{"file_path":"/tmp/y"}}]}}`

	msg, err := platform.ParseStreamMessage([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	tools, err := msg.ExtractToolUse()
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("got %d tool_use blocks, want 2", len(tools))
	}
	if tools[0].Name != "Read" {
		t.Errorf("got name %q, want Read", tools[0].Name)
	}
}

func TestStreamReader_propagates_read_errors(t *testing.T) {
	r := &errReader{data: `{"type":"system"}` + "\n", err: io.ErrUnexpectedEOF}
	sr := platform.NewStreamReader(r)

	msg, err := sr.Next()
	if err != nil {
		t.Fatalf("first Next() should succeed: %v", err)
	}
	if msg.Type != "system" {
		t.Errorf("got type %q, want system", msg.Type)
	}

	_, err = sr.Next()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("expected underlying error, got %v", err)
	}
}

type errReader struct {
	data string
	err  error
	done bool
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	n := copy(p, r.data)
	if n < len(r.data) {
		r.data = r.data[n:]
		return n, nil
	}
	r.done = true
	return n, nil
}
