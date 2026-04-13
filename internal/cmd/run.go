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
		Long: `Run continuous divergence checking with D-Mail generation and optional
PR convergence analysis.

Without --base: performs a one-shot divergence check (phases 0-4),
generates D-Mails from divergence scoring, then enters a waiting loop
that monitors the inbox for incoming D-Mails and re-checks on arrival.

With --base: runs a daemon loop that monitors the inbox and performs
post-merge divergence checks against the specified upstream branch,
adding PR convergence analysis via the gh CLI on top of divergence
scoring.

If [path] is omitted, the current working directory is used. Requires
'amadeus init' to have been run first.`,
		Example: `  # One-shot divergence check with D-Mail waiting loop
  amadeus run

  # Continuous post-merge check against main branch
  amadeus run --base main

  # Dry-run mode (generate prompts without executing)
  amadeus run --dry-run

  # Full calibration check with JSON output
  amadeus run --full --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			full, _ := cmd.Flags().GetBool("full")
			quiet, _ := cmd.Flags().GetBool("quiet")
			jsonOut, _ := cmd.Flags().GetBool("json")
			lang, _ := cmd.Flags().GetString("lang")
			baseBranch, _ := cmd.Flags().GetString("base")
			collectorEnable, _ := cmd.Flags().GetBool("collector-enable")
			collectorDisable, _ := cmd.Flags().GetBool("collector-disable")
			collectorProjectID, _ := cmd.Flags().GetString("collector-project-id")
			collectorAPIURL, _ := cmd.Flags().GetString("collector-api-url")
			collectorQueryLimit, _ := cmd.Flags().GetInt("collector-query-limit")
			collectorFeedbackTypes, _ := cmd.Flags().GetStringSlice("collector-feedback-type")

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

			if _, err := session.InitGateDir(divRoot, logger, ""); err != nil {
				return fmt.Errorf("init .gate: %w", err)
			}

			// Acquire daemon lock — prevents multiple instances on the same directory
			runDir := filepath.Join(divRoot, ".run")
			unlock, lockErr := session.TryLockDaemon(runDir)
			if lockErr != nil {
				return fmt.Errorf("daemon lock: %w", lockErr)
			}
			defer unlock()

			if configPath == "" {
				configPath = filepath.Join(divRoot, "config.yaml")
			}
			cfg, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if collectorEnable && collectorDisable {
				return fmt.Errorf("--collector-enable and --collector-disable cannot be used together")
			}
			if collectorEnable {
				enabled := true
				cfg.ImprovementCollector.Enabled = &enabled
			}
			if collectorDisable {
				enabled := false
				cfg.ImprovementCollector.Enabled = &enabled
			}
			if cmd.Flags().Changed("collector-project-id") {
				cfg.ImprovementCollector.ProjectID = collectorProjectID
			}
			if cmd.Flags().Changed("collector-api-url") {
				cfg.ImprovementCollector.APIURL = collectorAPIURL
			}
			if cmd.Flags().Changed("collector-query-limit") {
				cfg.ImprovementCollector.QueryLimit = collectorQueryLimit
			}
			if cmd.Flags().Changed("collector-feedback-type") {
				cfg.ImprovementCollector.FeedbackTypes = collectorFeedbackTypes
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

			// Initialize process-wide circuit breaker for rate limit / server error protection
			session.SetCircuitBreaker(platform.NewCircuitBreaker(logger))

			if cmd.Flags().Changed("idle-timeout") {
				cfg.IdleTimeout, _ = cmd.Flags().GetDuration("idle-timeout")
			}

			if lang != "" {
				if !domain.ValidLang(lang) {
					return fmt.Errorf("unsupported language: %s (supported: ja, en)", lang)
				}
				cfg.Lang = lang
			}

			// Wire approver (default: no gate — amadeus uses explicit --approve-cmd to enable)
			approveCmd, _ := cmd.Flags().GetString("approve-cmd")
			autoApprove := approveCmd == ""
			if v, _ := cmd.Flags().GetBool("auto-approve"); v {
				autoApprove = true
			}
			approver := session.BuildApprover(
				domain.FlagApproverConfig{AutoApprove: autoApprove, ApproveCmd: approveCmd},
				cmd.InOrStdin(), cmd.ErrOrStderr(),
			)

			// Wire notifier
			notifyCmd, _ := cmd.Flags().GetString("notify-cmd")
			var notifier port.Notifier
			if notifyCmd != "" {
				notifier = session.NewCmdNotifier(notifyCmd)
			} else {
				notifier = &port.NopNotifier{}
			}

			reviewCmd, _ := cmd.Flags().GetString("review-cmd")

			// One-time cutover: migrate to global SeqNr (ADR S0040, idempotent)
			var seqAlloc port.SeqAllocator
			if !dryRun {
				var closeSeq func()
				var cutoverErr error
				seqAlloc, closeSeq, cutoverErr = session.EnsureCutover(cmd.Context(), divRoot, "amadeus.state", logger)
				if cutoverErr != nil {
					return fmt.Errorf("cutover: %w", cutoverErr)
				}
				defer closeSeq()
			}

			// Composition root: wire session.Amadeus
			store := session.NewProjectionStore(divRoot)
			eventStore := session.NewEventStore(divRoot, logger)
			outbox, outboxErr := session.NewOutboxStoreForDir(repoRoot)
			if outboxErr != nil {
				return fmt.Errorf("outbox store: %w", outboxErr)
			}
			defer outbox.Close()

			projector := &session.Projector{Store: store, OutboxStore: outbox}
			git := session.NewGitClient(repoRoot)

			// PRReader/PRWriter/IssueWriter require gh CLI — only create when --base is set
			var prReader *session.GhPRReader
			var prWriter *session.GhPRWriter
			var issueWriter *session.GhIssueWriter
			if baseBranch != "" {
				prReader = session.NewGhPRReader(repoRoot)
				prWriter = session.NewGhPRWriter(repoRoot)
				issueWriter = session.NewGhIssueWriter(repoRoot)
			}

			insightWriter := session.NewInsightWriter(
				filepath.Join(divRoot, "insights"),
				filepath.Join(divRoot, ".run"),
			)
			collector, closeCollector, collectorErr := session.NewImprovementCollector(repoRoot, cfg.ImprovementCollector, insightWriter, logger)
			if collectorErr != nil {
				return fmt.Errorf("improvement collector: %w", collectorErr)
			}
			defer closeCollector()

			routingPolicy, policyErr := session.LoadRoutingPolicy(divRoot)
			if policyErr != nil {
				logger.Warn("routing policy load: %v (using default)", policyErr)
				routingPolicy = domain.DefaultRoutingPolicy()
			}

			// ImprovementTaskDispatcher: SQLite-backed dedup in production, nop in dry-run
			var dispatcher port.ImprovementTaskDispatcher
			if dryRun {
				dispatcher = &port.NopImprovementTaskDispatcher{}
			} else {
				d, dispatcherErr := session.NewImprovementTaskDispatcher(divRoot, logger)
				if dispatcherErr != nil {
					return fmt.Errorf("improvement task dispatcher: %w", dispatcherErr)
				}
				defer d.Close()
				dispatcher = d
			}

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
				PRWriter:    prWriter,
				IssueWriter: issueWriter,
				SeqAlloc:    seqAlloc,
				Insights:    insightWriter,
				Collector:   collector,
				Policy:      routingPolicy,
				Dispatcher:  dispatcher,
			}

			defer a.CloseRunner()

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
				noMerge, _ := cmd.Flags().GetBool("no-merge")
				autoMerge := !noMerge // default ON when --base is set
				runErr := usecase.Run(cmd.Context(), domain.NewExecuteRunCommand(rp, baseBranch), domain.RunOptions{
					CheckOptions: checkOpts,
					BaseBranch:   baseBranch,
					AutoMerge:    autoMerge,
					ReadyLabel:   "sightjack:ready",
				}, a, cfg, logger, notifier, &platform.OTelPolicyMetrics{}, prReader, store, dispatcher)
				return tryWriteHandover(cmd.Context(), runErr, repoRoot, domain.HandoverState{
					Tool:       "amadeus",
					Operation:  "divergence",
					InProgress: "divergence run with --base " + baseBranch,
					PartialState: map[string]string{
						"BaseBranch": baseBranch,
						"DryRun":     fmt.Sprintf("%v", checkOpts.DryRun),
						"Full":       fmt.Sprintf("%v", checkOpts.Full),
					},
				}, logger)
			}

			// Without --base: one-shot check + D-Mail waiting loop.
			// RunCheck runs Phase 0-4 including generateDMails (Phase 3), which
			// produces KindImplFeedback / KindDesignFeedback from divergence scoring.
			// PR convergence analysis is skipped (PRReader=nil), but divergence-based
			// impl feedback is still generated and flushed to outbox.
			metrics := &platform.OTelPolicyMetrics{}
			checkErr := usecase.RunCheck(cmd.Context(), domain.NewExecuteCheckCommand(rp), checkOpts,
				a, cfg, logger, notifier, metrics, dispatcher)
			if checkErr != nil {
				if _, ok := checkErr.(*domain.DriftError); !ok {
					return tryWriteHandover(cmd.Context(), checkErr, repoRoot, domain.HandoverState{
						Tool:       "amadeus",
						Operation:  "divergence",
						InProgress: "initial divergence check (no --base)",
						PartialState: map[string]string{
							"DryRun": fmt.Sprintf("%v", checkOpts.DryRun),
							"Full":   fmt.Sprintf("%v", checkOpts.Full),
						},
					}, logger)
				}
			}

			// Skip waiting in dry-run, one-shot (--full/--json), or when explicitly disabled
			if dryRun || full || jsonOut || cfg.IdleTimeout < 0 {
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
						a, cfg, logger, notifier, metrics, dispatcher)
				},
				func(ctx context.Context) (bool, error) {
					return session.WaitForDMail(ctx, inboxCh, cfg.IdleTimeout, logger)
				},
				logger,
			)
			if waitErr != nil {
				return tryWriteHandover(cmd.Context(), waitErr, repoRoot, domain.HandoverState{
					Tool:         "amadeus",
					Operation:    "divergence",
					InProgress:   "divergence check waiting loop",
					Completed:    []string{"Initial divergence check completed"},
					Remaining:    []string{"D-Mail waiting loop interrupted"},
					PartialState: map[string]string{"Phase": "waiting"},
				}, logger)
			}
			// waitErr==nil but ctx may be cancelled (WaitForDMail returns (false, nil) on cancel)
			writeHandoverOnCancel(cmd.Context(), repoRoot, domain.HandoverState{
				Tool:         "amadeus",
				Operation:    "divergence",
				InProgress:   "D-Mail waiting phase (clean exit on Ctrl+C)",
				Completed:    []string{"Initial divergence check completed"},
				PartialState: map[string]string{"Phase": "waiting-cancelled"},
			}, logger)
			return waitErr
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
	cmd.Flags().Bool("collector-enable", false, "force-enable the improvement collector")
	cmd.Flags().Bool("collector-disable", false, "disable the improvement collector even when env vars are present")
	cmd.Flags().String("collector-project-id", "", "override the Weave/W&B project id for the improvement collector")
	cmd.Flags().String("collector-api-url", "", "override the Weave API base URL for the improvement collector")
	cmd.Flags().Int("collector-query-limit", 0, "override the improvement collector query limit (0 = default)")
	cmd.Flags().StringSlice("collector-feedback-type", nil, "restrict the improvement collector to specific feedback types")
	// New flag for run
	cmd.Flags().String("base", "", "upstream branch for post-merge divergence check")
	cmd.Flags().Bool("no-merge", false, "disable automatic PR merging (only effective with --base)")
	cmd.Flags().Duration("idle-timeout", domain.DefaultIdleTimeout, "idle timeout — exit after no D-Mail activity (0 = 24h safety cap, negative = disable)")

	return cmd
}
