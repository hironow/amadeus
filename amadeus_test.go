package amadeus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	prev := CheckResult{
		Commit:     "abc123",
		Divergence: 0.10,
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: a divergence jump is detected and flagged
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(io.Discard, false)}
	if err := a.FlagForceFullNext(0.10, 0.35); err != nil {
		t.Fatal(err)
	}
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
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	prev := CheckResult{
		Commit:              "abc123",
		CheckCountSinceFull: 3,
	}
	if err := store.SaveLatest(prev); err != nil {
		t.Fatal(err)
	}

	// when: AdvanceCheckCount is called (simulating a no-shift early return)
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}}
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
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
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
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}}
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

func TestAmadeus_NoShift_UpdatesCheckedAtAndEmitsEvent(t *testing.T) {
	// given: a state store with a previous result from yesterday
	dir := t.TempDir()
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	eventsDir := filepath.Join(root, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventStore := &FileEventStore{Dir: eventsDir}
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
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    eventStore,
		Projector: &Projector{Store: store},
	}
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

	// then: event store should have a CheckCompleted event
	events, err := eventStore.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventCheckCompleted {
		t.Errorf("event type = %q, want %q", events[0].Type, EventCheckCompleted)
	}
}

func TestAmadeus_NoShift_ClearsStalePRAndDMailData(t *testing.T) {
	// given: a previous result with PRs and DMails from a real check
	dir := t.TempDir()
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
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
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}}
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

