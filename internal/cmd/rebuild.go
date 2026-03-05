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
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, ".gate")
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

			return usecase.Rebuild(domain.RebuildCommand{
				RepoPath: repoRoot,
			}, eventStore, projector, logger)
		},
	}

	return cmd
}
