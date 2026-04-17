package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrIncompleteRead is returned when Claude's evaluation response is missing
// expected file kinds in the files_read field. This indicates Claude did not
// read all required eval files before producing its evaluation.
var ErrIncompleteRead = errors.New("claude: evaluation based on incomplete file reads")

// EvalFileKind identifies a type of eval file written for Claude to read.
type EvalFileKind string

const (
	EvalKindADRs              EvalFileKind = "adrs"
	EvalKindDoDs              EvalFileKind = "dods"
	EvalKindDiff              EvalFileKind = "diff"
	EvalKindPreviousScores    EvalFileKind = "previous_scores"
	EvalKindPRReviews         EvalFileKind = "pr_reviews"
	EvalKindCodebaseStructure EvalFileKind = "codebase_structure"
	EvalKindDependencyMap     EvalFileKind = "dependency_map"
)

// FormatEvalFile produces a markdown string with YAML front matter for an eval file.
// The front matter includes kind, generated_by, generated_at, read_only, and warning fields.
func FormatEvalFile(kind EvalFileKind, content string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`---
kind: %s
generated_by: amadeus
generated_at: "%s"
read_only: true
warning: "DO NOT modify this file. It is auto-generated for evaluation only."
---
%s`, string(kind), now, content)
}

// ValidateFilesRead checks that all expected kinds were reported as read by Claude.
// Returns ErrIncompleteRead if any expected kind is missing from got.
// Extra kinds in got (superset) are allowed.
func ValidateFilesRead(got []string, expected []string) error { // nosemgrep: parse-dont-validate.validate-returns-error-only-go — performs set membership check on two []string slices; no single parsed domain type to return [permanent]
	gotSet := make(map[string]bool, len(got))
	for _, k := range got {
		gotSet[k] = true
	}
	var missing []string
	for _, k := range expected {
		if !gotSet[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %v", ErrIncompleteRead, missing)
	}
	return nil
}
