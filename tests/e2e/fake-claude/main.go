// fake-claude is a test double for the Claude Code CLI.
//
// It mimics the subset of Claude CLI behaviour that amadeus relies on:
//   - Reads a prompt from stdin (amadeus pipes prompt text via cmd.Stdin).
//   - Writes a canned ClaudeResponse JSON to stdout.
//   - Accepts (and ignores) the flags amadeus passes: --model, --output-format, --print, etc.
//   - Produces no stderr output on success.
//
// The response is selected by matching keywords in the prompt text.
// If FAKE_CLAUDE_PROMPT_LOG_DIR is set, each prompt is logged for inspection.
//
// Install as /usr/local/bin/claude inside an E2E Docker container so that
// exec.CommandContext(ctx, "claude", ...) resolves to this binary.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	prompt, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake-claude: read stdin: %v\n", err)
		os.Exit(1)
	}

	text := string(prompt)

	// Log prompt when FAKE_CLAUDE_PROMPT_LOG_DIR is set.
	if logDir := os.Getenv("FAKE_CLAUDE_PROMPT_LOG_DIR"); logDir != "" {
		logPrompt(logDir, text)
	}

	// Match prompt content to a fixture.
	for _, f := range fixtures {
		if f.match(text) {
			fmt.Fprint(os.Stdout, f.content)
			return
		}
	}

	// Default: return a clean response with no D-Mails.
	fmt.Fprint(os.Stdout, defaultCleanResponse)
}

// logPrompt appends the prompt text to a sequentially-named file in dir.
func logPrompt(dir, prompt string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entries, _ := os.ReadDir(dir)
	seq := len(entries) + 1
	filename := fmt.Sprintf("prompt_%03d.txt", seq)
	_ = os.WriteFile(filepath.Join(dir, filename), []byte(prompt), 0o644)
}

// fixture maps a prompt keyword match to a canned JSON response.
type fixture struct {
	keyword string
	match   func(prompt string) bool
	content string
}

var fixtures = []fixture{
	{
		keyword: "FULL calibration",
		match:   func(p string) bool { return strings.Contains(p, "FULL calibration") },
		content: fullCalibrationResponse,
	},
	{
		keyword: "diff check",
		match:   func(p string) bool { return strings.Contains(p, "Changes Since Last Check") },
		content: diffCheckResponse,
	},
}

// --- Canned JSON responses matching amadeus ClaudeResponse schema ---

var defaultCleanResponse = strings.TrimSpace(`
{
  "axes": {
    "adr_integrity": {"score": 5, "details": "Minor naming drift"},
    "dod_fulfillment": {"score": 0, "details": "All DoDs met"},
    "dependency_integrity": {"score": 0, "details": "Clean"},
    "implicit_constraints": {"score": 0, "details": "No issues"}
  },
  "dmails": [],
  "reasoning": "Codebase is in good shape. No significant divergence detected."
}
`)

var fullCalibrationResponse = strings.TrimSpace(`
{
  "axes": {
    "adr_integrity": {"score": 15, "details": "ADR-003 minor tension with auth module"},
    "dod_fulfillment": {"score": 20, "details": "Issue #42 edge case not covered"},
    "dependency_integrity": {"score": 10, "details": "auth -> cart dependency detected"},
    "implicit_constraints": {"score": 5, "details": "Naming convention drift in utils/"}
  },
  "dmails": [
    {
      "description": "ADR-003 needs review for auth module changes",
      "detail": "The auth module implementation has drifted from ADR-003 guidelines.",
      "issues": ["MY-100"],
      "targets": ["auth/session.go"]
    }
  ],
  "reasoning": "Full calibration detected moderate divergence. ADR-003 tension is the primary concern.",
  "impact_radius": [
    {"area": "auth/session.go", "impact": "direct", "detail": "Session validation changed"},
    {"area": "api/middleware.go", "impact": "indirect", "detail": "Uses auth session"}
  ]
}
`)

var diffCheckResponse = strings.TrimSpace(`
{
  "axes": {
    "adr_integrity": {"score": 10, "details": "Minor ADR tension"},
    "dod_fulfillment": {"score": 5, "details": "DoD partially met"},
    "dependency_integrity": {"score": 0, "details": "No new violations"},
    "implicit_constraints": {"score": 0, "details": "Clean"}
  },
  "dmails": [],
  "reasoning": "Diff check shows minor tensions but no D-Mails warranted."
}
`)
