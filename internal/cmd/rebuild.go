package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
	"github.com/spf13/cobra"
)

func newRebuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild projections from event store",
		Long: "Replays all events from .gate/events/ to regenerate .run/ projection files and archive/ D-Mails from scratch.\n" +
			"NOTE: Inbox-sourced D-Mails (consumed via ScanInbox) are NOT reconstructed because\n" +
			"inbox.consumed events contain only metadata, not the full D-Mail content.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")

			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			divRoot := filepath.Join(repoRoot, ".gate")
			logger := amadeus.NewLogger(cmd.ErrOrStderr(), verbose)

			eventStore := &amadeus.FileEventStore{
				Dir: filepath.Join(divRoot, "events"),
			}
			store := amadeus.NewProjectionStore(divRoot)
			projector := &amadeus.Projector{Store: store}

			events, err := eventStore.LoadAll()
			if err != nil {
				return fmt.Errorf("load events: %w", err)
			}

			logger.Info("rebuilding projections from %d event(s)", len(events))

			if err := projector.Rebuild(events); err != nil {
				return fmt.Errorf("rebuild: %w", err)
			}

			logger.Info("rebuild complete")
			return nil
		},
	}

	return cmd
}
