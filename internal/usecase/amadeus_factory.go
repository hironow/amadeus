package usecase

import (
	"fmt"
	"io"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/port"
	"github.com/hironow/amadeus/internal/session"
)

// AmadeusParams holds all parameters needed to construct an Amadeus orchestrator.
// cmd layer populates this from CLI flags and passes it to usecase functions.
type AmadeusParams struct {
	GateDir     string
	RepoDir     string
	Config      domain.Config
	Logger      domain.Logger
	DataOut     io.Writer
	Approver    port.Approver
	Notifier    port.Notifier
	ReviewCmd   string
	ClaudeCmd   string
	ClaudeModel string
	WithGit     bool // true for commands that need git (e.g. check)
}

// buildResult holds the constructed Amadeus and a cleanup function.
type buildResult struct {
	Amadeus *session.Amadeus
	Cleanup func()
}

// buildAmadeus constructs a session.Amadeus from AmadeusParams.
// The returned cleanup function must be called (typically via defer) to close
// the outbox store. Returns an error if the outbox store cannot be opened.
func buildAmadeus(p AmadeusParams) (*buildResult, error) {
	store := session.NewProjectionStore(p.GateDir)
	eventStore := session.NewEventStore(p.GateDir)

	outboxStore, err := session.NewOutboxStoreForGateDir(p.GateDir)
	if err != nil {
		return nil, fmt.Errorf("outbox store: %w", err)
	}

	projector := &session.Projector{Store: store, OutboxStore: outboxStore}

	a := &session.Amadeus{
		Config:      p.Config,
		Store:       store,
		Events:      eventStore,
		Projector:   projector,
		Logger:      p.Logger,
		DataOut:     p.DataOut,
		Approver:    p.Approver,
		Notifier:    p.Notifier,
		ReviewCmd:   p.ReviewCmd,
		ClaudeCmd:   p.ClaudeCmd,
		ClaudeModel: p.ClaudeModel,
	}

	if p.WithGit {
		a.Git = session.NewGitClient(p.RepoDir)
		a.RepoDir = p.RepoDir
	}

	return &buildResult{
		Amadeus: a,
		Cleanup: func() { outboxStore.Close() },
	}, nil
}
