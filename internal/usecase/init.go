package usecase

import "github.com/hironow/amadeus/internal/session"

// InitGate initializes the .gate directory structure at the given root path.
func InitGate(gateDir string) error {
	return session.InitGateDir(gateDir)
}
