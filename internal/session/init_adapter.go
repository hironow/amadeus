package session

import "github.com/hironow/amadeus/internal/domain"

// InitAdapter implements port.InitRunner by delegating to session.InitGateDir.
type InitAdapter struct {
	Logger domain.Logger
}

// InitGateDir creates the state directory structure.
func (a *InitAdapter) InitGateDir(stateDir string) error {
	return InitGateDir(stateDir, a.Logger)
}
