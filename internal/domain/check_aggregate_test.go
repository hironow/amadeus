package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestCheckAggregate_ShouldFullCheck_ForceFlag(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())

	// when — force flag overrides everything
	result := agg.ShouldFullCheck(true)

	// then
	if !result {
		t.Error("expected full check when force flag is set")
	}
}

func TestCheckAggregate_ShouldFullCheck_IntervalReached(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.FullCheck.Interval = 3
	agg := domain.NewCheckAggregate(cfg)
	// Simulate 3 diff checks
	for range 3 {
		agg.AdvanceCheckCount(false, false)
	}

	// when
	result := agg.ShouldFullCheck(false)

	// then
	if !result {
		t.Error("expected full check when interval reached")
	}
}

func TestCheckAggregate_ShouldFullCheck_BelowInterval(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.FullCheck.Interval = 5
	agg := domain.NewCheckAggregate(cfg)

	// when
	result := agg.ShouldFullCheck(false)

	// then
	if result {
		t.Error("expected diff check when below interval")
	}
}

func TestCheckAggregate_AdvanceCheckCount_Diff(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())

	// when
	agg.AdvanceCheckCount(false, false)
	agg.AdvanceCheckCount(false, false)

	// then
	if agg.CheckCount() != 2 {
		t.Errorf("expected check count 2, got %d", agg.CheckCount())
	}
}

func TestCheckAggregate_AdvanceCheckCount_FullResetsToZero(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.AdvanceCheckCount(false, false)
	agg.AdvanceCheckCount(false, false)

	// when
	agg.AdvanceCheckCount(true, false) // full check resets

	// then
	if agg.CheckCount() != 0 {
		t.Errorf("expected check count 0 after full, got %d", agg.CheckCount())
	}
}

func TestCheckAggregate_ShouldPromoteToFull(t *testing.T) {
	// given
	cfg := domain.DefaultConfig()
	cfg.FullCheck.OnDivergenceJump = 0.05
	agg := domain.NewCheckAggregate(cfg)

	// when — small delta below threshold
	small := agg.ShouldPromoteToFull(0.10, 0.12)

	// then
	if small {
		t.Error("expected no promotion for small delta")
	}

	// when — large delta above threshold
	large := agg.ShouldPromoteToFull(0.10, 0.20)

	// then
	if !large {
		t.Error("expected promotion for large delta")
	}
}

func TestCheckAggregate_ShouldFullCheck_ForceFullNext(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.SetForceFullNext(true)

	// when
	result := agg.ShouldFullCheck(false)

	// then
	if !result {
		t.Error("expected full check when ForceFullNext is set")
	}
}

func TestCheckAggregate_Restore(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	prev := domain.CheckResult{
		CheckCountSinceFull: 3,
		ForceFullNext:       true,
		Divergence:          0.42,
	}

	// when
	agg.Restore(prev)

	// then
	if agg.CheckCount() != 3 {
		t.Errorf("expected check count 3, got %d", agg.CheckCount())
	}
	if !agg.ForceFullNext() {
		t.Error("expected ForceFullNext to be true after restore")
	}
}

