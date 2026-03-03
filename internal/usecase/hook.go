package usecase

import "github.com/hironow/amadeus/internal/session"

// InstallHook installs the amadeus post-merge git hook.
func InstallHook(gitDir string) error {
	return session.InstallHook(gitDir)
}

// UninstallHook removes the amadeus post-merge git hook.
func UninstallHook(gitDir string) error {
	return session.UninstallHook(gitDir)
}
