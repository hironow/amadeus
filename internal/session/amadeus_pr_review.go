package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// PRReviewLabel is the label applied to PRs after amadeus review.
// Replaces the old "amadeus:reviewed-{sha8}" pattern to prevent unbounded label accumulation.
const PRReviewLabel = "amadeus:reviewed"

// PRReviewLabelLegacyPrefix is the old label prefix kept for backward compatibility
// during migration. Labels with this prefix are cleaned up on encounter.
const PRReviewLabelLegacyPrefix = "amadeus:reviewed-"

// evaluatePRDiffs fetches open PRs targeting integrationBranch, evaluates each
// PR's diff against ADRs/DoDs using Claude, generates feedback D-Mails, and
// applies a commit-aware review label.
//
// Skip conditions:
//   - PRReader is nil (PR reading disabled)
//   - PR already has label `amadeus:reviewed-{current_head_sha8}`
//
// Label is applied ONLY after all D-Mails are successfully emitted.
// Each PR is evaluated independently: one PR failure does not block others.
func (a *Amadeus) evaluatePRDiffs(ctx context.Context, integrationBranch string) ([]domain.DMail, error) {
	if a.PRReader == nil {
		return nil, nil
	}

	prs, err := a.PRReader.ListOpenPRs(ctx, integrationBranch)
	if err != nil {
		return nil, fmt.Errorf("list open PRs for diff review: %w", err)
	}

	var allDMails []domain.DMail
	for _, pr := range prs {
		if pr.HasLabel(PRReviewLabel) {
			a.Logger.Info("PR %s: already reviewed at %s, skipping", pr.Number(), pr.HeadSHAShort())
			continue
		}

		dmails, evalErr := a.evaluateSinglePR(ctx, pr)
		if evalErr != nil {
			a.Logger.Warn("PR %s evaluation error: %v", pr.Number(), evalErr)
			continue // best-effort: skip this PR, continue with others
		}
		allDMails = append(allDMails, dmails...)

		if a.PRWriter != nil {
			// Clean up legacy "amadeus:reviewed-{sha8}" labels
			for _, label := range pr.Labels() {
				if strings.HasPrefix(label, PRReviewLabelLegacyPrefix) {
					if rmErr := a.PRWriter.RemoveLabel(ctx, pr.Number(), label); rmErr != nil {
						a.Logger.Warn("PR %s: remove legacy label %s: %v", pr.Number(), label, rmErr)
					}
				}
			}

			// Apply single review label after successful D-Mail emission.
			if labelErr := a.PRWriter.ApplyLabel(ctx, pr.Number(), PRReviewLabel); labelErr != nil {
				a.Logger.Warn("PR %s: failed to apply label: %v", pr.Number(), labelErr)
			} else {
				a.Logger.Info("PR %s: labeled %s", pr.Number(), PRReviewLabel)
			}
		}
	}

	return allDMails, nil
}

