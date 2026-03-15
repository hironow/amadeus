package testdata

import (
	"os"
	"os/signal"

	"github.com/blang/semver/v4"
)

// ==========================================================================
// go-safety.yaml test fixture
// Covers: signal-notify-without-stop, semver-must-parse-panic,
//         prefer-errors-is-for-not-exist, devtty-hard-fail-needs-fallback,
//         raw-nanosecond-duration
// ==========================================================================

func badSignalNotifySafety() {
	ch := make(chan os.Signal, 1)
	// ruleid: signal-notify-without-stop
	signal.Notify(ch, os.Interrupt)
}

func goodSignalNotifySafety() {
	ch := make(chan os.Signal, 1)
	// ok: signal-notify-without-stop
	signal.Notify(ch, os.Interrupt)
	defer signal.Stop(ch)
}

func badSemverMustParseSafety() {
	// ruleid: semver-must-parse-panic
	_ = semver.MustParse("1.0.0")
}

func badDevTTYSafety() error {
	// ruleid: devtty-hard-fail-needs-fallback
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return err
	}
	_ = tty
	return nil
}

func badIsNotExistSafety() {
	// ruleid: prefer-errors-is-for-not-exist
	_ = os.IsNotExist(nil)
}
