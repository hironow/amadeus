// Package harness mediates between the LLM and the task environment.
// It is the single import surface for all decision, validation, and
// specification logic. Internal sub-packages (policy, verifier, filter)
// represent the LLM-dependence spectrum but are not imported directly
// by callers.
//
// See: AutoHarness (arxiv 2603.03329v1) — "Harness as Policy" spectrum.
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

// FilterMergeReady returns only the PRs that are ready to merge.
var FilterMergeReady = policy.FilterMergeReady

// DetermineMergeMethod returns the merge strategy for a PR based on chain position.
var DetermineMergeMethod = policy.DetermineMergeMethod

// BuildPRConvergenceReport builds a convergence report from open PRs.
var BuildPRConvergenceReport = policy.BuildPRConvergenceReport

// ClassifyConvergenceScenario classifies a chain's convergence scenario.
var ClassifyConvergenceScenario = policy.ClassifyConvergenceScenario

// BuildConvergenceDMail constructs a valid DMail from a PRConvergenceReport.
var BuildConvergenceDMail = policy.BuildConvergenceDMail

// BuildConvergenceDMailBody produces a Markdown body from a PRConvergenceReport.
var BuildConvergenceDMailBody = policy.BuildConvergenceDMailBody

// IsPipelinePR checks if a PR was created by the 4-tool pipeline.
var IsPipelinePR = policy.IsPipelinePR

// IsPipelinePRWithIssueContext extends IsPipelinePR with issue-link checking.
var IsPipelinePRWithIssueContext = policy.IsPipelinePRWithIssueContext

// ExtractGitHubIssueNumbers extracts GitHub issue numbers from text.
var ExtractGitHubIssueNumbers = policy.ExtractGitHubIssueNumbers

// --- verifier layer (validation rules, no LLM) ---

// ValidateDMail validates a D-Mail against schema v1 rules.
var ValidateDMail = verifier.ValidateDMail

// ClassifyProviderError classifies a provider error from stderr output.
func ClassifyProviderError(provider domain.Provider, stderr string) domain.ProviderErrorInfo {
	return verifier.ClassifyProviderError(provider, stderr)
}

// --- filter layer (LLM action spaces: prompts, response schemas) ---

// PromptRegistry is the type alias for the filter.Registry.
type PromptRegistry = filter.Registry

// PromptConfig is the type alias for a single prompt configuration.
type PromptConfig = filter.PromptConfig

// NewPromptRegistry creates a new prompt registry from embedded YAML files.
var NewPromptRegistry = filter.NewRegistry

// DefaultPromptRegistry returns the process-wide singleton PromptRegistry.
var DefaultPromptRegistry = filter.DefaultRegistry

// MustDefaultPromptRegistry returns the singleton or panics. Safe with embed.FS.
var MustDefaultPromptRegistry = filter.MustDefaultRegistry

// ExpandPromptTemplate performs {key} substitution on a template string.
var ExpandPromptTemplate = filter.ExpandTemplate
