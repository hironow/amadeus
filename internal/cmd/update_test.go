package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"testing"
)

func TestUpdate_CheckFlagExists(t *testing.T) {
	// given
	root := NewRootCommand()

	// then
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "update" {
			found = true
			if f := sub.Flags().Lookup("check"); f == nil {
				t.Error("expected --check flag on update command")
			}
		}
	}
	if !found {
		t.Fatal("expected update subcommand to exist")
	}
}

func TestUpdate_AlreadyLatest(t *testing.T) {
	// given — version comparison logic: if current >= latest, "already up to date"
	// This tests the semver comparison helper, not the HTTP call.
	cases := []struct {
		name     string
		current  string
		latest   string
		upToDate bool
	}{
		{name: "same version", current: "1.0.0", latest: "1.0.0", upToDate: true},
		{name: "current newer", current: "2.0.0", latest: "1.0.0", upToDate: true},
		{name: "current older", current: "1.0.0", latest: "2.0.0", upToDate: false},
		{name: "dev version", current: "dev", latest: "1.0.0", upToDate: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := isUpToDate(tc.current, tc.latest)

			// then
			if got != tc.upToDate {
				t.Errorf("isUpToDate(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.upToDate)
			}
		})
	}
}
