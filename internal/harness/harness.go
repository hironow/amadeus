// Package harness mediates between the LLM and the task environment.
// It is the single import surface for all decision, validation, and
// specification logic. Internal sub-packages (policy, verifier, filter)
// represent the LLM-dependence spectrum but are not imported directly
// by callers.
package harness

import (
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness/filter"
	"github.com/hironow/amadeus/internal/harness/policy"
	"github.com/hironow/amadeus/internal/harness/verifier"
)

// --- policy layer (deterministic decisions, no LLM) ---

// EvaluateMergeReadiness evaluates whether a PR is ready to merge.
var EvaluateMergeReadiness = policy.EvaluateMergeReadiness

// DetermineMergeMethod returns the merge strategy for a PR based on chain position.
var DetermineMergeMethod = policy.DetermineMergeMethod

// BuildPRConvergenceReport builds a convergence report from open PRs.
var BuildPRConvergenceReport = policy.BuildPRConvergenceReport

// BuildConvergenceDMail constructs a valid DMail from a PRConvergenceReport.
var BuildConvergenceDMail = policy.BuildConvergenceDMail

// IsPipelinePR checks if a PR was created by the 4-tool pipeline.
var IsPipelinePR = policy.IsPipelinePR

// IsPipelinePRWithIssueContext extends IsPipelinePR with issue-link checking.
var IsPipelinePRWithIssueContext = policy.IsPipelinePRWithIssueContext

// --- verifier layer (validation rules, no LLM) ---

// ValidateDMail validates a D-Mail against schema v1 rules.
var ValidateDMail = verifier.ValidateDMail

// ClassifyProviderError classifies a provider error from stderr output.
func ClassifyProviderError(provider domain.Provider, stderr string) domain.ProviderErrorInfo {
	return verifier.ClassifyProviderError(provider, stderr)
}

// --- filter layer (LLM action spaces: prompts, response schemas) ---

// PromptRegistry is a type alias for filter.PromptRegistry.
type PromptRegistry = filter.PromptRegistry

// DefaultPromptRegistry returns the process-wide PromptRegistry.
var DefaultPromptRegistry = filter.Default

// MustDefaultPromptRegistry returns the singleton or panics. Safe with embed.FS.
var MustDefaultPromptRegistry = filter.MustDefault

// --- filter layer: optimization (Phase 3) ---

type PromptOptimizer = filter.PromptOptimizer
type EvalCase = filter.EvalCase
type OptimizedResult = filter.OptimizedResult

var SavePrompt = filter.Save
var PromptsDir = filter.PromptsDir