func TestAmadeus_PrintCheckOutput(t *testing.T) {
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
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
		{Name: "feedback-001", Kind: KindFeedback, Description: "naming issue", Severity: SeverityLow},
	}
	a.PrintCheckOutput(result, dmails, 0.133)

	// result display should go to DataOut, not Logger
	if logBuf.Len() != 0 {
		t.Errorf("expected no stderr output, got: %s", logBuf.String())
	}
	output := dataBuf.String()
	if !strings.Contains(output, "Divergence") {
		t.Errorf("expected 'Divergence' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "feedback-001") {
		t.Errorf("expected 'feedback-001' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ADR Integrity") {
		t.Errorf("expected 'ADR Integrity' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintCheckOutput_IncludesImpactRadius(t *testing.T) {
	// given
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
	}
	result := CheckResult{
		Divergence: 0.10,
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 10, Details: "ok"},
			AxisDoD:        {Score: 0, Details: "ok"},
			AxisDependency: {Score: 0, Details: "ok"},
			AxisImplicit:   {Score: 0, Details: "ok"},
		},
		ImpactRadius: []ImpactEntry{
			{Area: "auth/session.go", Impact: "direct", Detail: "Session validation changed"},
			{Area: "api/handler.go", Impact: "indirect", Detail: "Uses auth session"},
		},
	}

	// when
	a.PrintCheckOutput(result, nil, 0.08)

	// then
	output := dataBuf.String()
	if !strings.Contains(output, "Impact Radius") {
		t.Errorf("expected 'Impact Radius' section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "auth/session.go") {
		t.Errorf("expected 'auth/session.go' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "direct") {
		t.Errorf("expected 'direct' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintCheckOutput_NoImpactRadius(t *testing.T) {
	// given: no impact radius data
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
	}
	result := CheckResult{
		Divergence: 0.10,
		Axes: map[Axis]AxisScore{
			AxisADR: {Score: 10, Details: "ok"},
		},
	}

	// when
	a.PrintCheckOutput(result, nil, 0.08)

	// then: should not show Impact Radius section
	output := dataBuf.String()
	if strings.Contains(output, "Impact Radius") {
		t.Errorf("expected no 'Impact Radius' section when empty, got:\n%s", output)
	}
}

func TestAmadeus_PrintLog(t *testing.T) {
	// given: history and D-Mails
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

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
	eventsDir := filepath.Join(root, "events")
	eventStore := &FileEventStore{Dir: eventsDir}
	for _, r := range []CheckResult{r1, r2} {
		ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{Result: r}, r.CheckedAt)
		if err != nil {
			t.Fatal(err)
		}
		if err := eventStore.Append(ev); err != nil {
			t.Fatal(err)
		}
	}

	dmail := DMail{
		Name:        "feedback-001",
		Kind:        KindFeedback,
		Description: "ADR-003 violation",
		Severity:    SeverityHigh,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Events: eventStore, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// when
	err := a.PrintLog()

	// then
	if err != nil {
		t.Fatalf("PrintLog failed: %v", err)
	}
	// result display should go to DataOut, not Logger
	if logBuf.Len() != 0 {
		t.Errorf("expected no stderr output, got: %s", logBuf.String())
	}
	output := dataBuf.String()
	if !strings.Contains(output, "History:") {
		t.Errorf("expected 'History:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "bbb") {
		t.Errorf("expected commit 'bbb' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "feedback-001") {
		t.Errorf("expected 'feedback-001' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "sent") {
		t.Errorf("expected 'sent' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintCheckOutput_Quiet(t *testing.T) {
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
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
		{Name: "feedback-001", Kind: KindFeedback, Description: "low issue", Severity: SeverityLow},
		{Name: "feedback-002", Kind: KindFeedback, Description: "high issue", Severity: SeverityHigh},
	}

	// when: quiet mode
	a.PrintCheckOutputQuiet(result, dmails, 0.133)

	// then: result display should go to DataOut, not Logger
	if logBuf.Len() != 0 {
		t.Errorf("expected no stderr output, got: %s", logBuf.String())
	}
	output := strings.TrimSpace(dataBuf.String())
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
}

func TestAmadeus_PrintLog_Empty(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// when
	err := a.PrintLog()

	// then
	if err != nil {
		t.Fatalf("PrintLog failed: %v", err)
	}
	if logBuf.Len() != 0 {
		t.Errorf("expected no stderr output, got: %s", logBuf.String())
	}
	output := dataBuf.String()
	if !strings.Contains(output, "No history") {
		t.Errorf("expected 'No history' in output, got:\n%s", output)
	}
}

func TestDriftError_IsDrift(t *testing.T) {
	// given: a DriftError
	err := &DriftError{Divergence: 0.35, DMails: 2}

	// then: it implements error and is detectable via errors.As
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatal("expected errors.As to match DriftError")
	}
	if de.Divergence != 0.35 {
		t.Errorf("expected divergence 0.35, got %f", de.Divergence)
	}
	if de.DMails != 2 {
		t.Errorf("expected 2 dmails, got %d", de.DMails)
	}
}

func TestDriftError_Message(t *testing.T) {
	err := &DriftError{Divergence: 0.35, DMails: 1}
	msg := err.Error()
	if !strings.Contains(msg, "drift") {
		t.Errorf("expected 'drift' in error message, got: %s", msg)
	}
}

func TestDriftError_NotConfusedWithRegularError(t *testing.T) {
	// given: a regular error
	err := fmt.Errorf("something broke")

	// then: errors.As should NOT match DriftError
	var de *DriftError
	if errors.As(err, &de) {
		t.Fatal("regular error should not match DriftError")
	}
}

func TestExitCode_Nil(t *testing.T) {
	if code := ExitCode(nil); code != 0 {
		t.Errorf("expected 0 for nil, got %d", code)
	}
}

func TestExitCode_DriftError(t *testing.T) {
	err := &DriftError{Divergence: 0.35, DMails: 2}
	if code := ExitCode(err); code != 2 {
		t.Errorf("expected 2 for DriftError, got %d", code)
	}
}

func TestExitCode_RegularError(t *testing.T) {
	err := fmt.Errorf("something broke")
	if code := ExitCode(err); code != 1 {
		t.Errorf("expected 1 for regular error, got %d", code)
	}
}

func TestExitCode_WrappedDriftError(t *testing.T) {
	// given: DriftError wrapped in fmt.Errorf
	inner := &DriftError{Divergence: 0.50, DMails: 3}
	err := fmt.Errorf("check failed: %w", inner)

	// then: should still detect drift via errors.As
	if code := ExitCode(err); code != 2 {
		t.Errorf("expected 2 for wrapped DriftError, got %d", code)
	}
}

func TestRunCheck_DryRun_NilDataOut_NoPanic(t *testing.T) {
	// given: an Amadeus with DataOut=nil (library usage without explicit wiring)
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	a := &Amadeus{
		Config:  DefaultConfig(),
		Store:   NewProjectionStore(divRoot),
		Git:     NewGitClient(repo.dir),
		Logger:  NewLogger(&bytes.Buffer{}, false),
		DataOut: nil, // intentionally nil
	}

	// when: DryRun should not panic
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})

	// then
	if err != nil {
		t.Fatalf("RunCheck DryRun with nil DataOut should not fail: %v", err)
	}
}

func TestRunCheck_DryRun_DoesNotConsumeInbox(t *testing.T) {
	// given: inbox has a report d-mail
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	inboxDir := filepath.Join(divRoot, "inbox")
	reportContent := "---\nname: report-001\nkind: report\ndescription: test\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(inboxDir, "report-001.md"), []byte(reportContent), 0o644); err != nil {
		t.Fatal(err)
	}

	a := &Amadeus{
		Config:  DefaultConfig(),
		Store:   NewProjectionStore(divRoot),
		Git:     NewGitClient(repo.dir),
		Logger:  NewLogger(&bytes.Buffer{}, false),
		DataOut: &bytes.Buffer{},
	}

	// when: dry-run check
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: inbox file should still exist (not consumed)
	if _, err := os.Stat(filepath.Join(inboxDir, "report-001.md")); err != nil {
		t.Errorf("inbox file should not be consumed in dry-run mode: %v", err)
	}

	// then: consumed.json should not exist
	if _, err := os.Stat(filepath.Join(divRoot, ".run", "consumed.json")); err == nil {
		t.Error("consumed.json should not be created in dry-run mode")
	}
}

func TestRunCheck_DryRun_FullCheck_IncludesADRsInPrompt(t *testing.T) {
	// given: a repo with docs/adr/0001-test.md
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	adrDir := filepath.Join(repo.dir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-test.md"), []byte("# 0001. Use Go for CLI\n\nGo is the best choice.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Store:   NewProjectionStore(divRoot),
		Git:     NewGitClient(repo.dir),
		Logger:  NewLogger(&bytes.Buffer{}, false),
		DataOut: &dataBuf,
	}

	// when: full dry-run check
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := dataBuf.String()
	if !strings.Contains(prompt, "Use Go for CLI") {
		t.Errorf("expected ADR content in prompt, got:\n%s", prompt)
	}
}

func TestRunCheck_DryRun_FullCheck_NoADRs_StillWorks(t *testing.T) {
	// given: a repo with NO docs/adr/ directory
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}

	var dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Store:   NewProjectionStore(divRoot),
		Git:     NewGitClient(repo.dir),
		Logger:  NewLogger(&bytes.Buffer{}, false),
		DataOut: &dataBuf,
	}

	// when: full dry-run check
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})

	// then: no error, graceful degradation
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dataBuf.Len() == 0 {
		t.Error("expected prompt output, got empty")
	}
}

