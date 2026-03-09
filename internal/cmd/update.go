package cmd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Self-update amadeus to the latest release",
		Long: `Self-update amadeus to the latest GitHub release.

Downloads the latest release, verifies the checksum, and replaces
the current binary. Use --check to only check for updates without
installing.`,
		Example: `  # Check for updates
  amadeus update --check

  # Update to the latest version
  amadeus update`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			updater, err := selfupdate.NewUpdater(selfupdate.Config{
				Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
			})
			if err != nil {
				return fmt.Errorf("failed to create updater: %w", err)
			}

			latest, found, err := updater.DetectLatest(cmd.Context(), selfupdate.ParseSlug("hironow/amadeus"))
			if err != nil {
				return fmt.Errorf("failed to detect latest version: %w", err)
			}
			if !found {
				fmt.Fprintln(cmd.ErrOrStderr(), "No release found.")
				return nil
			}

			if isUpToDate(Version, latest.Version()) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Already up to date (v%s).\n", Version)
				return nil
			}

			if checkOnly {
				fmt.Fprintf(cmd.ErrOrStderr(), "Update available: v%s → v%s\n", Version, latest.Version())
				return nil
			}

			exe, err := selfupdate.ExecutablePath()
			if err != nil {
				return fmt.Errorf("failed to locate executable: %w", err)
			}

			if err := updater.UpdateTo(cmd.Context(), latest, exe); err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Updated to v%s\n", latest.Version())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&checkOnly, "check", "C", false, "Check for updates without installing")

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
