package policy

import (
	"regexp"
	"sort"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// pipelineLabels are labels applied by the 4-tool pipeline.
var pipelineLabels = []string{
	"paintress:pr-open",
	"sightjack:ready",
}

// wavePattern matches sightjack wave branch segments like "-w1-", "-w3-1-", "-w2-12-".
var wavePattern = regexp.MustCompile(`-w\d+-`)

// amadeusFixPrefixes are branch name segments from amadeus-generated fix PRs.
var amadeusFixPrefixes = []string{
	"am-pr-review-",
	"am-implementation-feedback-",
	"am-conflict-",
}

// githubIssuePattern matches GitHub issue references like #1, #21, #123.
var githubIssuePattern = regexp.MustCompile(`#(\d+)`)

// IsPipelinePR checks if a PR was created by the 4-tool pipeline by examining
// labels and branch naming patterns. This is a pure function; issue-link checks
// requiring I/O are handled at the session layer.
//
// Detection methods (any match → true):
//  1. Label: paintress:pr-open or sightjack:ready
//  2. Branch pattern (head or base): wave (-wN-), expedition/, or amadeus fix prefix
func IsPipelinePR(pr domain.PRState) bool {
	// 1. Label check
	for _, label := range pipelineLabels {
		if pr.HasLabel(label) {
			return true
		}
	}

	// 2. Branch pattern check (head and base)
	for _, branch := range []string{pr.HeadBranch(), pr.BaseBranch()} {
		if wavePattern.MatchString(branch) {
			return true
		}
		if strings.HasPrefix(branch, "expedition/") {
			return true
		}
		for _, prefix := range amadeusFixPrefixes {
			if strings.Contains(branch, prefix) {
				return true
			}
		}
	}

	return false
}

// IsPipelinePRWithIssueContext extends IsPipelinePR by also checking whether
// the PR title references any issue from the sightjack pipeline.
// sightjackIssueNumbers is a pre-fetched list of issue numbers (as strings,
// without "#") that have the sightjack:ready label.
func IsPipelinePRWithIssueContext(pr domain.PRState, sightjackIssueNumbers []string) bool {
	if IsPipelinePR(pr) {
		return true
	}
	if len(sightjackIssueNumbers) == 0 {
		return false
	}
	issueSet := make(map[string]bool, len(sightjackIssueNumbers))
	for _, n := range sightjackIssueNumbers {
		issueSet[n] = true
	}
	for _, num := range ExtractGitHubIssueNumbers(pr.Title()) {
		if issueSet[num] {
			return true
		}
	}
	return false
}

// ExtractGitHubIssueNumbers extracts GitHub issue numbers from text (e.g. "#1", "#21").
// Returns sorted, deduplicated issue numbers as strings (without the # prefix).
// Returns nil if no issue references are found.
func ExtractGitHubIssueNumbers(text string) []string {
	matches := githubIssuePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m[1]] = true
	}

	nums := make([]string, 0, len(seen))
	for n := range seen {
		nums = append(nums, n)
	}
	sort.Strings(nums)
	return nums
}
