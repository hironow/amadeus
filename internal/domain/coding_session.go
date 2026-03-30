package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Provider identifies which AI coding tool backs a session.
type Provider string

const (
	ProviderClaudeCode Provider = "claude-code"
	ProviderCodex      Provider = "codex"
	ProviderCopilot    Provider = "copilot"
	ProviderGeminiCLI  Provider = "gemini-cli"
	ProviderPi         Provider = "pi"
	ProviderKiro       Provider = "kiro"
)

var knownProviders = map[string]Provider{
	string(ProviderClaudeCode): ProviderClaudeCode,
	string(ProviderCodex):      ProviderCodex,
	string(ProviderCopilot):    ProviderCopilot,
	string(ProviderGeminiCLI):  ProviderGeminiCLI,
	string(ProviderPi):         ProviderPi,
	string(ProviderKiro):       ProviderKiro,
}

// ParseProvider validates and returns a Provider from a raw string.
func ParseProvider(s string) (Provider, error) {
	p, ok := knownProviders[s]
	if !ok {
		return "", fmt.Errorf("unknown provider: %q", s)
	}
	return p, nil
}

// SessionStatus tracks the lifecycle of a coding session.
type SessionStatus string

const (
	SessionRunning   SessionStatus = "running"
	SessionCompleted SessionStatus = "completed"
	SessionFailed    SessionStatus = "failed"
	SessionAbandoned SessionStatus = "abandoned"
)

// CodingSessionRecord is the persistent record of an AI coding session.
type CodingSessionRecord struct {
	ID                string
	ProviderSessionID string
	Provider          Provider
	Status            SessionStatus
	Model             string
	WorkDir           string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Metadata          map[string]string
}

// NewCodingSessionRecord creates a new record with status=running.
func NewCodingSessionRecord(provider Provider, model, workDir string) CodingSessionRecord {
	now := time.Now().UTC()
	return CodingSessionRecord{
		ID:        uuid.New().String(),
		Provider:  provider,
		Status:    SessionRunning,
		Model:     model,
		WorkDir:   workDir,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
	}
}

// Complete transitions from running to completed.
func (r *CodingSessionRecord) Complete(providerSessionID string) error {
	if r.Status != SessionRunning {
		return fmt.Errorf("cannot complete session in %q state", r.Status)
	}
	r.Status = SessionCompleted
	r.ProviderSessionID = providerSessionID
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// Fail transitions from running to failed.
func (r *CodingSessionRecord) Fail(reason string) error {
	if r.Status != SessionRunning {
		return fmt.Errorf("cannot fail session in %q state", r.Status)
	}
	r.Status = SessionFailed
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata["failure_reason"] = reason
	r.UpdatedAt = time.Now().UTC()
	return nil
}
