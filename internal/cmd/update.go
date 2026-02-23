package cmd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

const repoSlug = "hironow/amadeus"

func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update amadeus to the latest version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			checkOnly, _ := cmd.Flags().GetBool("check")
			ctx := cmd.Context()

			source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
			if err != nil {
				return fmt.Errorf("create github source: %w", err)
			}

			updater, err := selfupdate.NewUpdater(selfupdate.Config{
				Source:    source,
				Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
			})
			if err != nil {
				return fmt.Errorf("create updater: %w", err)
			}

			latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
			if err != nil {
				return fmt.Errorf("detect latest version: %w", err)
			}
			if !found {
				fmt.Fprintln(cmd.ErrOrStderr(), "no release found")
				return nil
			}

			if isUpToDate(Version, latest.Version()) {
				fmt.Fprintf(cmd.ErrOrStderr(), "already up to date (%s)\n", Version)
				return nil
			}

			if checkOnly {
				fmt.Fprintf(cmd.ErrOrStderr(), "new version available: %s (current: %s)\n",
					latest.Version(), Version)
				return nil
			}

			exe, err := selfupdate.ExecutablePath()
			if err != nil {
				return fmt.Errorf("get executable path: %w", err)
			}

			if err := updater.UpdateTo(ctx, latest, exe); err != nil {
				return fmt.Errorf("update binary: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "updated to %s\n", latest.Version())
			return nil
		},
	}

	cmd.Flags().BoolP("check", "C", false, "check for updates without installing")

	return cmd
}

// isUpToDate returns true if current version is >= latest version.
// Non-semver versions (e.g. "dev") are always considered out of date.
func isUpToDate(current, latest string) bool {
	cv, err := semver.NewVersion(current)
	if err != nil {
		return false
	}
	lv, err := semver.NewVersion(latest)
	if err != nil {
		return false
	}
	return !cv.LessThan(lv)
}