func TestRunCheck_DryRun_DiffCheck_IncludesADRsInPrompt(t *testing.T) {
	// given: a repo with ADRs and a previous check state (so diff mode triggers)
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	adrDir := filepath.Join(repo.dir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-auth.md"), []byte("# 0001. JWT Auth\n\nUse JWT.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewProjectionStore(divRoot)
	git := NewGitClient(repo.dir)
	// Save a previous state with current commit so diff mode detects the new PR merges
	commit, err := git.CurrentCommit()
	if err != nil {
		t.Fatal(err)
	}
	// Save a previous state with an older commit to trigger shift detection
	if err := store.SaveLatest(CheckResult{Commit: commit + "~2"}); err != nil {
		t.Fatal(err)
	}

	// Create a new commit so there's a diff
	repo.shell(t, "echo 'package main' > /repo/new.go")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "Merge pull request #99 from feature/new")

	var dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       git,
		Logger:    NewLogger(&bytes.Buffer{}, false),
		DataOut:   &dataBuf,
	}

	// when: diff dry-run check
	err = a.RunCheck(context.Background(), CheckOptions{DryRun: true, Quiet: true})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := dataBuf.String()
	if !strings.Contains(prompt, "JWT Auth") {
		t.Errorf("expected ADR content in diff prompt, got:\n%s", prompt)
	}
}

func TestRunCheck_DryRun_DiffCheck_IncludesIssueIDs(t *testing.T) {
	// given: a repo with a merge commit containing an issue ID
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}

	store := NewProjectionStore(divRoot)
	git := NewGitClient(repo.dir)
	commit, err := git.CurrentCommit()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLatest(CheckResult{Commit: commit + "~2"}); err != nil {
		t.Fatal(err)
	}

	// Create a merge commit with an issue ID in the message
	repo.shell(t, "echo 'package main' > /repo/feature.go")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "Merge pull request #10 from feat/dod-fetch (MY-303)")

	var dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       git,
		Logger:    NewLogger(&bytes.Buffer{}, false),
		DataOut:   &dataBuf,
	}

	// when: diff dry-run check
	err = a.RunCheck(context.Background(), CheckOptions{DryRun: true, Quiet: true})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := dataBuf.String()
	if !strings.Contains(prompt, "MY-303") {
		t.Errorf("expected issue ID MY-303 in diff prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Linear MCP") {
		t.Errorf("expected Linear MCP instruction in diff prompt, got:\n%s", prompt)
	}
}

func TestRunCheck_DryRun_DiffCheck_NoIssueIDs_OmitsSection(t *testing.T) {
	// given: a repo with a merge commit WITHOUT issue ID
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}

	store := NewProjectionStore(divRoot)
	git := NewGitClient(repo.dir)
	commit, err := git.CurrentCommit()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLatest(CheckResult{Commit: commit + "~2"}); err != nil {
		t.Fatal(err)
	}

	repo.shell(t, "echo 'package main' > /repo/plain.go")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "Merge pull request #11 from feat/no-issue-ref")

	var dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       git,
		Logger:    NewLogger(&bytes.Buffer{}, false),
		DataOut:   &dataBuf,
	}

	// when
	err = a.RunCheck(context.Background(), CheckOptions{DryRun: true, Quiet: true})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := dataBuf.String()
	if strings.Contains(prompt, "Linked Linear Issues") {
		t.Errorf("expected no Linked Linear Issues section when no IDs found, got:\n%s", prompt)
	}
}

func TestRunCheck_DryRun_DiffCheck_NoIssueIDs_SuppressesDoDs(t *testing.T) {
	// given: a repo with DoD files committed BEFORE the baseline,
	// then a merge commit WITHOUT issue IDs after the baseline.
	// DoDs should not appear in the prompt's LinkedDoDs section.
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}

	// Create DoD files before the baseline commit
	repo.shell(t, "mkdir -p /repo/docs/dod")
	repo.shell(t, "echo '# Sprint 42 DoD' > /repo/docs/dod/sprint-42.md")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "add DoD files")

	store := NewProjectionStore(divRoot)
	git := NewGitClient(repo.dir)

	// Set baseline to current commit (DoD files already exist)
	commit, err := git.CurrentCommit()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLatest(CheckResult{Commit: commit}); err != nil {
		t.Fatal(err)
	}

	// Add a merge commit without issue IDs after the baseline
	repo.shell(t, "echo 'package main' > /repo/plain.go")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-m", "Merge pull request #99 from feat/no-issue-ref")

	var dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       git,
		Logger:    NewLogger(&bytes.Buffer{}, false),
		DataOut:   &dataBuf,
	}

	// when
	err = a.RunCheck(context.Background(), CheckOptions{DryRun: true, Quiet: true})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := dataBuf.String()
	// DoD content should NOT appear in the LinkedDoDs section
	// (DoD files exist on disk but no issue IDs were extracted from PR titles)
	if strings.Contains(prompt, "Sprint 42 DoD") {
		t.Errorf("expected DoDs to be suppressed when no issue IDs found, but prompt contains DoD content")
	}
}

func TestPrintCheckOutput_JSON(t *testing.T) {
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
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
		{Name: "feedback-001", Kind: KindFeedback, Description: "naming issue", Severity: SeverityLow},
	}

	// when
	if err := a.PrintCheckOutputJSON(result, dmails, 0.133); err != nil {
		t.Fatalf("PrintCheckOutputJSON failed: %v", err)
	}

	// then: DataOut should be valid JSON
	if logBuf.Len() != 0 {
		t.Errorf("expected no stderr output, got: %s", logBuf.String())
	}
	output := dataBuf.Bytes()
	var parsed map[string]any
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}
	// should contain divergence
	if _, ok := parsed["divergence"]; !ok {
		t.Error("expected 'divergence' key in JSON output")
	}
	// should contain axes
	if _, ok := parsed["axes"]; !ok {
		t.Error("expected 'axes' key in JSON output")
	}
	// should contain dmails
	if _, ok := parsed["dmails"]; !ok {
		t.Error("expected 'dmails' key in JSON output")
	}
}

