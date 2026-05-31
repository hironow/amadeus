//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestDoctorRepair_StalePID(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_doctor_repair_stale"
	initTestRepo(t, ctx, c, dir)
	pidPath := fmt.Sprintf("%s/.gate/watch.pid", dir)

	// Create stale PID file inside container
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("echo '99999' > %s", pidPath)})

	// when: run doctor --repair --json
	stdout, _, _ := runCmd(t, ctx, c, dir, "doctor", "--repair", "--json")

	// then: PID file should be removed
	if fileExistsInContainer(t, ctx, c, pidPath) {
		t.Error("expected stale PID file to be removed after --repair")
	}

	// then: output should contain stale-pid fix
	var result struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	parseJSONOutput(t, stdout, &result)
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
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_doctor_repair_skill"
	initTestRepo(t, ctx, c, dir)
	skillPath := fmt.Sprintf("%s/.gate/skills/dmail-sendable/SKILL.md", dir)

	execInContainer(t, ctx, c, []string{"rm", "-f", skillPath})

	// when: run doctor --repair --json
	stdout, _, _ := runCmd(t, ctx, c, dir, "doctor", "--repair", "--json")

	// then: SKILL.md should be regenerated
	if !fileExistsInContainer(t, ctx, c, skillPath) {
		t.Error("expected SKILL.md to be regenerated after --repair")
	}

	// then: output should contain SKILL.md fix
	var result struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	parseJSONOutput(t, stdout, &result)
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
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_doctor_repair_norepair"
	initTestRepo(t, ctx, c, dir)
	pidPath := fmt.Sprintf("%s/.gate/watch.pid", dir)

	// Create stale PID file inside container
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("echo '99999' > %s", pidPath)})

	// when: run doctor --json (WITHOUT --repair)
	stdout, _, _ := runCmd(t, ctx, c, dir, "doctor", "--json")

	// then: PID file should NOT be removed
	if !fileExistsInContainer(t, ctx, c, pidPath) {
		t.Error("PID file should NOT be removed without --repair flag")
	}

	// then: output should NOT contain stale-pid fix
	if strings.Contains(stdout, "stale-pid") {
		t.Errorf("expected no stale-pid check without --repair, got: %s", stdout)
	}
}
