package session

import (
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// InitAdapter implements port.InitRunner by delegating to session.InitGateDir.
type InitAdapter struct {
	Logger domain.Logger
}

// InitProject creates the state directory structure.
// amadeus uses only baseDir; opts are ignored (no team/project/lang/strictness).
func (a *InitAdapter) InitProject(baseDir string, _ ...port.InitOption) ([]string, error) {
	stateDir := filepath.Join(baseDir, domain.StateDir)
	return nil, InitGateDir(stateDir, a.Logger)
}
