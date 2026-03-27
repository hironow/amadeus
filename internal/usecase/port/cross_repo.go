package port

import "github.com/hironow/amadeus/internal/domain"

// CrossRepoReader reads divergence state from sibling tool state directories.
type CrossRepoReader interface {
	ReadToolSnapshot(tool domain.ToolName, stateDir string) (domain.ToolSnapshot, error)
}
