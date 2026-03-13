//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorRepair_StalePID(t *testing.T) {
	// given: initialized repo with a stale PID file
	dir := initTestRepo(t)
	pidPath := filepath.Join(dir, ".gate", "watch.pid")
	// PID 99999 is almost certainly not running
	if err := os.WriteFile(pidPath, []byte("99999"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when: run doctor --repair --json
	stdout, _, _ := runCmd(t, dir, "doctor", "--repair", "--json")

	// then: PID file should be removed
	if _, err := os.Stat(pidPath); err == nil {
		t.Error("expected stale PID file to be removed after --repair")
	}

	// then: output should contain stale-pid fix
	var result struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, stdout)
	}
	found := false
	for _, c := range result.Checks {
		if c.Name == "stale-pid" && c.Status == "FIX" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale-pid check with FIX status in output, got: %s", stdout)
	}
}

func TestDoctorRepair_MissingSkillMD(t *testing.T) {
	// given: initialized repo with SKILL.md deleted
	dir := initTestRepo(t)
	skillPath := filepath.Join(dir, ".gate", "skills", "dmail-sendable", "SKILL.md")
	if err := os.Remove(skillPath); err != nil {
		t.Fatal(err)
	}

	// when: run doctor --repair --json
	stdout, _, _ := runCmd(t, dir, "doctor", "--repair", "--json")

	// then: SKILL.md should be regenerated
	if _, err := os.Stat(skillPath); err != nil {
		t.Error("expected SKILL.md to be regenerated after --repair")
	}

	// then: output should contain SKILL.md fix
	var result struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, stdout)
	}
	found := false
	for _, c := range result.Checks {
		if c.Name == "SKILL.md" && c.Status == "FIX" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SKILL.md check with FIX status in output, got: %s", stdout)
	}
}

func TestDoctorRepair_NoRepairFlag(t *testing.T) {
	// given: initialized repo with a stale PID file
	dir := initTestRepo(t)
	pidPath := filepath.Join(dir, ".gate", "watch.pid")
	if err := os.WriteFile(pidPath, []byte("99999"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when: run doctor --json (WITHOUT --repair)
	stdout, _, _ := runCmd(t, dir, "doctor", "--json")

	// then: PID file should NOT be removed
	if _, err := os.Stat(pidPath); err != nil {
		t.Error("PID file should NOT be removed without --repair flag")
	}

	// then: output should NOT contain stale-pid fix
	if strings.Contains(stdout, "stale-pid") {
		t.Errorf("expected no stale-pid check without --repair, got: %s", stdout)
	}
}
