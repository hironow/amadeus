package session

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// InitAdapter implements port.InitRunner by delegating to session.InitGateDir.
type InitAdapter struct {
	Logger     domain.Logger
	LastResult *InitResult // populated after InitProject for display by cmd layer
}

// InitProject creates the state directory structure.
// Accepts WithLang option for CLI language override.
func (a *InitAdapter) InitProject(baseDir string, opts ...port.InitOption) ([]string, error) {
	cfg := port.ApplyInitOptions(opts...)
	stateDir := filepath.Join(baseDir, domain.StateDir)
	result, err := InitGateDir(stateDir, a.Logger, cfg.Lang)
	a.LastResult = result
	if err != nil {
		return nil, err
	}

	// Claude Code entry skill materialization (refs issue 0032 D5):
	// .claude/skills/review-gate makes /review-gate auto-discovered by a
	// bare `claude` session in this project.
	if err := InstallClaudeSkills(baseDir, platform.ClaudeSkillsFS, a.Logger); err != nil {
		result.Add(".claude/skills", InitWarning, fmt.Sprintf("failed to install claude skills: %v", err))
	} else {
		result.Add(".claude/skills/review-gate/", InitCreated, "")
	}
	return result.Warnings(), nil
}
