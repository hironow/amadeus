package cmd

import (
	"context"
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

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run continuous divergence check and PR convergence",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			full, _ := cmd.Flags().GetBool("full")
			quiet, _ := cmd.Flags().GetBool("quiet")
			jsonOut, _ := cmd.Flags().GetBool("json")
			lang, _ := cmd.Flags().GetString("lang")
			baseBranch, _ := cmd.Flags().GetString("base")

			repoRoot, err := resolveTargetDir(args)
			if err != nil {
				return err
			}

			divRoot := filepath.Join(repoRoot, domain.StateDir)

			// Pre-flight check: ensure init has been run
			if _, statErr := os.Stat(divRoot); statErr != nil {
				return fmt.Errorf("not initialized — run 'amadeus init' first")
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

			// Preflight: verify required binaries exist
			bins := []string{"git"}
			// gh is needed for PR reader (pre-merge pipeline)
			// claude is needed for post-merge pipeline (when --base is set) and not dry-run
			if baseBranch != "" {
				bins = append(bins, "gh")
				if !dryRun {
					bins = append(bins, cfg.ClaudeCmd)
				}
			}
			if preErr := session.PreflightCheck(bins...); preErr != nil {
				return preErr
			}

			if cmd.Flags().Changed("wait-timeout") {
				cfg.WaitTimeout, _ = cmd.Flags().GetDuration("wait-timeout")
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

			// PRReader requires gh CLI — only create when --base is set
			var prReader *session.GhPRReader
			if baseBranch != "" {
				prReader = session.NewGhPRReader(repoRoot)
			}

			insightWriter := session.NewInsightWriter(
				filepath.Join(divRoot, "insights"),
				filepath.Join(divRoot, ".run"),
			)

			a := &session.Amadeus{
				Config:      cfg,
				Store:       store,
				Events:      eventStore,
				Projector:   projector,
				Git:         git,
				RepoDir:     repoRoot,
				Logger:      logger,
				DataOut:     cmd.OutOrStdout(),
				Approver:    approver,
				Notifier:    notifier,
				Metrics:     &platform.OTelPolicyMetrics{},
				ReviewCmd:   reviewCmd,
				ClaudeCmd:   cfg.ClaudeCmd,
				ClaudeModel: cfg.Model,
				PRReader:    prReader,
				Insights:    insightWriter,
			}

			// Parse -> COMMAND -> usecase -> EventEmitter -> EVENT
			rp, rpErr := domain.NewRepoPath(repoRoot)
			if rpErr != nil {
				return rpErr
			}

			checkOpts := domain.CheckOptions{
				Full:   full,
				DryRun: dryRun,
				Quiet:  quiet,
				JSON:   jsonOut,
			}

			// With --base: daemon loop with inbox monitoring + post-merge checks.
			// Adds PR convergence analysis (via PRReader/gh) on top of divergence scoring.
			if baseBranch != "" {
				runErr := usecase.Run(cmd.Context(), domain.NewExecuteRunCommand(rp, baseBranch), domain.RunOptions{
					CheckOptions: checkOpts,
					BaseBranch:   baseBranch,
				}, a, cfg, logger, notifier, &platform.OTelPolicyMetrics{})
				return tryWriteHandover(cmd.Context(), runErr, repoRoot, "divergence run with --base "+baseBranch, logger)
			}

			// Without --base: one-shot check + D-Mail waiting loop.
			// RunCheck runs Phase 0-4 including generateDMails (Phase 3), which
			// produces KindImplFeedback / KindDesignFeedback from divergence scoring.
			// PR convergence analysis is skipped (PRReader=nil), but divergence-based
			// impl feedback is still generated and flushed to outbox.
			metrics := &platform.OTelPolicyMetrics{}
			checkErr := usecase.RunCheck(cmd.Context(), domain.NewExecuteCheckCommand(rp), checkOpts,
				a, cfg, logger, notifier, metrics)
			if checkErr != nil {
				if _, ok := checkErr.(*domain.DriftError); !ok {
					return checkErr
				}
			}

			// Skip waiting in dry-run, one-shot (--full/--json), or when explicitly disabled
			if dryRun || full || jsonOut || cfg.WaitTimeout < 0 {
				return checkErr
			}

			// Start inbox monitor for waiting phase
			inboxCh, monErr := session.MonitorInbox(cmd.Context(), divRoot, logger)
			if monErr != nil {
				return fmt.Errorf("inbox monitor: %w", monErr)
			}

			// Waiting loop: wait for D-Mail → re-check → repeat.
			// Skips expensive RunCheck after consecutive no-drift results
			// while always draining the inbox channel promptly.
			waitErr := runWaitingLoop(
				cmd.Context(),
				func(ctx context.Context) error {
					return usecase.RunCheck(ctx, domain.NewExecuteCheckCommand(rp), checkOpts,
						a, cfg, logger, notifier, metrics)
				},
				func(ctx context.Context) (bool, error) {
					return session.WaitForDMail(ctx, inboxCh, cfg.WaitTimeout, logger)
				},
				logger,
			)
			return tryWriteHandover(cmd.Context(), waitErr, repoRoot, "divergence check waiting loop", logger)
		},
	}

	// Inherit all check flags
	cmd.Flags().BoolP("dry-run", "n", false, "generate prompt only (post-merge)")
	cmd.Flags().BoolP("full", "f", false, "force full calibration check")
	cmd.Flags().BoolP("quiet", "q", false, "summary-only output")
	cmd.Flags().BoolP("json", "j", false, "output as JSON")
	cmd.Flags().Bool("auto-approve", false, "skip approval gate")
	cmd.Flags().String("approve-cmd", "", "external command for approval ({message} placeholder)")
	cmd.Flags().String("notify-cmd", "", "external command for notifications ({title} and {message} placeholders)")
	cmd.Flags().String("review-cmd", "", "code review command after check (exit 0=pass, non-zero=comments)")
	// New flag for run
	cmd.Flags().String("base", "", "upstream branch for post-merge divergence check")
	cmd.Flags().Duration("wait-timeout", domain.DefaultWaitTimeout, "D-Mail waiting phase timeout (0 = 24h safety cap, negative = disable waiting)")

	return cmd
}
