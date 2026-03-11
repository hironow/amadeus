// fake-claude is a unified test double for the Claude Code CLI.
//
// It handles all 3 tool protocols via auto-detection:
//
//  1. amadeus  — pipes prompt to stdin, expects JSON on stdout.
//  2. sightjack — passes -p flag whose prompt contains a .json file path;
//     writes fixture JSON to that path.
//  3. paintress — passes -p flag with prompt; writes text response to stdout.
//
// Environment variables:
//
//	FAKE_CLAUDE_FIXTURE_SET  — fixture level (minimal, small, middle, hard)
//	FAKE_CLAUDE_FIXTURE_DIR  — absolute path to fixtures directory
//	FAKE_CLAUDE_PROMPT_LOG_DIR — directory to log all prompts
//	FAKE_CLAUDE_ENV_LOG_DIR  — directory to log env vars and args as JSON
//	FAKE_CLAUDE_FAIL_PATTERN — glob pattern; if prompt matches → exit 1
//	FAKE_CLAUDE_FAIL_COUNT   — fail N times then succeed (file-based counter)
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// jsonPathRe matches the first absolute .json path in the prompt text.
// Used to detect sightjack mode (the prompt embeds an output file path).
var jsonPathRe = regexp.MustCompile(`(/[^\s"]+\.json)`)

// protocol enumerates the three auto-detected modes.
type protocol int

const (
	protoAmadeus   protocol = iota // stdin pipe → JSON stdout
	protoSightjack                 // -p flag with JSON file path → write file
	protoPaintress                 // -p flag with text prompt → text stdout
)

func main() {
	// Log env vars when FAKE_CLAUDE_ENV_LOG_DIR is set.
	if envLogDir := os.Getenv("FAKE_CLAUDE_ENV_LOG_DIR"); envLogDir != "" {
		logEnv(envLogDir)
	}

	// Handle --version flag (used by doctor's CheckTool).
	if hasFlag(os.Args[1:], "--version") || hasFlag(os.Args[1:], "-v") {
		fmt.Println("fake-claude 0.0.0-test")
		return
	}

	// Handle `mcp list` subcommand (used by doctor's auth/MCP checks).
	if len(os.Args) >= 3 && os.Args[1] == "mcp" && os.Args[2] == "list" {
		fmt.Println("  linear        ✓  connected")
		return
	}

	// Handle --print flag WITHOUT -p (used by doctor's inference check).
	// When -p is also present, fall through to protocol detection (sightjack/paintress).
	if hasFlag(os.Args[1:], "--print") && extractPrompt(os.Args[1:]) == "" {
		// Find the last positional argument as the prompt
		prompt := ""
		for i := len(os.Args) - 1; i >= 1; i-- {
			if !strings.HasPrefix(os.Args[i], "-") && os.Args[i-1] != "--output-format" && os.Args[i-1] != "--max-turns" {
				prompt = os.Args[i]
				break
			}
		}
		if strings.Contains(prompt, "1+1") {
			fmt.Print("2")
		} else {
			fmt.Print("unknown")
		}
		return
	}

	proto, prompt := detectProtocol(os.Args[1:])
	outputFormat := extractOutputFormat(os.Args[1:])

	// Log prompt for observability.
	if logDir := os.Getenv("FAKE_CLAUDE_PROMPT_LOG_DIR"); logDir != "" {
		logPrompt(logDir, prompt)
	}

	// Fail-count gate: fail N times, then succeed.
	if shouldFailByCount() {
		fmt.Fprintf(os.Stderr, "fake-claude: simulated failure (fail-count)\n")
		os.Exit(1)
	}

	// Fail-pattern gate: if prompt matches glob → exit 1.
	if pat := os.Getenv("FAKE_CLAUDE_FAIL_PATTERN"); pat != "" {
		matched, _ := filepath.Match(pat, prompt)
		if matched || strings.Contains(prompt, pat) {
			fmt.Fprintf(os.Stderr, "fake-claude: simulated failure (pattern %q matched)\n", pat)
			os.Exit(1)
		}
	}

	switch proto {
	case protoAmadeus:
		handleAmadeus(prompt, outputFormat)
	case protoSightjack:
		handleSightjack(prompt, outputFormat)
	case protoPaintress:
		handlePaintress(prompt, outputFormat)
	}
}

