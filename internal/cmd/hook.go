package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/session"
	"github.com/spf13/cobra"
)

func newInstallHookCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install-hook",
		Short: "Install post-merge git hook",
		Long: `Install a post-merge git hook that triggers amadeus after git pull/merge.

The hook is installed into the current repository's .git/hooks/ directory.
It runs 'amadeus run' automatically after each merge, enabling continuous
divergence monitoring without manual intervention. The command must be
run from within a git repository.`,
		Example: `  # Install hook in current git repository
  amadeus install-hook

  # Verify the hook was installed
  cat .git/hooks/post-merge`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			gitDir, err := findGitDir()
			if err != nil {
				return err
			}
			if err := session.InstallHook(gitDir); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  Installed post-merge hook in %s\n", filepath.Join(gitDir, "hooks", "post-merge"))
			return nil
		},
	}
}

func newUninstallHookCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-hook",
		Short: "Remove post-merge git hook",
		Long: `Remove the amadeus post-merge git hook from the current repository.

Only removes the hook if it was installed by amadeus. The command must
be run from within a git repository.`,
		Example: `  # Remove the post-merge hook
  amadeus uninstall-hook`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			gitDir, err := findGitDir()
			if err != nil {
				return err
			}
			if err := session.UninstallHook(gitDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "  Removed amadeus post-merge hook")
			return nil
		},
	}
}

func findGitDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		repoRoot, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		gitDir = filepath.Join(repoRoot, gitDir)
	}
	return gitDir, nil
}
