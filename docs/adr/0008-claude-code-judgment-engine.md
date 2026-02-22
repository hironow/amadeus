# 0008. Claude Code as Judgment Engine

**Date:** 2026-02-23
**Status:** Accepted

## Context

Integrity verification requires evaluating whether code changes comply with
architectural decisions, definitions of done, and dependency constraints. This
evaluation is inherently qualitative — rule-based static analysis cannot capture
the nuanced reasoning needed to assess compliance with prose-based ADRs and DoDs.

A language model can perform this judgment, but the integration must be
deterministic in its interface (structured input/output) while leveraging the
model's reasoning capabilities.

## Decision

Use Claude Code CLI as the judgment engine, invoked as a subprocess with
structured JSON I/O.

### Invocation

```
claude --model opus --output-format json --dangerously-skip-permissions --print
```

- **`--model opus`**: Uses the most capable model for complex architectural reasoning.
- **`--output-format json`**: Ensures machine-parseable output on stdout.
- **`--dangerously-skip-permissions`**: Required for non-interactive execution
  (no TTY prompts).
- **`--print`**: Suppresses interactive mode, outputs result and exits.
- **stdin**: Prompt is piped via stdin (`cmd.Stdin = bytes.NewBufferString(prompt)`).

### Prompt Templates

Prompts are embedded via `//go:embed templates/*.md.tmpl` and rendered with
Go's `text/template`:

- **`diff_check_{lang}.md.tmpl`**: For incremental checks. Receives
  `DiffCheckParams` (previous scores, PR diffs, relevant ADRs, linked DoDs,
  linked issue IDs).
- **`full_check_{lang}.md.tmpl`**: For full calibration. Receives
  `FullCheckParams` (codebase structure, all ADRs, recent DoDs, dependency map).
- **Language variants**: `ja` (Japanese) and `en` (English), selected via
  `Config.Lang`.

### Response Contract

Claude returns a `ClaudeResponse` JSON object:

```json
{
  "axes": { "adr_integrity": {"score": 25, "details": "..."}, ... },
  "dmails": [{"description": "...", "detail": "...", "issues": [...], "targets": [...]}],
  "reasoning": "...",
  "impact_radius": [{"area": "...", "impact": "direct", "detail": "..."}]
}
```

- **`axes`**: Four-axis scores (0-100) with detail explanations.
- **`dmails`**: D-Mail candidates identified by the model.
- **`reasoning`**: Free-text explanation of the overall assessment.
- **`impact_radius`**: Structural impact analysis (direct/indirect/transitive).

### Testability

`runClaude` is a package-level function variable (`var runClaude = defaultRunClaude`),
allowing tests to inject a fake implementation that returns predetermined
`ClaudeResponse` JSON without invoking the real CLI.

## Consequences

### Positive
- Qualitative reasoning capability that static analysis cannot provide
- Structured JSON contract enables deterministic downstream processing
- Language-specific prompts support bilingual (ja/en) operation
- Function variable pattern enables comprehensive unit testing without Claude

### Negative
- Runtime dependency on Claude CLI binary (must be installed and authenticated)
- Model output is non-deterministic — same input may produce different scores
- Prompt quality directly impacts evaluation accuracy
- Subprocess invocation adds latency compared to in-process evaluation
