package platform

// white-box-reason: platform internals: tests StreamMessage JSON parsing with real Claude CLI init format

import (
	"encoding/json"
	"testing"
)

func TestParseStreamMessage_InitWithPluginObjects(t *testing.T) {
	t.Parallel()

	// given: init message matching actual Claude CLI output (plugins as objects)
	initJSON := `{
		"type": "system",
		"subtype": "init",
		"session_id": "test-session",
		"model": "claude-sonnet-4-20250514",
		"tools": ["Read", "Write", "Bash"],
		"skills": ["commit", "review-pr"],
		"plugins": [
			{"name": "superpowers", "path": "/some/path/superpowers"},
			{"name": "linear", "path": "/some/path/linear"}
		],
		"mcp_servers": [
			{"name": "deepwiki", "status": "connected"},
			{"name": "linear", "status": "needs-auth"}
		]
	}`

	// when
	msg, err := ParseStreamMessage([]byte(initJSON))

	// then
	if err != nil {
		t.Fatalf("ParseStreamMessage failed: %v", err)
	}
	if msg.Type != "system" {
		t.Errorf("Type = %q, want system", msg.Type)
	}
	if msg.Subtype != "init" {
		t.Errorf("Subtype = %q, want init", msg.Subtype)
	}
	if len(msg.Tools) != 3 {
		t.Errorf("Tools count = %d, want 3", len(msg.Tools))
	}
	if len(msg.Skills) != 2 {
		t.Errorf("Skills count = %d, want 2", len(msg.Skills))
	}
	if len(msg.Plugins) != 2 {
		t.Errorf("Plugins count = %d, want 2", len(msg.Plugins))
	}
	if len(msg.Plugins) > 0 && msg.Plugins[0].Name != "superpowers" {
		t.Errorf("Plugins[0].Name = %q, want superpowers", msg.Plugins[0].Name)
	}
	if len(msg.MCPServers) != 2 {
		t.Errorf("MCPServers count = %d, want 2", len(msg.MCPServers))
	}
}

func TestParseStreamMessage_HookResponse(t *testing.T) {
	t.Parallel()

	// given
	hookJSON := `{
		"type": "system",
		"subtype": "hook_response",
		"hook_id": "h1",
		"hook_name": "pre-tool-use",
		"stdout": "hook output content"
	}`

	// when
	msg, err := ParseStreamMessage([]byte(hookJSON))

	// then
	if err != nil {
		t.Fatalf("ParseStreamMessage failed: %v", err)
	}
	if msg.Subtype != "hook_response" {
		t.Errorf("Subtype = %q, want hook_response", msg.Subtype)
	}
	if msg.Stdout != "hook output content" {
		t.Errorf("Stdout = %q, want hook output content", msg.Stdout)
	}
}

func TestParseStreamMessage_ResultMessage(t *testing.T) {
	t.Parallel()

	// given
	resultJSON := `{"type": "result", "result": "2", "is_error": false}`

	// when
	msg, err := ParseStreamMessage([]byte(resultJSON))

	// then
	if err != nil {
		t.Fatalf("ParseStreamMessage failed: %v", err)
	}
	if msg.Type != "result" {
		t.Errorf("Type = %q, want result", msg.Type)
	}
	if msg.Result != "2" {
		t.Errorf("Result = %q, want 2", msg.Result)
	}
}

func TestParseStreamMessage_InvalidJSON(t *testing.T) {
	t.Parallel()

	// given
	invalidJSON := `{not valid json}`

	// when
	_, err := ParseStreamMessage([]byte(invalidJSON))

	// then
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseStreamMessage_InitEmptyCollections(t *testing.T) {
	t.Parallel()

	// given: init with empty/missing optional fields
	initJSON := `{"type": "system", "subtype": "init", "model": "claude-sonnet-4-20250514"}`

	// when
	msg, err := ParseStreamMessage([]byte(initJSON))

	// then
	if err != nil {
		t.Fatalf("ParseStreamMessage failed: %v", err)
	}
	if len(msg.Tools) != 0 {
		t.Errorf("Tools count = %d, want 0", len(msg.Tools))
	}
	if len(msg.Plugins) != 0 {
		t.Errorf("Plugins count = %d, want 0", len(msg.Plugins))
	}
}

func TestPluginInfo_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	// given
	plugin := PluginInfo{Name: "superpowers", Path: "/some/path"}

	// when
	data, err := json.Marshal(plugin)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded PluginInfo
	err = json.Unmarshal(data, &decoded)

	// then
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.Name != "superpowers" {
		t.Errorf("Name = %q, want superpowers", decoded.Name)
	}
	if decoded.Path != "/some/path" {
		t.Errorf("Path = %q, want /some/path", decoded.Path)
	}
}
