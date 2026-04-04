package platform

import "embed"

//go:embed templates/skills/*/SKILL.md
var SkillsFS embed.FS
