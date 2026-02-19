package amadeus

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAmadeus_ShouldFullCheck_Diff(t *testing.T) {
	a := &Amadeus{Config: DefaultConfig(), CheckCount: 3}
	if a.ShouldFullCheck(false) {
		t.Error("expected diff check when count < interval")
	}
}

func TestAmadeus_ShouldFullCheck_FullByInterval(t *testing.T) {
	a := &Amadeus{Config: DefaultConfig(), CheckCount: 10}
	if !a.ShouldFullCheck(false) {
		t.Error("expected full check when count >= interval")
	}
}

func TestAmadeus_ShouldFullCheck_FullByFlag(t *testing.T) {
	a := &Amadeus{Config: DefaultConfig()}
	if !a.ShouldFullCheck(true) {
		t.Error("expected full check when --full flag is set")
	}
}

func TestAmadeus_ShouldFullCheck_FullByForceFullNext(t *testing.T) {
	// given: ForceFullNext is set in persisted state, count is low
	a := &Amadeus{Config: DefaultConfig(), CheckCount: 1, ForceFullNext: true}

	// when/then: should trigger full check even though count < interval and no flag
	if !a.ShouldFullCheck(false) {
		t.Error("expected full check when ForceFullNext is set")
	}
}

func TestAmadeus_DivergenceJump_SetsForceFullNext(t *testing.T) {
	// given: a state store with previous divergence data
	dir := t.TempDir()
	root := dir + "/.divergence"
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	prev := CheckResult{
		Commit:     "abc123",
		Divergence: 0.10,
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: a divergence jump is detected and flagged
	a := &Amadeus{Config: DefaultConfig(), Store: store}
	a.FlagForceFullNext()
	if err := a.SaveCheckState("def456", prev, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// then: persisted state should have ForceFullNext=true
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ForceFullNext {
		t.Error("expected ForceFullNext to be true in persisted state")
	}
}

func TestAmadeus_NoShift_AdvancesCheckCount(t *testing.T) {
	// given: a state store with a previous result at count=3
	dir := t.TempDir()
	root := dir + "/.divergence"
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	prev := CheckResult{
		Commit:              "abc123",
		CheckCountSinceFull: 3,
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: AdvanceCheckCount is called (simulating a no-shift early return)
	a := &Amadeus{Config: DefaultConfig(), Store: store}
	a.CheckCount = prev.CheckCountSinceFull
	a.AdvanceCheckCount(false)
	if err := a.SaveCheckState(prev.Commit, prev, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// then: persisted count should be 4
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CheckCountSinceFull != 4 {
		t.Errorf("expected check count 4, got %d", loaded.CheckCountSinceFull)
	}
}

func TestAmadeus_NoShift_PreservesPriorDivergence(t *testing.T) {
	// given: a state store with previous divergence data
	dir := t.TempDir()
	root := dir + "/.divergence"
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	prev := CheckResult{
		Commit:     "abc123",
		Divergence: 0.145,
		Axes: map[Axis]AxisScore{
			AxisADR: {Score: 15, Details: "ADR-003 minor tension"},
		},
		CheckCountSinceFull: 3,
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: SaveCheckState is called (simulating no-shift early return)
	a := &Amadeus{Config: DefaultConfig(), Store: store}
	a.CheckCount = prev.CheckCountSinceFull
	a.AdvanceCheckCount(false)
	if err := a.SaveCheckState(prev.Commit, prev, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// then: prior divergence and axes must be preserved
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Divergence != 0.145 {
		t.Errorf("expected divergence 0.145, got %f", loaded.Divergence)
	}
	if _, ok := loaded.Axes[AxisADR]; !ok {
		t.Error("expected ADR axis to be preserved")
	}
	if loaded.CheckCountSinceFull != 4 {
		t.Errorf("expected check count 4, got %d", loaded.CheckCountSinceFull)
	}
}

func TestAmadeus_NoShift_UpdatesCheckedAtAndHistory(t *testing.T) {
	// given: a state store with a previous result from yesterday
	dir := t.TempDir()
	root := dir + "/.divergence"
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	yesterday := time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC)
	prev := CheckResult{
		CheckedAt:           yesterday,
		Commit:              "abc123",
		Divergence:          0.10,
		CheckCountSinceFull: 2,
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: SaveCheckState is called with a new timestamp
	now := time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC)
	a := &Amadeus{Config: DefaultConfig(), Store: store}
	a.CheckCount = prev.CheckCountSinceFull
	a.AdvanceCheckCount(false)
	if err := a.SaveCheckState("def456", prev, now); err != nil {
		t.Fatal(err)
	}

	// then: CheckedAt should be updated to now
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.CheckedAt.Equal(now) {
		t.Errorf("expected CheckedAt %v, got %v", now, loaded.CheckedAt)
	}

	// then: history should have a new entry
	entries, err := os.ReadDir(filepath.Join(root, "history"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(entries))
	}
}

func TestAmadeus_NoShift_ClearsStalePRAndDMailData(t *testing.T) {
	// given: a previous result with PRs and DMails from a real check
	dir := t.TempDir()
	root := dir + "/.divergence"
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	prev := CheckResult{
		Commit:       "abc123",
		Type:         CheckTypeFull,
		Divergence:   0.10,
		PRsEvaluated: []string{"#120", "#122"},
		DMails:       []string{"d-001"},
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: SaveCheckState is called (no-shift early return)
	a := &Amadeus{Config: DefaultConfig(), Store: store}
	a.AdvanceCheckCount(false)
	now := time.Date(2026, 2, 20, 14, 0, 0, 0, time.UTC)
	if err := a.SaveCheckState("def456", prev, now); err != nil {
		t.Fatal(err)
	}

	// then: PRsEvaluated and DMails should be cleared, Type should be diff
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.PRsEvaluated) != 0 {
		t.Errorf("expected empty PRsEvaluated, got %v", loaded.PRsEvaluated)
	}
	if len(loaded.DMails) != 0 {
		t.Errorf("expected empty DMails, got %v", loaded.DMails)
	}
	if loaded.Type != CheckTypeDiff {
		t.Errorf("expected Type %q, got %q", CheckTypeDiff, loaded.Type)
	}
	// divergence should still be preserved
	if loaded.Divergence != 0.10 {
		t.Errorf("expected divergence 0.10, got %f", loaded.Divergence)
	}
}

func TestAmadeus_ShouldPromoteToFull_LargeJump(t *testing.T) {
	// given: default config has OnDivergenceJump = 0.15
	a := &Amadeus{Config: DefaultConfig()}

	// when: divergence jumps from 0.10 to 0.30 (delta = 0.20, exceeds 0.15)
	if !a.ShouldPromoteToFull(0.10, 0.30) {
		t.Error("expected promotion to full check on large divergence jump")
	}
}

func TestAmadeus_ShouldPromoteToFull_SmallJump(t *testing.T) {
	// given: default config has OnDivergenceJump = 0.15
	a := &Amadeus{Config: DefaultConfig()}

	// when: divergence jumps from 0.10 to 0.20 (delta = 0.10, below 0.15)
	if a.ShouldPromoteToFull(0.10, 0.20) {
		t.Error("expected no promotion on small divergence jump")
	}
}

func TestAmadeus_ShouldPromoteToFull_ExactThreshold(t *testing.T) {
	// given: default config has OnDivergenceJump = 0.15
	a := &Amadeus{Config: DefaultConfig()}

	// when: divergence jumps exactly by threshold (delta = 0.15)
	if !a.ShouldPromoteToFull(0.10, 0.25) {
		t.Error("expected promotion at exact threshold boundary")
	}
}

func TestResolveDMail_Approve(t *testing.T) {
	// given: a pending HIGH D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	dmail := DMail{
		ID:       "d-001",
		Severity: SeverityHigh,
		Status:   DMailPending,
		Target:   TargetSightjack,
		Summary:  "ADR violation",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "approve", "")

	// then
	if err != nil {
		t.Fatalf("ResolveDMail failed: %v", err)
	}
	loaded, _ := store.LoadDMail("d-001")
	if loaded.Status != DMailApproved {
		t.Errorf("expected status approved, got %s", loaded.Status)
	}
	if loaded.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
	if loaded.ResolvedAction == nil || *loaded.ResolvedAction != "approve" {
		t.Errorf("expected ResolvedAction approve, got %v", loaded.ResolvedAction)
	}
}

func TestResolveDMail_Reject(t *testing.T) {
	// given: a pending HIGH D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	dmail := DMail{
		ID:       "d-001",
		Severity: SeverityHigh,
		Status:   DMailPending,
		Target:   TargetSightjack,
		Summary:  "ADR violation",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "reject", "false positive")

	// then
	if err != nil {
		t.Fatalf("ResolveDMail failed: %v", err)
	}
	loaded, _ := store.LoadDMail("d-001")
	if loaded.Status != DMailRejected {
		t.Errorf("expected status rejected, got %s", loaded.Status)
	}
	if loaded.RejectReason == nil || *loaded.RejectReason != "false positive" {
		t.Errorf("expected reject reason 'false positive', got %v", loaded.RejectReason)
	}
}

func TestResolveDMail_AlreadyResolved(t *testing.T) {
	// given: an already approved D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	now := time.Now().UTC()
	action := "approve"
	dmail := DMail{
		ID:             "d-001",
		Severity:       SeverityHigh,
		Status:         DMailApproved,
		ResolvedAt:     &now,
		ResolvedAction: &action,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "reject", "oops")

	// then: should error
	if err == nil {
		t.Error("expected error when resolving already-resolved D-Mail")
	}
}

func TestResolveDMail_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-999", "approve", "")

	// then
	if err == nil {
		t.Error("expected error for non-existent D-Mail")
	}
}

func TestResolveDMail_RejectEmptyReason(t *testing.T) {
	// given: a pending HIGH D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	dmail := DMail{
		ID:       "d-001",
		Severity: SeverityHigh,
		Status:   DMailPending,
		Target:   TargetSightjack,
		Summary:  "ADR violation",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "reject", "")

	// then
	if err == nil {
		t.Fatal("expected error for reject with empty reason")
	}
	if !strings.Contains(err.Error(), "reason") {
		t.Errorf("expected error to mention 'reason', got: %v", err)
	}
	// D-Mail should remain pending (not persisted)
	loaded, _ := store.LoadDMail("d-001")
	if loaded.Status != DMailPending {
		t.Errorf("expected status to remain pending, got %s", loaded.Status)
	}
}

func TestAmadeus_PrintCheckOutput(t *testing.T) {
	var buf bytes.Buffer
	a := &Amadeus{
		Config: DefaultConfig(),
		Logger: NewLogger(&buf, false),
	}
	result := CheckResult{
		Divergence: 0.145,
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 15, Details: "ADR-003 minor tension"},
			AxisDoD:        {Score: 20, Details: "Issue #42 edge case"},
			AxisDependency: {Score: 10, Details: "clean"},
			AxisImplicit:   {Score: 5, Details: "naming drift"},
		},
		PRsEvaluated: []string{"#120", "#122"},
	}
	dmails := []DMail{
		{ID: "d-001", Severity: SeverityLow, Status: DMailSent, Target: TargetPaintress, Summary: "naming issue"},
	}
	a.PrintCheckOutput(result, dmails, 0.133)
	output := buf.String()
	if !strings.Contains(output, "Divergence") {
		t.Errorf("expected 'Divergence' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "d-001") {
		t.Errorf("expected 'd-001' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ADR Integrity") {
		t.Errorf("expected 'ADR Integrity' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintLog(t *testing.T) {
	// given: history and D-Mails
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	r1 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		Commit:     "aaa",
		Type:       CheckTypeFull,
		Divergence: 0.10,
	}
	r2 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Commit:     "bbb",
		Type:       CheckTypeDiff,
		Divergence: 0.15,
		DMails:     []string{"d-001"},
	}
	for _, r := range []CheckResult{r1, r2} {
		if err := store.SaveHistory(r); err != nil {
			t.Fatal(err)
		}
	}

	dmail := DMail{
		ID:       "d-001",
		Severity: SeverityHigh,
		Status:   DMailPending,
		Target:   TargetSightjack,
		Summary:  "ADR-003 violation",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.PrintLog()

	// then
	if err != nil {
		t.Fatalf("PrintLog failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "History:") {
		t.Errorf("expected 'History:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "bbb") {
		t.Errorf("expected commit 'bbb' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "d-001") {
		t.Errorf("expected 'd-001' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintCheckOutput_Quiet(t *testing.T) {
	var buf bytes.Buffer
	a := &Amadeus{
		Config: DefaultConfig(),
		Logger: NewLogger(&buf, false),
	}
	result := CheckResult{
		Divergence: 0.145,
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 15, Details: "ADR-003"},
			AxisDoD:        {Score: 20, Details: "edge case"},
			AxisDependency: {Score: 10, Details: "clean"},
			AxisImplicit:   {Score: 5, Details: "naming"},
		},
	}
	dmails := []DMail{
		{ID: "d-001", Severity: SeverityLow, Status: DMailSent},
		{ID: "d-002", Severity: SeverityHigh, Status: DMailPending},
	}

	// when: quiet mode
	a.PrintCheckOutputQuiet(result, dmails, 0.133)

	// then: single line with divergence, delta, dmail count, pending count
	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line in quiet mode, got %d:\n%s", len(lines), output)
	}
	if !strings.Contains(output, "0.145000") {
		t.Errorf("expected divergence value in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2 D-Mails") {
		t.Errorf("expected '2 D-Mails' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "1 pending") {
		t.Errorf("expected '1 pending' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintLog_Empty(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.PrintLog()

	// then
	if err != nil {
		t.Fatalf("PrintLog failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No history") {
		t.Errorf("expected 'No history' in output, got:\n%s", output)
	}
}
