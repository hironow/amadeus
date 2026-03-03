package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/amadeus/internal/domain"
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
			if preErr := usecase.PreflightCheck(bins...); preErr != nil {
				return preErr
			}

			if err := usecase.InitGate(divRoot); err != nil {
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

			logger := loggerFrom(cmd)

			// Wire approver
			autoApprove, _ := cmd.Flags().GetBool("auto-approve")
			approveCmd, _ := cmd.Flags().GetString("approve-cmd")

			var approver domain.Approver
			switch {
			case autoApprove:
				approver = &domain.AutoApprover{}
			case approveCmd != "":
				approver = usecase.NewCmdApprover(approveCmd)
			default:
				approver = &domain.AutoApprover{} // default: no gate
			}

			// Wire notifier
			notifyCmd, _ := cmd.Flags().GetString("notify-cmd")
			var notifier domain.Notifier
			if notifyCmd != "" {
				notifier = usecase.NewCmdNotifier(notifyCmd)
			} else {
				notifier = &domain.NopNotifier{}
			}

			reviewCmd, _ := cmd.Flags().GetString("review-cmd")

			// COMMAND → usecase → Aggregate → EVENT
			return usecase.RunCheckFromParams(cmd.Context(), domain.ExecuteCheckCommand{
				RepoPath: repoRoot,
			}, domain.CheckOptions{
				Full:   full,
				DryRun: dryRun,
				Quiet:  quiet,
				JSON:   jsonOut,
			}, usecase.AmadeusParams{
				GateDir:   divRoot,
				RepoDir:   repoRoot,
				Config:    cfg,
				Logger:    logger,
				DataOut:   cmd.OutOrStdout(),
				Approver:  approver,
				Notifier:  notifier,
				ReviewCmd: reviewCmd,
				WithGit:   true,
			})
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
