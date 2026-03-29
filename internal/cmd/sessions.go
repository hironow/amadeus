package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase/port"
	"github.com/spf13/cobra"
)

func newSessionsCmd() *cobra.Command {
	sessCmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage AI coding sessions",
	}
	sessCmd.AddCommand(
		newSessionsListCmd(),
		newSessionsEnterCmd(),
	)
	return sessCmd
}

func newSessionsListCmd() *cobra.Command {
	var (
		statusFilter string
		limit        int
	)
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded coding sessions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := cmd.Context().Value(loggerKey).(domain.Logger)
			repoRoot := resolveRepoRoot(cmd)
			dbPath := filepath.Join(repoRoot, domain.StateDir, ".run", "sessions.db")

			store, err := session.NewSQLiteCodingSessionStore(dbPath)
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}
			defer store.Close()

			opts := port.ListSessionOpts{Limit: limit}
			if statusFilter != "" {
				s := domain.SessionStatus(statusFilter)
				opts.Status = &s
			}

			records, err := store.List(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt == "json" {
				return json.NewEncoder(os.Stdout).Encode(records)
			}

			if len(records) == 0 {
				logger.Info("No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPROVIDER\tSTATUS\tMODEL\tPROVIDER_SESSION\tCREATED")
			for _, r := range records {
				pid := r.ProviderSessionID
				if len(pid) > 12 {
					pid = pid[:12] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.ID, r.Provider, r.Status, r.Model,
					pid, r.CreatedAt.Format("2006-01-02 15:04"))
			}
			return w.Flush()
		},
	}
	listCmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (running, completed, failed, abandoned)")
	listCmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return listCmd
}

func newSessionsEnterCmd() *cobra.Command {
	var providerID string
	enterCmd := &cobra.Command{
		Use:   "enter [session-record-id]",
		Short: "Re-enter an AI coding session interactively",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot := resolveRepoRoot(cmd)
			dbPath := filepath.Join(repoRoot, domain.StateDir, ".run", "sessions.db")

			store, err := session.NewSQLiteCodingSessionStore(dbPath)
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}
			defer store.Close()

			var rec domain.CodingSessionRecord
			if len(args) == 1 {
				rec, err = store.Load(cmd.Context(), args[0])
				if err != nil {
					return fmt.Errorf("load session %q: %w", args[0], err)
				}
			} else if providerID != "" {
				rec, err = store.LatestByProviderSessionID(cmd.Context(), domain.ProviderClaudeCode, providerID)
				if err != nil {
					return fmt.Errorf("find session by provider ID %q: %w", providerID, err)
				}
			} else {
				return fmt.Errorf("provide a session record ID or --provider-id")
			}

			if rec.ProviderSessionID == "" {
				return fmt.Errorf("session %q has no provider session ID", rec.ID)
			}
			if rec.WorkDir == "" {
				return fmt.Errorf("session %q has no WorkDir recorded", rec.ID)
			}

			configPath, _ := cmd.Flags().GetString("config")
			if configPath == "" {
				configPath = filepath.Join(repoRoot, domain.StateDir, "config.yaml")
			}
			cfg, _ := loadConfig(configPath)

			enterCfg := session.EnterConfig{
				ProviderCmd:       cfg.ClaudeCmd,
				ProviderSessionID: rec.ProviderSessionID,
				WorkDir:           rec.WorkDir,
				ConfigBase:        repoRoot,
				Stdin:             os.Stdin,
				Stdout:            os.Stdout,
				Stderr:            os.Stderr,
			}
			return session.EnterSession(cmd.Context(), enterCfg)
		},
	}
	enterCmd.Flags().StringVar(&providerID, "provider-id", "", "Resume by provider session ID directly")
	return enterCmd
}

// resolveRepoRoot returns the repo root from the config flag or cwd.
func resolveRepoRoot(cmd *cobra.Command) string {
	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		// Config path is relative to repo root (e.g. .gate/config.yaml)
		abs, _ := filepath.Abs(filepath.Dir(filepath.Dir(configPath)))
		return abs
	}
	cwd, _ := os.Getwd()
	return cwd
}
