package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/spf13/cobra"
)

func newCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Run divergence check",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			full, _ := cmd.Flags().GetBool("full")
			quiet, _ := cmd.Flags().GetBool("quiet")
			jsonOut, _ := cmd.Flags().GetBool("json")
			lang, _ := cmd.Flags().GetString("lang")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, ".gate")

			// Pre-flight check: ensure init has been run
			if _, statErr := os.Stat(divRoot); statErr != nil {
				return fmt.Errorf("not initialized — run 'amadeus init' first")
			}

			// Preflight: verify required binaries exist
			bins := []string{"git"}
			if !dryRun {
				bins = append(bins, "claude")
			}
			if preErr := session.PreflightCheck(bins...); preErr != nil {
				return preErr
			}

			if err := session.InitGateDir(divRoot); err != nil {
				return fmt.Errorf("init .gate: %w", err)
			}

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if lang != "" {
				if !amadeus.ValidLang(lang) {
					return fmt.Errorf("unsupported language: %s (supported: ja, en)", lang)
				}
				cfg.Lang = lang
			}

			logger := loggerFrom(cmd)

			store := session.NewProjectionStore(divRoot)
			eventStore := session.NewEventStore(divRoot)

			outboxStore, err := session.NewOutboxStoreForGateDir(divRoot)
			if err != nil {
				return fmt.Errorf("outbox store: %w", err)
			}
			defer outboxStore.Close()

			// Wire approver
			autoApprove, _ := cmd.Flags().GetBool("auto-approve")
			approveCmd, _ := cmd.Flags().GetString("approve-cmd")

			var approver amadeus.Approver
			switch {
			case autoApprove:
				approver = &amadeus.AutoApprover{}
			case approveCmd != "":
				approver = session.NewCmdApprover(approveCmd)
			default:
				approver = &amadeus.AutoApprover{} // default: no gate
			}

			// Wire notifier
			notifyCmd, _ := cmd.Flags().GetString("notify-cmd")
			var notifier amadeus.Notifier
			if notifyCmd != "" {
				notifier = session.NewCmdNotifier(notifyCmd)
			} else {
				notifier = &amadeus.NopNotifier{}
			}

			reviewCmd, _ := cmd.Flags().GetString("review-cmd")

			a := &session.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    eventStore,
				Projector: &session.Projector{Store: store, OutboxStore: outboxStore},
				Git:       session.NewGitClient(repoRoot),
				RepoDir:   repoRoot,
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
				Approver:  approver,
				Notifier:  notifier,
				ReviewCmd: reviewCmd,
			}

			// COMMAND → usecase → Aggregate → EVENT
			return usecase.RunCheck(cmd.Context(), amadeus.ExecuteCheckCommand{
				RepoPath: repoRoot,
			}, amadeus.CheckOptions{
				Full:   full,
				DryRun: dryRun,
				Quiet:  quiet,
				JSON:   jsonOut,
			}, a)
		},
	}

	cmd.Flags().BoolP("dry-run", "n", false, "generate prompt only")
	cmd.Flags().BoolP("full", "f", false, "force full calibration check")
	cmd.Flags().BoolP("quiet", "q", false, "summary-only output")
	cmd.Flags().BoolP("json", "j", false, "output as JSON")
	cmd.Flags().Bool("auto-approve", false, "skip approval gate")
	cmd.Flags().String("approve-cmd", "", "external command for approval ({message} placeholder)")
	cmd.Flags().String("notify-cmd", "", "external command for notifications ({title} and {message} placeholders)")
	cmd.Flags().String("review-cmd", "", "code review command after check (exit 0=pass, non-zero=comments)")

	return cmd
}