func TestCheckAggregate_RecordCheck_ProducesEvents(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	result := domain.CheckResult{
		CheckedAt:  time.Now().UTC(),
		Commit:     "abc123",
		Type:       domain.CheckTypeDiff,
		Divergence: 0.15,
	}

	// when
	events, err := agg.RecordCheck(result, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Type != domain.EventCheckCompleted {
		t.Errorf("expected %s, got %s", domain.EventCheckCompleted, events[0].Type)
	}
}

func TestCheckAggregate_RecordCheck_GateDeniedFullCheckSkipsBaseline(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	result := domain.CheckResult{
		CheckedAt:  time.Now().UTC(),
		Commit:     "abc123",
		Type:       domain.CheckTypeFull,
		Divergence: 0.15,
		GateDenied: true,
	}

	// when
	events, err := agg.RecordCheck(result, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event for gate-denied full check, got %d", len(events))
	}
	if events[0].Type != domain.EventCheckCompleted {
		t.Errorf("expected %s, got %s", domain.EventCheckCompleted, events[0].Type)
	}
}

func TestCheckAggregate_ShouldFullCheck_SuppressedDuringCooldown(t *testing.T) {
	// given: forceFullNext is true, then full check completes
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.SetForceFullNext(true)
	agg.AdvanceCheckCount(true, false) // triggers cooldown

	// when: check if another full check should run
	shouldFull := agg.ShouldFullCheck(false)

	// then: should be suppressed during cooldown
	if shouldFull {
		t.Error("expected ShouldFullCheck to return false during cooldown")
	}
	if agg.CooldownRemaining() != 3 {
		t.Errorf("expected cooldown 3, got %d", agg.CooldownRemaining())
	}
}

func TestCheckAggregate_ShouldFullCheck_FiresAfterCooldownExpires(t *testing.T) {
	// given: force full check, advance to trigger cooldown
	cfg := domain.DefaultConfig()
	cfg.FullCheck.Interval = 1 // low interval so it triggers after cooldown
	agg := domain.NewCheckAggregate(cfg)
	agg.SetForceFullNext(true)
	agg.AdvanceCheckCount(true, false) // cooldown starts at 3

	// when: advance 3 diff checks to expire cooldown
	for range 3 {
		agg.AdvanceCheckCount(false, false)
	}

	// then: cooldown expired, should allow full check
	if agg.CooldownRemaining() != 0 {
		t.Errorf("expected cooldown 0, got %d", agg.CooldownRemaining())
	}
	// checkCount is now 3, interval is 1, so should be true
	if !agg.ShouldFullCheck(false) {
		t.Error("expected ShouldFullCheck to return true after cooldown expires")
	}
}

func TestCheckAggregate_AdvanceCheckCount_ClearsForceFullAndStartsCooldown(t *testing.T) {
	// given: forceFullNext is true
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.SetForceFullNext(true)

	// when: advance with full check
	agg.AdvanceCheckCount(true, false)

	// then: forceFullNext cleared, cooldown started
	if agg.ForceFullNext() {
		t.Error("expected ForceFullNext to be false after AdvanceCheckCount(true)")
	}
	if agg.CheckCount() != 0 {
		t.Errorf("expected check count 0, got %d", agg.CheckCount())
	}
	if agg.CooldownRemaining() != 3 {
		t.Errorf("expected cooldown 3, got %d", agg.CooldownRemaining())
	}
}

func TestCheckAggregate_ShouldFullCheck_ForceFlagOverridesCooldown(t *testing.T) {
	// given: cooldown is active (forceFullNext was consumed)
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.SetForceFullNext(true)
	agg.AdvanceCheckCount(true, false) // triggers cooldown via forceFullNext still true

	// when: explicit --full flag is passed during cooldown
	result := agg.ShouldFullCheck(true)

	// then: forceFlag always overrides cooldown
	if !result {
		t.Error("expected forceFlag to override cooldown")
	}
}

func TestCheckAggregate_AdvanceCheckCount_WasForcedStartsCooldown(t *testing.T) {
	// given: forceFullNext was already cleared (simulating detectShift flow),
	// but wasForced=true is passed to signal the check was force-triggered.
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	// Note: forceFullNext is false — simulating that detectShift cleared it.

	// when: advance with fullCheck=true and wasForced=true
	agg.AdvanceCheckCount(true, true)

	// then: cooldown should start even though forceFullNext is false
	if agg.CooldownRemaining() != 3 {
		t.Errorf("expected cooldown 3 from wasForced, got %d", agg.CooldownRemaining())
	}
}

func TestCheckAggregate_RecordCheck_ClearsForceFullNext(t *testing.T) {
	// given: forceFullNext is true
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.SetForceFullNext(true)
	result := domain.CheckResult{
		CheckedAt:  time.Now().UTC(),
		Commit:     "abc123",
		Type:       domain.CheckTypeFull,
		Divergence: 0.15,
	}

	// when
	_, err := agg.RecordCheck(result, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: forceFullNext should be cleared
	if agg.ForceFullNext() {
		t.Error("expected ForceFullNext to be false after RecordCheck")
	}
}

func TestCheckAggregate_RecordCheck_ClearsForceFullNext_GateDenied(t *testing.T) {
	// given: forceFullNext is true, check is gate-denied
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	agg.SetForceFullNext(true)
	result := domain.CheckResult{
		CheckedAt:  time.Now().UTC(),
		Commit:     "abc123",
		Type:       domain.CheckTypeFull,
		Divergence: 0.15,
		GateDenied: true,
	}

	// when
	_, err := agg.RecordCheck(result, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: forceFullNext should be cleared even on gate-denied
	if agg.ForceFullNext() {
		t.Error("expected ForceFullNext to be false after gate-denied RecordCheck")
	}
}

func TestCheckAggregate_RecordCheck_NoForceFullNext_RemainsFalse(t *testing.T) {
	// given: forceFullNext is not set
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	result := domain.CheckResult{
		CheckedAt:  time.Now().UTC(),
		Commit:     "abc123",
		Type:       domain.CheckTypeDiff,
		Divergence: 0.15,
	}

	// when
	_, err := agg.RecordCheck(result, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then
	if agg.ForceFullNext() {
		t.Error("expected ForceFullNext to remain false")
	}
}

func TestCheckAggregate_RecordCheck_FullCheckProducesBaselineEvent(t *testing.T) {
	// given
	agg := domain.NewCheckAggregate(domain.DefaultConfig())
	result := domain.CheckResult{
		CheckedAt:  time.Now().UTC(),
		Commit:     "abc123",
		Type:       domain.CheckTypeFull,
		Divergence: 0.15,
	}

	// when
	events, err := agg.RecordCheck(result, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events for full check, got %d", len(events))
	}
	if events[1].Type != domain.EventBaselineUpdated {
		t.Errorf("expected %s as second event, got %s", domain.EventBaselineUpdated, events[1].Type)
	}
}
