package session

import (
	"fmt"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
)

// buildDiffCheckPrompt renders the file-reference diff_check prompt for the given language
// using the harness PromptRegistry.
func buildDiffCheckPrompt(lang string, params domain.DiffCheckParams) (string, error) {
	promptName := fmt.Sprintf("fileref_diff_check_%s", lang)
	reg := harness.MustDefaultPromptRegistry()
	if _, getErr := reg.Get(promptName); getErr != nil {
		return "", fmt.Errorf("unsupported language %q: %w", lang, getErr)
	}

	vars := map[string]string{
		"eval_dir":                    params.EvalDir,
		"pr_reviews_section":          renderPRReviewsSection(lang, params),
		"linked_issues_section":       renderLinkedIssuesSection(lang, params),
		"repeated_violations_section": renderRepeatedViolationsSection(lang, params),
		"divergence_trend_section":    renderDivergenceTrendSection(lang, params),
	}

	return reg.Expand(promptName, vars)
}

// buildFullCheckPrompt renders the file-reference full_check prompt for the given language
// using the harness PromptRegistry.
func buildFullCheckPrompt(lang string, params domain.FullCheckParams) (string, error) {
	promptName := fmt.Sprintf("fileref_full_check_%s", lang)
	reg := harness.MustDefaultPromptRegistry()
	if _, getErr := reg.Get(promptName); getErr != nil {
		return "", fmt.Errorf("unsupported language %q: %w", lang, getErr)
	}

	vars := map[string]string{
		"eval_dir": params.EvalDir,
	}

	return reg.Expand(promptName, vars)
}

// renderPRReviewsSection renders the optional PR reviews file reference line.
func renderPRReviewsSection(_ string, params domain.DiffCheckParams) string {
	if !params.HasPRReviews {
		return ""
	}
	return fmt.Sprintf("5. %s/pr_reviews.md\n", params.EvalDir)
}

// renderLinkedIssuesSection renders the optional linked Linear issues block.
func renderLinkedIssuesSection(lang string, params domain.DiffCheckParams) string {
	if params.LinkedIssueIDs == "" {
		return ""
	}
	if lang == "ja" {
		return fmt.Sprintf(`
## リンクされたLinear Issue
マージされたコミットに以下のLinear Issue IDが見つかりました: %s

**Action required**: Linear MCPツール (`+"`get_issue`"+`) を使用して各issueのタイトル、説明、受入基準を取得してください。これを `+"`dod_fulfillment`"+` 評価のDefinition of Done (DoD) コンテキストとして使用してください。

**重要**: issueのステータスフィールド（例: "Done", "In Progress"）を信用しないでください。issueの説明と受入基準のみを真のソースとして使用してください。
`, params.LinkedIssueIDs)
	}
	return fmt.Sprintf(`
## Linked Linear Issues
The following Linear Issue IDs were found in the merged commits: %s

**Action required**: Use the Linear MCP tool (`+"`get_issue`"+`) to fetch each issue's title, description, and acceptance criteria. Use this as the Definition of Done (DoD) context for evaluating `+"`dod_fulfillment`"+`.

**Important**: Do NOT trust the issue's status field (e.g., "Done", "In Progress"). Focus exclusively on the issue description and acceptance criteria as the source of truth.
`, params.LinkedIssueIDs)
}

// renderRepeatedViolationsSection renders the optional repeated violations warning block.
func renderRepeatedViolationsSection(lang string, params domain.DiffCheckParams) string {
	if len(params.RepeatedViolations) == 0 {
		return ""
	}
	var sb strings.Builder
	if lang == "ja" {
		sb.WriteString("\n## 繰り返し違反の警告\n")
		sb.WriteString("以下の整合性軸が直近の複数チェックで継続的に違反しきい値を超えています:\n")
		for _, v := range params.RepeatedViolations {
			sb.WriteString(fmt.Sprintf("- **%s** (%d回連続違反): %s\n", v.Axis, v.Count, v.Description))
		}
		sb.WriteString("\nこれらの軸に**特に注意**してください。同じ問題が継続している場合、D-Mailの深刻度をエスカレートしてください。\n")
	} else {
		sb.WriteString("\n## Repeated Violations Warning\n")
		sb.WriteString("The following integrity axes have been consistently above the violation threshold across recent checks:\n")
		for _, v := range params.RepeatedViolations {
			sb.WriteString(fmt.Sprintf("- **%s** (%d consecutive violations): %s\n", v.Axis, v.Count, v.Description))
		}
		sb.WriteString("\n**Pay special attention** to these axes. If the same issues persist, escalate the severity in your D-Mail output.\n")
	}
	return sb.String()
}

// renderDivergenceTrendSection renders the optional divergence trend block.
func renderDivergenceTrendSection(lang string, params domain.DiffCheckParams) string {
	if params.DivergenceTrend == nil {
		return ""
	}
	if lang == "ja" {
		return fmt.Sprintf("\n## 乖離トレンド\n現在のトレンド: **%s** (デルタ: %.1f)\n\n%s\n",
			params.DivergenceTrend.Class, params.DivergenceTrend.Delta, params.DivergenceTrend.Message)
	}
	return fmt.Sprintf("\n## Divergence Trend\nCurrent trend: **%s** (delta: %.1f)\n\n%s\n",
		params.DivergenceTrend.Class, params.DivergenceTrend.Delta, params.DivergenceTrend.Message)
}
