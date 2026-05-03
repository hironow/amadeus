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

// CorrectionDecision is the deterministic routing decision for a corrective D-Mail.
type CorrectionDecision = policy.CorrectionDecision

// DetermineCorrectionDecision resolves corrective routing/action policy.
var DetermineCorrectionDecision = policy.DetermineCorrectionDecision

// CorrectiveTargetAgentForFailure resolves the owner for a corrective failure.
var CorrectiveTargetAgentForFailure = policy.CorrectiveTargetAgentForFailure

// --- Rival Contract v1 facade (Phase 3: amadeus contract-aware drift) ---

// RivalContract is the parsed Rival Contract v1 body re-exported for
// session-layer adapters that need the canonical six-section shape.
type RivalContract = policy.RivalContract

// RivalContractMetadata is the parsed Rival Contract v1 metadata view.
type RivalContractMetadata = policy.RivalContractMetadata

// Rival Contract v1.1 DomainStyle enum re-exports. Session adapters use
// these via the harness facade so the layer rule (session must not
// import harness/policy directly) stays satisfied.
const (
	DomainStyleEventSourced = policy.DomainStyleEventSourced
	DomainStyleGeneric      = policy.DomainStyleGeneric
	DomainStyleMixed        = policy.DomainStyleMixed
)

// CurrentContract pairs a parsed Rival Contract v1 body with its metadata
// and originating D-Mail name. amadeus uses this as the projection
// result of ProjectCurrentContracts.
type CurrentContract = policy.CurrentContract

// ContractConflict is emitted when two D-Mails claim the same contract
// id at the same revision but disagree on body/supersedes lineage.
type ContractConflict = policy.ContractConflict

// EvidenceItem is a single deterministic Evidence bullet.
type EvidenceItem = policy.EvidenceItem

// ProjectCurrentContracts is the deterministic Rival Contract v1
// projection that selects the winning revision per contract_id.
var ProjectCurrentContracts = policy.ProjectCurrentContracts

// ParseRivalContractBody parses a Rival Contract v1 markdown body.
var ParseRivalContractBody = policy.ParseRivalContractBody

// ParseEvidenceItems parses Evidence bullets into deterministic items.
var ParseEvidenceItems = policy.ParseEvidenceItems

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