func TestPrintCheckOutputJSON_IncludesImpactRadius(t *testing.T) {
	// given
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
	}
	result := CheckResult{
		Divergence: 0.10,
		Axes: map[Axis]AxisScore{
			AxisADR: {Score: 10, Details: "ok"},
		},
		ImpactRadius: []ImpactEntry{
			{Area: "auth/session.go", Impact: "direct", Detail: "Session validation changed"},
			{Area: "api/handler.go", Impact: "indirect", Detail: "Uses auth session"},
		},
	}

	// when
	if err := a.PrintCheckOutputJSON(result, nil, 0.08); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then
	var parsed map[string]any
	if err := json.Unmarshal(dataBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, dataBuf.String())
	}
	ir, ok := parsed["impact_radius"]
	if !ok {
		t.Fatal("expected 'impact_radius' key in JSON output")
	}
	entries, ok := ir.([]any)
	if !ok {
		t.Fatalf("expected impact_radius to be array, got %T", ir)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestPrintCheckOutputJSON_EmptyImpactRadius(t *testing.T) {
	// given: no impact radius data
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:  DefaultConfig(),
		Logger:  NewLogger(&logBuf, false),
		DataOut: &dataBuf,
	}
	result := CheckResult{
		Divergence: 0.10,
		Axes: map[Axis]AxisScore{
			AxisADR: {Score: 10, Details: "ok"},
		},
	}

	// when
	if err := a.PrintCheckOutputJSON(result, nil, 0.08); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: impact_radius should be empty array (not null)
	var parsed map[string]any
	if err := json.Unmarshal(dataBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, dataBuf.String())
	}
	ir, ok := parsed["impact_radius"]
	if !ok {
		t.Fatal("expected 'impact_radius' key in JSON output")
	}
	entries, ok := ir.([]any)
	if !ok {
		t.Fatalf("expected impact_radius to be array, got %T", ir)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestRunCheck_ConsumesInbox(t *testing.T) {
	// given: set up .gate with an inbox report
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	// Drop a report into inbox/
	content := []byte("---\nname: report-001\nkind: report\ndescription: test report\n---\n\nReport body.\n")
	if err := os.WriteFile(filepath.Join(root, "inbox", "report-001.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewProjectionStore(root)

	// Verify inbox has file before scan
	inboxEntries, _ := os.ReadDir(filepath.Join(root, "inbox"))
	mdBefore := 0
	for _, e := range inboxEntries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdBefore++
		}
	}
	if mdBefore != 1 {
		t.Fatalf("expected 1 inbox file before, got %d", mdBefore)
	}

	// when: consume inbox directly (RunCheck requires Claude, so test ScanInbox + SaveConsumed)
	dmails, err := store.ScanInbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(dmails) != 1 {
		t.Fatalf("expected 1 consumed, got %d", len(dmails))
	}

	// Save consumed records (same logic as in RunCheck)
	now := time.Now().UTC()
	var records []ConsumedRecord
	for _, d := range dmails {
		records = append(records, ConsumedRecord{
			Name:       d.Name,
			Kind:       d.Kind,
			ConsumedAt: now,
			Source:     d.Name + ".md",
		})
	}
	if err := store.SaveConsumed(records); err != nil {
		t.Fatal(err)
	}

	// then: inbox is empty
	inboxEntries, _ = os.ReadDir(filepath.Join(root, "inbox"))
	mdAfter := 0
	for _, e := range inboxEntries {
		if strings.HasSuffix(e.Name(), ".md") {
			mdAfter++
		}
	}
	if mdAfter != 0 {
		t.Errorf("expected inbox empty after scan, got %d", mdAfter)
	}

	// then: archive has the file
	if _, err := os.Stat(filepath.Join(root, "archive", "report-001.md")); err != nil {
		t.Errorf("expected report in archive: %v", err)
	}

	// then: consumed records persisted
	loaded, err := store.LoadConsumed()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 consumed record, got %d", len(loaded))
	}
	if loaded[0].Name != "report-001" {
		t.Errorf("expected report-001, got %s", loaded[0].Name)
	}
}

// --- Full pipeline tests using fake Claude ---

func TestRunCheck_FullPipeline_CreatesDMails(t *testing.T) {
	// given: a git repo with fake claude returning a response with D-Mails
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(divRoot)

	cannedResponse := `{
		"axes": {
			"adr_integrity": {"score": 15, "details": "ADR-003 tension"},
			"dod_fulfillment": {"score": 20, "details": "Issue #42 gap"},
			"dependency_integrity": {"score": 10, "details": "auth->cart"},
			"implicit_constraints": {"score": 5, "details": "naming drift"}
		},
		"dmails": [
			{
				"description": "ADR-003 needs review",
				"detail": "Auth module drifted from ADR-003.",
				"issues": ["MY-100"],
				"targets": ["auth/session.go"]
			}
		],
		"reasoning": "Moderate divergence detected."
	}`
	eventStore := &FileEventStore{Dir: filepath.Join(divRoot, "events")}
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    eventStore,
		Projector: &Projector{Store: store},
		Git:       NewGitClient(repo.dir),
		Claude:    &fakeClaudeRunner{response: cannedResponse},
		Logger:    NewLogger(&logBuf, false),
		DataOut:   &dataBuf,
	}

	// when
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, Quiet: true})

	// then: RunCheck returns DriftError (divergence > 0)
	var driftErr *DriftError
	if !errors.As(err, &driftErr) {
		t.Fatalf("expected DriftError, got: %v", err)
	}
	if driftErr.DMails != 1 {
		t.Errorf("expected 1 D-Mail in DriftError, got %d", driftErr.DMails)
	}

	// then: D-Mail created in archive
	allDMails, err := store.LoadAllDMails()
	if err != nil {
		t.Fatalf("LoadAllDMails: %v", err)
	}
	feedbackDMails := filterDMailsByKind(allDMails, KindFeedback)
	if len(feedbackDMails) != 1 {
		t.Fatalf("expected 1 feedback D-Mail, got %d", len(feedbackDMails))
	}
	if feedbackDMails[0].Description != "ADR-003 needs review" {
		t.Errorf("expected description 'ADR-003 needs review', got %q", feedbackDMails[0].Description)
	}
	if len(feedbackDMails[0].Issues) != 1 || feedbackDMails[0].Issues[0] != "MY-100" {
		t.Errorf("expected issues [MY-100], got %v", feedbackDMails[0].Issues)
	}

	// then: check.completed event emitted
	events, err := eventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll events: %v", err)
	}
	checkEvents := 0
	for _, ev := range events {
		if ev.Type == EventCheckCompleted {
			checkEvents++
		}
	}
	if checkEvents < 1 {
		t.Fatalf("expected at least 1 check.completed event, got %d", checkEvents)
	}
}

