package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/spf13/cobra"
)

func newLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log [path]",
		Short: "Show divergence log",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			jsonOut, _ := cmd.Flags().GetBool("json")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, domain.StateDir)

			if _, err := os.Stat(divRoot); errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf(".gate/ not found. Run 'amadeus init' first")
			}

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
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

			a := &session.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    eventStore,
				Projector: projector,
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
			}

			if jsonOut {
				return a.PrintLogJSON()
			}
			return a.PrintLog()
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}
