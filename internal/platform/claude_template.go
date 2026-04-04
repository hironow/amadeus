package platform

import "embed"

//go:embed templates/skills/*/SKILL.md
var SkillTemplateFS embed.FS