// detectProtocol determines which tool is calling us.
//
// The key discriminator is the -p flag:
//   - amadeus does NOT pass -p; it pipes the prompt to stdin.
//   - sightjack passes -p with a prompt containing an absolute .json file path.
//   - paintress passes -p with a plain text prompt.
//
// Detection order:
//  1. If -p flag is present AND prompt contains an absolute .json path → sightjack.
//  2. If -p flag is present (no JSON path) → paintress.
//  3. If no -p flag → read stdin → amadeus.
func detectProtocol(args []string) (protocol, string) {
	// Check -p flag first (sightjack and paintress both use it).
	prompt := extractPrompt(args)
	if prompt != "" {
		// Check if the prompt contains an absolute .json file path → sightjack.
		if jsonPathRe.MatchString(prompt) {
			return protoSightjack, prompt
		}
		// -p present but no JSON path → paintress.
		return protoPaintress, prompt
	}

	// No -p flag → amadeus mode (reads from stdin).
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake-claude: read stdin: %v\n", err)
		os.Exit(1)
	}
	text := strings.TrimSpace(string(data))
	if text != "" {
		return protoAmadeus, text
	}

	// No -p and empty stdin — treat as paintress with empty prompt.
	return protoPaintress, ""
}

// hasFlag returns true if the given flag appears anywhere in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// extractPrompt finds the value of the -p flag in args.
func extractPrompt(args []string) string {
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// extractOutputFormat finds --output-format value in args.
func extractOutputFormat(args []string) string {
	for i, arg := range args {
		if arg == "--output-format" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return "text"
}

// wrapStreamJSON wraps a response body in stream-json NDJSON format.
// Matches real Claude CLI output: system init -> assistant(thinking) -> assistant(text) -> result.
// The --verbose flag (required for stream-json) enables thinking blocks in the output.
func wrapStreamJSON(body string) string {
	escaped, _ := json.Marshal(body)
	escapedStr := string(escaped) // includes surrounding quotes

	initLine := `{"type":"system","subtype":"init","session_id":"fake-session"}`
	thinkingLine := `{"type":"assistant","session_id":"fake-session","message":{"id":"msg_fake","type":"message","role":"assistant","content":[{"type":"thinking","thinking":"Analyzing the request.","signature":"fake-sig"}],"model":"claude-opus-4-6","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":10}},"parent_tool_use_id":null}`
	assistantLine := fmt.Sprintf(`{"type":"assistant","session_id":"fake-session","message":{"id":"msg_fake","type":"message","role":"assistant","content":[{"type":"text","text":%s}],"model":"claude-opus-4-6","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":50}},"parent_tool_use_id":null}`, escapedStr)
	resultLine := fmt.Sprintf(`{"type":"result","subtype":"success","session_id":"fake-session","result":%s,"is_error":false,"num_turns":1,"duration_ms":1000,"duration_api_ms":900,"total_cost_usd":0.01,"usage":{"input_tokens":100,"output_tokens":50},"stop_reason":"end_turn","uuid":"fake-uuid"}`, escapedStr)

	return initLine + "\n" + thinkingLine + "\n" + assistantLine + "\n" + resultLine + "\n"
}

// ---------------------------------------------------------------------------
// amadeus mode: keyword match on stdin → JSON to stdout
// ---------------------------------------------------------------------------

func handleAmadeus(prompt, outputFormat string) {
	content := loadFixture("am", prompt)
	if content == "" {
		content = defaultAmadeusResponse
	}
	if outputFormat == "stream-json" {
		fmt.Fprint(os.Stdout, wrapStreamJSON(content))
	} else {
		fmt.Fprint(os.Stdout, content)
	}
}

// ---------------------------------------------------------------------------
// sightjack mode: extract JSON path from prompt → write fixture file
// ---------------------------------------------------------------------------

func handleSightjack(prompt, outputFormat string) {
	outputPath := jsonPathRe.FindString(prompt)
	if outputPath == "" {
		// Should not reach here since detectProtocol already matched.
		return
	}

	content := loadFixtureByFilename("sj", filepath.Base(outputPath), prompt)
	if content == "" {
		content = defaultSightjackResponse
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "fake-claude: mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "fake-claude: write: %v\n", err)
		os.Exit(1)
	}

	// Additionally output NDJSON to stdout if stream-json.
	if outputFormat == "stream-json" {
		fmt.Fprint(os.Stdout, wrapStreamJSON(content))
	}
}

// ---------------------------------------------------------------------------
// paintress mode: keyword match on -p prompt → text to stdout
// ---------------------------------------------------------------------------

func handlePaintress(prompt, outputFormat string) {
	content := loadFixture("pt", prompt)
	if content == "" {
		content = defaultPaintressResponse
	}
	if outputFormat == "stream-json" {
		fmt.Fprint(os.Stdout, wrapStreamJSON(content))
	} else {
		fmt.Fprint(os.Stdout, content)
	}
}

// ---------------------------------------------------------------------------
// Fixture loading: reads from FAKE_CLAUDE_FIXTURE_DIR at runtime
// ---------------------------------------------------------------------------

// fixtureMapping defines keyword → fixture filename pairs per tool prefix.
// The first match wins.
var fixtureMapping = []struct {
	prefix  string // "am", "sj", "pt"
	keyword string // substring to match in prompt
	file    string // filename under FAKE_CLAUDE_FIXTURE_DIR
}{
	// amadeus
	{"am", "FULL calibration", "am_full_calibration.json"},
	{"am", "Changes Since Last Check", "am_diff_check.json"},
	{"am", "check", "am_check.json"},

	// sightjack
	{"sj", "classify", "sj_classify.json"},
	{"sj", "deep_scan", "sj_deep_scan.json"},
	{"sj", "cluster_", "sj_deep_scan.json"},
	{"sj", "wave_", "sj_wave_gen.json"},
	{"sj", "apply_", "sj_wave_apply.json"},
	{"sj", "nextgen_", "sj_nextgen.json"},
	{"sj", "architect_", "sj_architect.json"},

	// paintress
	{"pt", "expedition", "pt_expedition.txt"},
	{"pt", "FAKE_EMPTY", "pt_empty.txt"},
	{"pt", "rate limit", "pt_rate_limit.txt"},
}

// loadFixture finds a matching fixture file for the given tool prefix and prompt.
func loadFixture(prefix, prompt string) string {
	fixtureDir := os.Getenv("FAKE_CLAUDE_FIXTURE_DIR")
	if fixtureDir == "" {
		return ""
	}
	for _, m := range fixtureMapping {
		if m.prefix != prefix {
			continue
		}
		if strings.Contains(prompt, m.keyword) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, m.file))
			if err != nil {
				// Fixture file not found — fall through to default.
				continue
			}
			return string(data)
		}
	}
	return ""
}

