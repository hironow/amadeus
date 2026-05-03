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
		"eval_dir":                       params.EvalDir,
		"pr_reviews_section":             renderPRReviewsSection(lang, params),
		"linked_issues_section":          renderLinkedIssuesSection(lang, params),
		"repeated_violations_section":    renderRepeatedViolationsSection(lang, params),
		"divergence_trend_section":       renderDivergenceTrendSection(lang, params),
		"rival_contract_section":         renderRivalContractSection(lang, params.CurrentContract),
		"event_sourced_glossary_section": renderEventSourcedGlossarySection(lang, params.CurrentContract),
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
		"eval_dir":                       params.EvalDir,
		"rival_contract_section":         renderRivalContractSection(lang, params.CurrentContract),
		"event_sourced_glossary_section": renderEventSourcedGlossarySection(lang, params.CurrentContract),
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

// renderRivalContractSection renders the optional Rival Contract v1 prompt
// section. It returns the empty string when no current contract is
// available (legacy archive, conflict, or empty content) so the prompt
// degrades gracefully — existing scoring behavior is unchanged when
// Rival Contract v1 specifications do not yet exist in the archive.
//
// The rendered block summarises the four contract-aware sections that
// are useful as scoring context (Intent / Decisions / Boundaries /
// Evidence). Body text is truncated per section so the overall prompt
// stays under the size budget enforced by existing diff/full-check
// tests; long contracts surface a "(truncated)" marker so the model
// knows to consult the archive directly.
func renderRivalContractSection(lang string, current *domain.RivalContractContext) string {
	if current == nil || !current.HasContent() {
		return ""
	}

	var sb strings.Builder
	if lang == "ja" {
		sb.WriteString("\n## Rival Contract (current specification)\n")
		fmt.Fprintf(&sb, "- 契約ID: `%s`", current.ContractID)
		if current.Revision > 0 {
			fmt.Fprintf(&sb, " (revision %d)", current.Revision)
		}
		sb.WriteString("\n")
		if current.Title != "" {
			fmt.Fprintf(&sb, "- タイトル: %s\n", current.Title)
		}
		writeContractField(&sb, "Intent", current.Intent)
		writeContractField(&sb, "Decisions", current.Decisions)
		writeContractField(&sb, "Boundaries", current.Boundaries)
		writeContractField(&sb, "Evidence", current.Evidence)
		sb.WriteString("\n契約の Boundaries と Evidence は暗黙のスタイルより優先して逸脱判定に使ってください。\n")
		return sb.String()
	}

	sb.WriteString("\n## Rival Contract (current specification)\n")
	fmt.Fprintf(&sb, "- Contract: `%s`", current.ContractID)
	if current.Revision > 0 {
		fmt.Fprintf(&sb, " (revision %d)", current.Revision)
	}
	sb.WriteString("\n")
	if current.Title != "" {
		fmt.Fprintf(&sb, "- Title: %s\n", current.Title)
	}
	writeContractField(&sb, "Intent", current.Intent)
	writeContractField(&sb, "Decisions", current.Decisions)
	writeContractField(&sb, "Boundaries", current.Boundaries)
	writeContractField(&sb, "Evidence", current.Evidence)
	sb.WriteString("\nTreat Boundaries and Evidence items as stronger than implicit codebase style when they conflict.\n")
	return sb.String()
}

// renderEventSourcedGlossarySection emits a canonical command/event/
// read-model glossary preamble when the current Rival Contract carries
// `metadata.domain_style == "event-sourced"` (Rival Contract v1.1).
//
// The glossary normalizes vocabulary the model must use when reasoning
// about an event-sourced contract Domain section so divergence scoring
// stays grounded in the canonical terms (Command, Event, Aggregate,
// Read Model, Policy). For every other domain_style value (including
// the legacy v1 default `""` and the explicit `generic` / `mixed`
// values), this returns the empty string so the rendered prompt is
// byte-identical to the legacy v1 output.
//
// Why a fixed marker string ("event-sourcing glossary"): Phase 1.1A
// only locks the *branching* behavior. The full glossary copy is a
// Phase 3 deliverable; cross-tool consumers depend on the marker, not
// the wording.
func renderEventSourcedGlossarySection(lang string, current *domain.RivalContractContext) string {
	if current == nil || !current.HasContent() {
		return ""
	}
	if current.DomainStyle != harness.DomainStyleEventSourced {
		return ""
	}

	if lang == "ja" {
		return "\n## イベントソーシング用語集 (event-sourcing glossary)\n" +
			"この契約の Domain セクションはイベントソーシング語彙で記述されています。" +
			"乖離を採点する際は以下の語彙の整合性を厳しく評価してください:\n" +
			"- **Command** (コマンド): 集約に対する未来形の入力 (例: AuthorizePurchase will -)。決して過去形にしないこと。\n" +
			"- **Event** (イベント): 集約の状態変化を表す過去形の事実 (例: PurchaseAuthorized did -)。決して未来形にしないこと。\n" +
			"- **Aggregate** (集約 / ドメインモデル): イベントを再生して整合性境界を保つ単位。コマンドの宛先。\n" +
			"- **Read Model** (リードモデル / 帳票): イベントから派生したクエリ用ビュー。コマンドの発行元の根拠となる。\n" +
			"- **Policy** (ポリシー): 「WHEN [EVENT] THEN [COMMAND]」を表現する自動連鎖。\n" +
			"\n境界違反の例: Command が過去形で書かれている、集約の状態を直接変更する I/O、Read Model が集約の可変状態に結合している、ポリシーが副作用を直接行うなど。これらは Boundaries や Decisions と衝突する深刻な乖離です。\n"
	}
	return "\n## Event-Sourcing Glossary (event-sourcing glossary)\n" +
		"This contract's Domain section is written in event-sourcing vocabulary." +
		" When scoring divergence, evaluate adherence to the following terms strictly:\n" +
		"- **Command**: a future-tense input to an aggregate (e.g. AuthorizePurchase will charge -). Never past tense.\n" +
		"- **Event**: a past-tense fact recording an aggregate state change (e.g. PurchaseAuthorized did record -). Never future tense.\n" +
		"- **Aggregate** (a.k.a. domain model): consistency boundary that replays events; the receiver of commands.\n" +
		"- **Read Model**: query-side view derived from events; the basis on which commands are issued.\n" +
		"- **Policy**: an automatic chain expressed as \"WHEN [EVENT] THEN [COMMAND]\".\n" +
		"\nBoundary-violation signals: a Command written in past tense, direct I/O mutating aggregate state, a Read Model coupled to an aggregate's mutable state, or a Policy performing side effects directly. These are serious divergences against Boundaries and Decisions.\n"
}

// rivalContractFieldBudget caps each Rival Contract section excerpt so
// the diff-check prompt stays within its existing 5000-byte budget.
const rivalContractFieldBudget = 800

// writeContractField appends a `### <name>` block with body text,
// truncated to a soft per-field budget. Empty fields are skipped.
func writeContractField(sb *strings.Builder, name, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	fmt.Fprintf(sb, "\n### %s\n", name)
	if len(body) > rivalContractFieldBudget {
		body = body[:rivalContractFieldBudget] + "\n(truncated; consult archive for full section)"
	}
	sb.WriteString(body)
	sb.WriteString("\n")
}