// evaluateSinglePR evaluates a single PR's diff against ADRs/DoDs using Claude.
// Returns error if D-Mail emission fails (prevents label from being applied).
func (a *Amadeus) evaluateSinglePR(ctx context.Context, pr domain.PRState) ([]domain.DMail, error) {
	diff, err := a.PRReader.GetPRDiff(ctx, pr.Number())
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}

	if diff == "" {
		a.Logger.Info("PR %s: empty diff, skipping evaluation", pr.Number())
		return nil, nil
	}

	prompt := a.buildPRReviewPrompt(pr, diff)

	if sharedCircuitBreaker != nil {
		if cbErr := sharedCircuitBreaker.Allow(ctx); cbErr != nil {
			return nil, fmt.Errorf("PR %s circuit breaker: %w", pr.Number(), cbErr)
		}
	}
	rawResp, err := a.claudeRunner().Run(ctx, prompt, nil, port.WithAllowedTools(DivergenceMeterAllowedTools...))
	recordCircuitBreaker(domain.ProviderClaudeCode, err, "")
	if err != nil {
		return nil, fmt.Errorf("claude evaluation: %w", err)
	}

	claudeResp, err := domain.ParseClaudeResponse([]byte(rawResp))
	if err != nil {
		return nil, fmt.Errorf("parse evaluation response: %w", err)
	}

	// Generate D-Mails from evaluation results
	now := time.Now().UTC()
	var dmails []domain.DMail
	for i, candidate := range claudeResp.DMails {
		kind := domain.KindDesignFeedback
		if candidate.Category == "implementation" {
			kind = domain.KindImplFeedback
		}

		severity := domain.SeverityMedium
		action := domain.ActionRetry
		switch domain.DMailAction(candidate.Action) {
		case domain.ActionRetry:
			action = domain.ActionRetry
			severity = domain.SeverityMedium
		case domain.ActionEscalate:
			action = domain.ActionEscalate
			severity = domain.SeverityHigh
		case domain.ActionResolve:
			action = domain.ActionResolve
			severity = domain.SeverityLow
		}

		// Unique name per candidate: include index to prevent D-Mail name collisions
		// when Claude produces multiple findings for the same PR.
		dmail := domain.DMail{
			SchemaVersion: domain.DMailSchemaVersion,
			Name:          fmt.Sprintf("am-pr-review-%s-%s-%d", pr.Number(), pr.HeadSHAShort(), i),
			Kind:          kind,
			Description:   candidate.Description,
			Severity:      severity,
			Targets:       candidate.Targets,
			Action:        action,
			Body:          candidate.Detail,
			Metadata: map[string]string{
				"pr_number":   pr.Number(),
				"pr_title":    pr.Title(),
				"head_sha":    pr.HeadSHA(),
				"review_type": "pr_diff",
			},
		}

		// Validate before emitting — reject protocol-violating D-Mails
		if errs := harness.ValidateDMail(dmail); len(errs) > 0 {
			a.Logger.Warn("PR %s: invalid D-Mail %s: %v", pr.Number(), dmail.Name, errs)
			continue
		}

		if a.Emitter != nil {
			if emitErr := a.Emitter.EmitDMailGenerated(dmail, now); emitErr != nil {
				// Emit failure is fatal for this PR — prevents label from being applied
				return nil, fmt.Errorf("emit D-Mail for PR %s: %w", pr.Number(), emitErr)
			}
		}

		dmails = append(dmails, dmail)
	}

	if len(dmails) > 0 {
		a.Logger.OK("PR %s: %d feedback D-Mail(s) generated", pr.Number(), len(dmails))
	} else {
		a.Logger.Info("PR %s: no issues found", pr.Number())
	}

	return dmails, nil
}

// buildPRReviewPrompt constructs the evaluation prompt for a single PR.
func (a *Amadeus) buildPRReviewPrompt(pr domain.PRState, diff string) string {
	return fmt.Sprintf(`You are amadeus, a post-merge integrity harness. You are evaluating a pull request diff against the project's Architecture Decision Records (ADRs) and Definitions of Done (DoDs).

## PR Information
- Number: %s
- Title: %s
- Base Branch: %s
- Head Branch: %s

## PR Diff
%s

## Instructions
1. Read all ADR files in docs/adr/ and DoD files if they exist
2. Evaluate whether this PR's changes comply with established ADRs and DoDs
3. Identify any violations, deviations, or areas of concern
4. Score the overall divergence (0 = fully compliant, 100 = completely divergent)

## Response Format (JSON)
{
  "files_read": ["docs/adr/...", ...],
  "axes": {
    "structural": {"score": 0, "details": "..."},
    "behavioral": {"score": 0, "details": "..."},
    "convention": {"score": 0, "details": "..."},
    "dependency": {"score": 0, "details": "..."}
  },
  "dmails": [
    {
      "description": "Brief description of the issue",
      "detail": "Detailed explanation",
      "targets": ["file.go"],
      "action": "retry|escalate|resolve",
      "category": "design|implementation"
    }
  ],
  "reasoning": "Overall assessment in %s"
}

Only report genuine ADR/DoD violations. Do not flag stylistic preferences or minor formatting issues.`, pr.Number(), pr.Title(), pr.BaseBranch(), pr.HeadBranch(), diff, a.Config.Lang)
}
