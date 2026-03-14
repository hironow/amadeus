package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newRebuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebuild [path]",
		Short: "Rebuild projections from event store",
		Long: "Replays all events from .gate/events/ to regenerate .run/ projection files and archive/ D-Mails from scratch.\n" +
			"NOTE: Inbox-sourced D-Mails (consumed via ScanInbox) are NOT reconstructed because\n" +
			"inbox.consumed events contain only metadata, not the full D-Mail content.",
		Example: `  # Rebuild projections in current directory
  amadeus rebuild

  # Rebuild for a specific project
  amadeus rebuild /path/to/project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, domain.StateDir)
			logger := loggerFrom(cmd)

			// Composition root: wire stores directly
			eventStore := session.NewEventStore(divRoot, logger)
			store := session.NewProjectionStore(divRoot)
			outbox, outboxErr := session.NewOutboxStoreForDir(divRoot)
			if outboxErr != nil {
				return fmt.Errorf("outbox store: %w", outboxErr)
			}
			defer outbox.Close()

			projector := &session.Projector{Store: store, OutboxStore: outbox}

			rp, rpErr := domain.NewRepoPath(repoRoot)
			if rpErr != nil {
				return rpErr
			}
			return usecase.Rebuild(domain.NewRebuildCommand(rp), eventStore, projector, logger)
		},
	}

	return cmd
}
