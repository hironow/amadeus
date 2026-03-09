package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/session"
	"github.com/hironow/amadeus/internal/usecase"
	"github.com/hironow/amadeus/internal/usecase/port"
	"github.com/spf13/cobra"
)

func newCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "check [path]",
		Short:      "Run divergence check",
		Deprecated: "use 'amadeus run' instead",
		Args:       cobra.MaximumNArgs(1),
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

			divRoot := filepath.Join(repoRoot, domain.StateDir)

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

			logger := loggerFrom(cmd)

			if err := session.InitGateDir(divRoot, logger); err != nil {
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
				if !domain.ValidLang(lang) {
					return fmt.Errorf("unsupported language: %s (supported: ja, en)", lang)
				}
				cfg.Lang = lang
			}

			// Wire approver
			autoApprove, _ := cmd.Flags().GetBool("auto-approve")
			approveCmd, _ := cmd.Flags().GetString("approve-cmd")

			var approver port.Approver
			switch {
			case autoApprove:
				approver = &port.AutoApprover{}
			case approveCmd != "":
				approver = session.NewCmdApprover(approveCmd)
			default:
				approver = &port.AutoApprover{} // default: no gate
			}

			// Wire notifier
			notifyCmd, _ := cmd.Flags().GetString("notify-cmd")
			var notifier port.Notifier
			if notifyCmd != "" {
				notifier = session.NewCmdNotifier(notifyCmd)
			} else {
				notifier = &port.NopNotifier{}
			}

			reviewCmd, _ := cmd.Flags().GetString("review-cmd")

			// Composition root: wire session.Amadeus
			store := session.NewProjectionStore(divRoot)
			eventStore := session.NewEventStore(divRoot, logger)
			outbox, outboxErr := session.NewOutboxStoreForDir(divRoot)
			if outboxErr != nil {
				return fmt.Errorf("outbox store: %w", outboxErr)
			}
			defer outbox.Close()

			projector := &session.Projector{Store: store, OutboxStore: outbox}
			git := session.NewGitClient(repoRoot)

			a := &session.Amadeus{
				Config:    cfg,
				Store:     store,
				Events:    eventStore,
				Projector: projector,
				Git:       git,
				RepoDir:   repoRoot,
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
				Approver:  approver,
				Notifier:  notifier,
				Metrics:   &platform.OTelPolicyMetrics{},
				ReviewCmd: reviewCmd,
			}

			// Parse → COMMAND → usecase → EventEmitter → EVENT
			rp, rpErr := domain.NewRepoPath(repoRoot)
			if rpErr != nil {
				return rpErr
			}
			return usecase.RunCheck(cmd.Context(), domain.NewExecuteCheckCommand(rp), domain.CheckOptions{
				Full:   full,
				DryRun: dryRun,
				Quiet:  quiet,
				JSON:   jsonOut,
			}, a, cfg, logger, notifier, &platform.OTelPolicyMetrics{})
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
