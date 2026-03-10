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
			// git and gh are always needed (gh for PR reader)
			bins := []string{"git", "gh"}
			// claude is needed only for post-merge pipeline (when --base is set) and not dry-run
			if baseBranch != "" && !dryRun {
				bins = append(bins, cfg.ClaudeCmd)
			}
			if preErr := session.PreflightCheck(bins...); preErr != nil {
				return preErr
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
			prReader := session.NewGhPRReader(repoRoot)

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
			}

			// Parse -> COMMAND -> usecase -> EventEmitter -> EVENT
			rp, rpErr := domain.NewRepoPath(repoRoot)
			if rpErr != nil {
				return rpErr
			}
			return usecase.Run(cmd.Context(), domain.NewExecuteRunCommand(rp, baseBranch), domain.RunOptions{
				CheckOptions: domain.CheckOptions{
					Full:   full,
					DryRun: dryRun,
					Quiet:  quiet,
					JSON:   jsonOut,
				},
				BaseBranch: baseBranch,
			}, a, cfg, logger, notifier, &platform.OTelPolicyMetrics{})
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

	return cmd
}