func TestRunCheck_ConvergenceTriggered_CreatesDMail(t *testing.T) {
	// given: pre-existing feedback D-Mails all targeting the same file
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(divRoot)

	// Seed 6 feedback D-Mails targeting "auth/session.go" (>= threshold*2 = HIGH)
	now := time.Now().UTC()
	for i := 1; i <= 6; i++ {
		name := fmt.Sprintf("feedback-%03d", i)
		if err := store.SaveDMail(DMail{
			Name:        name,
			Kind:        KindFeedback,
			Description: fmt.Sprintf("issue %d", i),
			Severity:    SeverityHigh,
			Targets:     []string{"auth/session.go"},
			Metadata: map[string]string{
				"created_at": now.Add(-time.Duration(i) * 24 * time.Hour).Format(time.RFC3339),
			},
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Fake claude returns a clean response (no new D-Mails)
	cleanResponse := `{
		"axes": {
			"adr_integrity": {"score": 5, "details": "minor"},
			"dod_fulfillment": {"score": 0, "details": "ok"},
			"dependency_integrity": {"score": 0, "details": "ok"},
			"implicit_constraints": {"score": 0, "details": "ok"}
		},
		"dmails": [],
		"reasoning": "Clean check."
	}`
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       NewGitClient(repo.dir),
		Claude:    &fakeClaudeRunner{response: cleanResponse},
		Logger:    NewLogger(&logBuf, false),
		DataOut:   &dataBuf,
	}

	// when
	_ = a.RunCheck(context.Background(), CheckOptions{Full: true, Quiet: true})

	// then: convergence D-Mail should be created
	allDMails, err := store.LoadAllDMails()
	if err != nil {
		t.Fatalf("LoadAllDMails: %v", err)
	}
	convergenceDMails := filterDMailsByKind(allDMails, KindConvergence)
	if len(convergenceDMails) != 1 {
		t.Fatalf("expected 1 convergence D-Mail, got %d", len(convergenceDMails))
	}
	if convergenceDMails[0].Severity != SeverityHigh {
		t.Errorf("expected HIGH severity, got %s", convergenceDMails[0].Severity)
	}

	// then: convergence D-Mail is in outbox (MY-359: all severities go to outbox)
	outboxDir := filepath.Join(divRoot, "outbox")
	entries, err := os.ReadDir(outboxDir)
	if err != nil {
		t.Fatalf("read outbox dir: %v", err)
	}
	hasOutboxConvergence := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "convergence") {
			hasOutboxConvergence = true
			break
		}
	}
	if !hasOutboxConvergence {
		t.Error("expected convergence D-Mail in outbox/")
	}
}

func filterDMailsByKind(dmails []DMail, kind DMailKind) []DMail {
	var result []DMail
	for _, d := range dmails {
		if d.Kind == kind {
			result = append(result, d)
		}
	}
	return result
}

func TestPrintLogJSON(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	r1 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		Commit:     "aaa",
		Type:       CheckTypeFull,
		Divergence: 0.10,
	}
	eventStore := &FileEventStore{Dir: filepath.Join(root, "events")}
	ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{Result: r1}, r1.CheckedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := eventStore.Append(ev); err != nil {
		t.Fatal(err)
	}

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Events: eventStore, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// when
	err = a.PrintLogJSON()

	// then
	if err != nil {
		t.Fatalf("PrintLogJSON failed: %v", err)
	}
	if logBuf.Len() != 0 {
		t.Errorf("expected no stderr output, got: %s", logBuf.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(dataBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, dataBuf.String())
	}
	if _, ok := parsed["history"]; !ok {
		t.Error("expected 'history' key in JSON output")
	}
	if _, ok := parsed["dmails"]; !ok {
		t.Error("expected 'dmails' key in JSON output")
	}
}

