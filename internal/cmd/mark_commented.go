package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newMarkCommentedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mark-commented <dmail-name> <issue-id> [path]",
		Short: "Record that a D-Mail has been posted as a comment",
		Long:  "Mark a D-Mail × Issue pair as commented in the sync state.",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			dmailName := args[0]
			issueID := args[1]
			jsonFlag, _ := cmd.Flags().GetBool("json")

			repoRoot, err := resolveTargetDir(args[2:])
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, domain.StateDir)

			if _, err := os.Stat(divRoot); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf(".gate/ not found. Run 'amadeus init' first")
				}
				return fmt.Errorf("stat .gate directory: %w", err)
			}

			logger := loggerFrom(cmd)

			// Composition root: wire session.Amadeus
			store := session.NewProjectionStore(divRoot)
			eventStore := session.NewEventStore(divRoot, logger)
			outbox, outboxErr := session.NewOutboxStoreForDir(divRoot)
			if outboxErr != nil {
				return fmt.Errorf("outbox store: %w", outboxErr)
			}
			defer outbox.Close()

			projector := &session.Projector{Store: store, OutboxStore: outbox}
			cfg := domain.DefaultConfig()

			agg := domain.NewCheckAggregate(cfg)
			emitter := usecase.NewCheckEventEmitter(agg, eventStore, projector, nil, logger)

			a := &session.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    eventStore,
				Projector: projector,
				Logger:    logger,
				Emitter:   emitter,
			}

			if err := a.MarkCommented(dmailName, issueID); err != nil {
				return fmt.Errorf("mark commented: %w", err)
			}

			if jsonFlag {
				out := struct {
					DMail   string `json:"dmail"`
					IssueID string `json:"issue_id"`
					Status  string `json:"status"`
				}{
					DMail:   dmailName,
					IssueID: issueID,
					Status:  "commented",
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Marked %s:%s as commented.\n", dmailName, issueID)
			return nil
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}
