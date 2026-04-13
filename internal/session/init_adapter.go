package session

import (
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
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
	return result.Warnings(), nil
}