// loadFixtureByFilename tries filepath.Match-based matching for sightjack,
// then falls back to keyword matching.
func loadFixtureByFilename(prefix, filename, prompt string) string {
	fixtureDir := os.Getenv("FAKE_CLAUDE_FIXTURE_DIR")
	if fixtureDir == "" {
		return ""
	}

	// sightjack-specific: match output filename patterns.
	sjPatterns := []struct {
		pattern string // filepath.Match pattern
		file    string // fixture file
	}{
		{"classify.json", "sj_classify.json"},
		{"cluster_*_c*.json", "sj_deep_scan.json"},
		{"wave_*_*.json", "sj_wave_gen.json"},
		{"apply_*_*.json", "sj_wave_apply.json"},
		{"nextgen_*_*.json", "sj_nextgen.json"},
		{"architect_*_*.json", "sj_architect.json"},
	}

	for _, sp := range sjPatterns {
		ok, _ := filepath.Match(sp.pattern, filename)
		if ok {
			data, err := os.ReadFile(filepath.Join(fixtureDir, sp.file))
			if err != nil {
				continue
			}
			return string(data)
		}
	}

	// Fall back to keyword matching.
	return loadFixture(prefix, prompt)
}

// ---------------------------------------------------------------------------
// Prompt logging
// ---------------------------------------------------------------------------

