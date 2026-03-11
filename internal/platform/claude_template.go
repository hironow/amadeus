package platform

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/hironow/amadeus/internal/domain"
)

//go:embed templates/*.md.tmpl
var templateFS embed.FS

//go:embed templates/skills/*/SKILL.md
var SkillTemplateFS embed.FS

// BuildDiffCheckPrompt renders the diff_check template for the given language.
func BuildDiffCheckPrompt(lang string, params domain.DiffCheckParams) (string, error) {
	name := fmt.Sprintf("templates/diff_check_%s.md.tmpl", lang)
	return renderTemplate(name, params)
}

// BuildFullCheckPrompt renders the full_check template for the given language.
func BuildFullCheckPrompt(lang string, params domain.FullCheckParams) (string, error) {
	name := fmt.Sprintf("templates/full_check_%s.md.tmpl", lang)
	return renderTemplate(name, params)
}

// BuildFileRefDiffCheckPrompt renders the file-reference diff_check template for the given language.
func BuildFileRefDiffCheckPrompt(lang string, params domain.FileRefDiffCheckParams) (string, error) {
	name := fmt.Sprintf("templates/fileref_diff_check_%s.md.tmpl", lang)
	return renderTemplate(name, params)
}

// BuildFileRefFullCheckPrompt renders the file-reference full_check template for the given language.
func BuildFileRefFullCheckPrompt(lang string, params domain.FileRefFullCheckParams) (string, error) {
	name := fmt.Sprintf("templates/fileref_full_check_%s.md.tmpl", lang)
	return renderTemplate(name, params)
}

// renderTemplate parses and executes a template from the embedded filesystem.
func renderTemplate(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(templateFS, name)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}
