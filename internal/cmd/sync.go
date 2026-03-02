package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [path]",
		Short: "Show D-Mail sync status (JSON)",
		Long:  "Output unsynced D-Mails and pending Linear comments as JSON.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}
			divRoot := filepath.Join(repoRoot, ".gate")

			if _, err := os.Stat(divRoot); errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf(".gate/ not found. Run 'amadeus init' first")
			}
			if err := session.InitGateDir(divRoot); err != nil {
				return fmt.Errorf("init gate dir: %w", err)
			}

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger := loggerFrom(cmd)
			store := session.NewProjectionStore(divRoot)

			outboxStore, err := session.NewOutboxStoreForGateDir(divRoot)
			if err != nil {
				return fmt.Errorf("outbox store: %w", err)
			}
			defer outboxStore.Close()

			a := &session.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    session.NewEventStore(divRoot),
				Projector: &session.Projector{Store: store, OutboxStore: outboxStore},
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
			}
			return usecase.PrintSync(amadeus.RunSyncCommand{
				RepoPath: repoRoot,
			}, a)
		},
	}

	return cmd
}