func TestPrintLogJSON_AllDMailsShowSentStatus(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	// given: D-Mails of various severities
	for _, spec := range []struct {
		name     string
		severity Severity
	}{
		{"feedback-001", SeverityHigh},
		{"feedback-002", SeverityMedium},
		{"feedback-003", SeverityLow},
	} {
		if err := store.SaveDMail(DMail{
			Name: spec.name, Kind: KindFeedback,
			Description: spec.name + " dmail", Severity: spec.severity,
		}); err != nil {
			t.Fatal(err)
		}
	}

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// when
	if err := a.PrintLogJSON(); err != nil {
		t.Fatalf("PrintLogJSON failed: %v", err)
	}

	// then: all D-Mails have status "sent"
	var parsed struct {
		DMails []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"dmails"`
	}
	if err := json.Unmarshal(dataBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, dataBuf.String())
	}
	if len(parsed.DMails) != 3 {
		t.Fatalf("expected 3 dmails, got %d", len(parsed.DMails))
	}
	for _, d := range parsed.DMails {
		if d.Status != "sent" {
			t.Errorf("%s: expected status 'sent', got %q", d.Name, d.Status)
		}
	}
}

func TestSaveConvergenceDMails_ReturnsErrorOnFailure(t *testing.T) {
	// given: a store whose archive directory is not writable
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	alerts := []ConvergenceAlert{
		{Target: "auth/session.go", Count: 6, Window: 14, Severity: SeverityHigh,
			DMails: []string{"f-001", "f-002", "f-003", "f-004", "f-005", "f-006"}},
	}

	// break the archive directory so SaveDMail fails
	archiveDir := filepath.Join(root, "archive")
	os.RemoveAll(archiveDir)
	os.WriteFile(archiveDir, []byte("not a dir"), 0o444) // block as a file

	// when
	_, err := a.saveConvergenceDMails(alerts)

	// then: error should be surfaced, not swallowed
	if err == nil {
		t.Fatal("expected error when archive is broken, got nil")
	}
}

func TestSaveConvergenceDMails_Success(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	alerts := []ConvergenceAlert{
		{Target: "auth/session.go", Count: 6, Window: 14, Severity: SeverityHigh,
			DMails: []string{"f-001", "f-002", "f-003", "f-004", "f-005", "f-006"}},
	}

	// when
	saved, err := a.saveConvergenceDMails(alerts)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved D-Mail, got %d", len(saved))
	}
	if saved[0].Kind != KindConvergence {
		t.Errorf("expected kind convergence, got %s", saved[0].Kind)
	}
}

func TestSaveConvergenceDMails_DeduplicatesExisting(t *testing.T) {
	// given: an existing convergence D-Mail for auth/session.go in archive
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	// Pre-save a convergence D-Mail for the same target
	if err := store.SaveDMail(DMail{
		Name:     "convergence-001",
		Kind:     KindConvergence,
		Severity: SeverityHigh,
		Targets:  []string{"auth/session.go"},
		Metadata: map[string]string{
			"created_at":      time.Now().UTC().Format(time.RFC3339),
			"convergence_for": "f-001,f-002,f-003,f-004,f-005,f-006",
		},
	}); err != nil {
		t.Fatal(err)
	}

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// Same alert that would produce a convergence D-Mail for auth/session.go
	alerts := []ConvergenceAlert{
		{Target: "auth/session.go", Count: 6, Window: 14, Severity: SeverityHigh,
			DMails: []string{"f-001", "f-002", "f-003", "f-004", "f-005", "f-006"}},
	}

	// when
	saved, err := a.saveConvergenceDMails(alerts)

	// then: should NOT create a duplicate
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 0 {
		t.Errorf("expected 0 saved D-Mails (deduplicated), got %d", len(saved))
	}
}

func TestSaveCheckState_ClearsConvergenceAlerts(t *testing.T) {
	// given: a previous result with stale convergence alerts
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	previous := CheckResult{
		Divergence: 0.15,
		ConvergenceAlerts: []ConvergenceAlert{
			{Target: "auth/session.go", Count: 3, Window: 14, Severity: SeverityMedium},
		},
	}

	// when: saving on no-shift path
	err := a.SaveCheckState("abc123", previous, time.Now().UTC())

	// then: convergence alerts should be cleared (not carried over stale)
	if err != nil {
		t.Fatalf("SaveCheckState failed: %v", err)
	}
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if len(loaded.ConvergenceAlerts) != 0 {
		t.Errorf("expected 0 convergence alerts (cleared on no-shift), got %d", len(loaded.ConvergenceAlerts))
	}
}

func TestPrintSync_OutputJSON(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	// D-Mail without issues (no pending comments)
	if err := store.SaveDMail(DMail{
		Name: "feedback-001", Kind: KindFeedback,
		Description: "no issues", Severity: SeverityLow,
	}); err != nil {
		t.Fatal(err)
	}
	// D-Mail with issue but not commented (pending comment)
	if err := store.SaveDMail(DMail{
		Name: "feedback-002", Kind: KindFeedback,
		Description: "has issue", Severity: SeverityLow,
		Issues: []string{"MY-250"},
	}); err != nil {
		t.Fatal(err)
	}
	// D-Mail with issue and already commented (should not appear)
	if err := store.SaveDMail(DMail{
		Name: "feedback-003", Kind: KindFeedback,
		Description: "commented", Severity: SeverityLow,
		Issues: []string{"MY-251"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkCommented("feedback-003", "MY-251"); err != nil {
		t.Fatal(err)
	}

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// when
	err := a.PrintSync()

	// then
	if err != nil {
		t.Fatalf("PrintSync failed: %v", err)
	}

	var output SyncOutput
	if err := json.Unmarshal(dataBuf.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, dataBuf.String())
	}
	if len(output.PendingComments) != 1 {
		t.Errorf("expected 1 pending comment, got %d", len(output.PendingComments))
	}
	if output.PendingComments[0].DMail != "feedback-002" {
		t.Errorf("expected feedback-002, got %s", output.PendingComments[0].DMail)
	}
}

func TestPrintSync_PendingCommentsFromIssues(t *testing.T) {
	// given: D-Mail with 2 issues, one already commented
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	if err := store.SaveDMail(DMail{
		Name: "feedback-020", Kind: KindFeedback,
		Description: "multi-issue", Severity: SeverityLow,
		Issues: []string{"MY-400", "MY-401"},
	}); err != nil {
		t.Fatal(err)
	}
	// Mark one as already commented
	if err := store.MarkCommented("feedback-020", "MY-400"); err != nil {
		t.Fatal(err)
	}

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Projector: &Projector{Store: store}, Logger: NewLogger(&logBuf, false), DataOut: &dataBuf}

	// when
	if err := a.PrintSync(); err != nil {
		t.Fatalf("PrintSync failed: %v", err)
	}

	// then
	var output SyncOutput
	if err := json.Unmarshal(dataBuf.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, dataBuf.String())
	}
	if len(output.PendingComments) != 1 {
		t.Fatalf("expected 1 pending comment, got %d", len(output.PendingComments))
	}
	if output.PendingComments[0].IssueID != "MY-401" {
		t.Errorf("expected MY-401, got %s", output.PendingComments[0].IssueID)
	}
}

func TestMarkCommented_CompositeKey(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	// when: mark same D-Mail for two different issues
	if err := store.MarkCommented("feedback-030", "MY-500"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkCommented("feedback-030", "MY-501"); err != nil {
		t.Fatal(err)
	}

	// then: both entries exist with distinct composite keys
	state, err := store.LoadSyncState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.CommentedDMails) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(state.CommentedDMails))
	}
	if _, ok := state.CommentedDMails["feedback-030:MY-500"]; !ok {
		t.Error("expected key feedback-030:MY-500")
	}
	if _, ok := state.CommentedDMails["feedback-030:MY-501"]; !ok {
		t.Error("expected key feedback-030:MY-501")
	}
}

func TestPrintLog_ShowsConsumed(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	now := time.Now().UTC().Truncate(time.Second)
	if err := store.SaveConsumed([]ConsumedRecord{
		{Name: "report-001", Kind: KindReport, ConsumedAt: now, Source: "report-001.md"},
	}); err != nil {
		t.Fatal(err)
	}

	// Save a check event so PrintLog doesn't bail early
	eventStore := &FileEventStore{Dir: filepath.Join(root, "events")}
	ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{
		Result: CheckResult{
			CheckedAt:  now,
			Commit:     "abc1234",
			Type:       CheckTypeFull,
			Divergence: 0.1,
		},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := eventStore.Append(ev); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    eventStore,
		Projector: &Projector{Store: store},
		Logger:    NewLogger(io.Discard, false),
		DataOut:   &buf,
	}

	// when
	if err := a.PrintLog(); err != nil {
		t.Fatal(err)
	}

	// then
	output := buf.String()
	if !strings.Contains(output, "Consumed") {
		t.Errorf("expected 'Consumed' section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "report-001") {
		t.Errorf("expected 'report-001' in output, got:\n%s", output)
	}
}

func TestPrintLogJSON_IncludesConsumed(t *testing.T) {
	// given
	dir := t.TempDir()
	root := filepath.Join(dir, ".gate")
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(root)

	now := time.Now().UTC().Truncate(time.Second)
	store.SaveConsumed([]ConsumedRecord{
		{Name: "report-001", Kind: KindReport, ConsumedAt: now, Source: "report-001.md"},
	})

	var buf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Logger:    NewLogger(io.Discard, false),
		DataOut:   &buf,
	}

	// when
	if err := a.PrintLogJSON(); err != nil {
		t.Fatal(err)
	}

	// then
	output := buf.String()
	if !strings.Contains(output, `"consumed"`) {
		t.Errorf("expected 'consumed' key in JSON, got:\n%s", output)
	}
	if !strings.Contains(output, "report-001") {
		t.Errorf("expected 'report-001' in JSON output, got:\n%s", output)
	}
}

// --- ClaudeRunner interface tests (MY-376) ---

func TestClaudeRunnerNilFallback(t *testing.T) {
	// given: Amadeus without Claude field set
	a := &Amadeus{Config: DefaultConfig()}

	// when
	runner := a.claudeRunner()

	// then: returns a non-nil ClaudeRunner via fallback
	if runner == nil {
		t.Fatal("expected non-nil ClaudeRunner from nil fallback")
	}
}

func TestRunCheck_ClaudeError_Propagates(t *testing.T) {
	// given: a git repo with Claude that returns an error
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(divRoot)

	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       NewGitClient(repo.dir),
		Claude:    &errorClaudeRunner{err: fmt.Errorf("claude unavailable")},
		Logger:    NewLogger(&logBuf, false),
		DataOut:   &dataBuf,
	}

	// when
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, Quiet: true})

	// then: error propagates
	if err == nil {
		t.Fatal("expected error from RunCheck when Claude fails")
	}
	if !strings.Contains(err.Error(), "claude unavailable") {
		t.Errorf("expected 'claude unavailable' in error, got: %v", err)
	}
}

func TestRunCheck_DryRun_SkipsClaude(t *testing.T) {
	// given: a git repo with a capturing Claude runner
	repo := setupTestRepo(t)
	divRoot := filepath.Join(repo.dir, ".gate")
	if err := InitGateDir(divRoot); err != nil {
		t.Fatal(err)
	}
	store := NewProjectionStore(divRoot)

	capturing := &capturingClaudeRunner{response: `{}`}
	var logBuf, dataBuf bytes.Buffer
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Projector: &Projector{Store: store},
		Git:       NewGitClient(repo.dir),
		Claude:    capturing,
		Logger:    NewLogger(&logBuf, false),
		DataOut:   &dataBuf,
	}

	// when
	err := a.RunCheck(context.Background(), CheckOptions{Full: true, DryRun: true, Quiet: true})

	// then: no error and Claude was never called
	if err != nil {
		t.Fatalf("expected no error from RunCheck in DryRun, got: %v", err)
	}
	if len(capturing.calls) != 0 {
		t.Errorf("expected 0 Claude calls in DryRun, got %d", len(capturing.calls))
	}
}

func TestAmadeus_AutoRebuild_RestoresProjectionsFromEvents(t *testing.T) {
	// given: events exist but .run/latest.json does not
	dir := t.TempDir()
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	eventsDir := filepath.Join(root, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a CheckCompleted event
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{
		Result: CheckResult{
			CheckedAt:  now,
			Commit:     "rebuild-me",
			Type:       CheckTypeFull,
			Divergence: 0.55,
		},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	eventStore := &FileEventStore{Dir: eventsDir}
	if err := eventStore.Append(ev); err != nil {
		t.Fatal(err)
	}

	// Ensure no latest.json exists
	os.Remove(filepath.Join(root, ".run", "latest.json"))

	store := NewProjectionStore(root)
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    eventStore,
		Projector: &Projector{Store: store},
		Logger:    NewLogger(io.Discard, false),
	}

	// when
	if err := a.autoRebuildIfNeeded(); err != nil {
		t.Fatalf("autoRebuildIfNeeded: %v", err)
	}

	// then: latest.json should be restored from events
	latest, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest: %v", err)
	}
	if latest.Commit != "rebuild-me" {
		t.Errorf("Commit = %q, want %q", latest.Commit, "rebuild-me")
	}
	if latest.Divergence != 0.55 {
		t.Errorf("Divergence = %f, want %f", latest.Divergence, 0.55)
	}
}

func TestAmadeus_AutoRebuild_SkipsWhenLatestExists(t *testing.T) {
	// given: latest.json already exists
	dir := t.TempDir()
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	store := NewProjectionStore(root)
	existing := CheckResult{Commit: "existing", Divergence: 0.10}
	if err := store.SaveLatest(existing); err != nil {
		t.Fatal(err)
	}

	eventsDir := filepath.Join(root, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    &FileEventStore{Dir: eventsDir},
		Projector: &Projector{Store: store},
		Logger:    NewLogger(io.Discard, false),
	}

	// when
	if err := a.autoRebuildIfNeeded(); err != nil {
		t.Fatalf("autoRebuildIfNeeded: %v", err)
	}

	// then: latest should still be the original, not rebuilt
	latest, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if latest.Commit != "existing" {
		t.Errorf("Commit = %q, want %q (should not rebuild)", latest.Commit, "existing")
	}
}

func TestAmadeus_AutoRebuild_SkipsWhenNoEventStore(t *testing.T) {
	// given: Events is nil (event sourcing disabled)
	dir := t.TempDir()
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}

	store := NewProjectionStore(root)
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    nil, // disabled
		Projector: nil,
		Logger:    NewLogger(io.Discard, false),
	}

	// when: should not panic or error
	if err := a.autoRebuildIfNeeded(); err != nil {
		t.Fatalf("autoRebuildIfNeeded with nil Events: %v", err)
	}
}

func TestAmadeus_AutoRebuild_SkipsWhenInboxEventsExist(t *testing.T) {
	// given: events include InboxConsumed, latest.json is missing, archive has inbox D-Mail
	dir := t.TempDir()
	root := dir + "/.gate"
	if err := InitGateDir(root); err != nil {
		t.Fatal(err)
	}
	eventsDir := filepath.Join(root, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)

	// CheckCompleted event
	ev1, err := NewEvent(EventCheckCompleted, CheckCompletedData{
		Result: CheckResult{CheckedAt: now, Commit: "aaa", Divergence: 0.2},
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// InboxConsumed event (metadata only — D-Mail content is NOT in the event)
	ev2, err := NewEvent(EventInboxConsumed, InboxConsumedData{
		Name: "report-001", Kind: KindReport, Source: "report-001.md",
	}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}

	eventStore := &FileEventStore{Dir: eventsDir}
	if err := eventStore.Append(ev1, ev2); err != nil {
		t.Fatal(err)
	}

	// Place an inbox-sourced D-Mail in archive/ (simulates ScanInbox at runtime)
	archiveDir := filepath.Join(root, "archive")
	inboxDMail := filepath.Join(archiveDir, "report-001.md")
	os.WriteFile(inboxDMail, []byte("---\nname: report-001\nkind: report\n---\nImportant report\n"), 0o644)

	// Remove latest.json to trigger rebuild condition
	os.Remove(filepath.Join(root, ".run", "latest.json"))

	store := NewProjectionStore(root)
	a := &Amadeus{
		Config:    DefaultConfig(),
		Store:     store,
		Events:    eventStore,
		Projector: &Projector{Store: store},
		Logger:    NewLogger(io.Discard, false),
	}

	// when
	if err := a.autoRebuildIfNeeded(); err != nil {
		t.Fatalf("autoRebuildIfNeeded: %v", err)
	}

	// then: inbox D-Mail in archive/ must NOT be destroyed
	if _, err := os.Stat(inboxDMail); err != nil {
		t.Errorf("inbox D-Mail was destroyed by auto-rebuild: %v", err)
	}

	// then: latest.json should NOT be restored (rebuild was skipped)
	latest, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if !latest.CheckedAt.IsZero() {
		t.Error("expected latest to remain empty (rebuild should have been skipped)")
	}
}

func TestEmit_ReturnsErrorWhenBothNil(t *testing.T) {
	// given: Amadeus with neither Events nor Projector
	a := &Amadeus{Config: DefaultConfig()}

	ev, err := NewEvent(EventCheckCompleted, CheckCompletedData{
		Result: CheckResult{Commit: "abc"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	emitErr := a.emit(ev)

	// then: should return error, not silently succeed
	if emitErr == nil {
		t.Error("expected error when both Events and Projector are nil")
	}
}
