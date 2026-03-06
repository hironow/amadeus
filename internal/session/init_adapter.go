package session

// InitAdapter implements port.InitRunner by delegating to session.InitGateDir.
type InitAdapter struct{}

// InitGateDir creates the state directory structure.
func (a *InitAdapter) InitGateDir(stateDir string) error {
	return InitGateDir(stateDir)
}
