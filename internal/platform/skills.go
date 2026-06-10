package platform

import "embed"

//go:embed templates/skills/*/SKILL.md
var SkillsFS embed.FS

// ClaudeSkillsFS embeds the Claude Code entry skills that `amadeus
// init` materializes into the target project's .claude/skills/ for
// bare-`claude` auto-discovery (refs issue 0032, decision D5).
//
//go:embed all:templates/claude-skills
var ClaudeSkillsFS embed.FS