// logPrompt writes the prompt text to a sequentially-named file in dir.
func logPrompt(dir, prompt string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entries, _ := os.ReadDir(dir)
	seq := len(entries) + 1
	filename := fmt.Sprintf("prompt_%03d.txt", seq)
	_ = os.WriteFile(filepath.Join(dir, filename), []byte(prompt), 0o644)
}

// logEnv writes selected environment variables and args to a JSON file.
func logEnv(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entries, _ := os.ReadDir(dir)
	seq := len(entries) + 1
	filename := fmt.Sprintf("env_%03d.json", seq)

	data := map[string]any{
		"args": os.Args[1:],
	}
	// Capture env vars that tools set via ParseShellCommand.
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "CLAUDE_MODEL", "ANTHROPIC_API_KEY"} {
		if v := os.Getenv(key); v != "" {
			data[key] = v
		}
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, filename), jsonBytes, 0o644)
}

// ---------------------------------------------------------------------------
// Fail-count: file-based counter at $TMPDIR/fake-claude-call-count
// ---------------------------------------------------------------------------

// shouldFailByCount checks FAKE_CLAUDE_FAIL_COUNT and increments a
// file-based counter. Returns true if the counter is still below the
// fail threshold.
func shouldFailByCount() bool {
	envVal := os.Getenv("FAKE_CLAUDE_FAIL_COUNT")
	if envVal == "" {
		return false
	}
	failCount, err := strconv.Atoi(envVal)
	if err != nil || failCount <= 0 {
		return false
	}

	counterPath := filepath.Join(os.TempDir(), "fake-claude-call-count")

	// Read current count.
	current := 0
	data, err := os.ReadFile(counterPath)
	if err == nil {
		current, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}

	// Increment and persist.
	current++
	_ = os.WriteFile(counterPath, []byte(strconv.Itoa(current)), 0o644)

	// Fail while count <= failCount.
	return current <= failCount
}

// ---------------------------------------------------------------------------
// Default responses (used when no fixture file matches)
// ---------------------------------------------------------------------------

var defaultAmadeusResponse = strings.TrimSpace(`
{
  "axes": {
    "adr_integrity": {"score": 0, "details": "Clean"},
    "dod_fulfillment": {"score": 0, "details": "All DoDs met"},
    "dependency_integrity": {"score": 0, "details": "Clean"},
    "implicit_constraints": {"score": 0, "details": "No issues"}
  },
  "dmails": [],
  "reasoning": "No significant divergence detected."
}
`)

var defaultSightjackResponse = strings.TrimSpace(`
{
  "clusters": [],
  "total_issues": 0
}
`)

var defaultPaintressResponse = `I'll analyze the issues and work on them systematically.

## Issue Analysis

Looking at the assigned issues, I'll start with the highest priority item.

### Changes Made

1. Updated the configuration file
2. Fixed the validation logic
3. Added missing test coverage

All changes have been committed and pushed.

__EXPEDITION_REPORT__
issue_id: TEST-001
issue_title: Test action
mission_type: add_dod
branch: feat/test-001
pr_url: none
status: success
reason: completed
remaining_issues: 0
bugs_found: 0
bug_issues: none
insight: All tasks completed successfully
failure_type: none
__EXPEDITION_END__
`
