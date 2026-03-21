package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestBaselineStalenessConfig_IsStale_StaleBaseline(t *testing.T) {
	// given: max age is 7 days, last check was 8 days ago
	cfg := domain.BaselineStalenessConfig{MaxAgeDays: 7}
	checkedAt := time.Now().UTC().Add(-8 * 24 * time.Hour)

	// when
	result := cfg.IsStale(checkedAt)

	// then
	if !result {
		t.Error("expected IsStale to return true when last check exceeds MaxAgeDays")
	}
}

func TestBaselineStalenessConfig_IsStale_FreshBaseline(t *testing.T) {
	// given: max age is 7 days, last check was 3 days ago
	cfg := domain.BaselineStalenessConfig{MaxAgeDays: 7}
	checkedAt := time.Now().UTC().Add(-3 * 24 * time.Hour)

	// when
	result := cfg.IsStale(checkedAt)

	// then
	if result {
		t.Error("expected IsStale to return false when last check is within MaxAgeDays")
	}
}

func TestBaselineStalenessConfig_IsStale_ZeroDisabled(t *testing.T) {
	// given: MaxAgeDays=0 means disabled
	cfg := domain.BaselineStalenessConfig{MaxAgeDays: 0}
	checkedAt := time.Now().UTC().Add(-365 * 24 * time.Hour) // very old

	// when
	result := cfg.IsStale(checkedAt)

	// then
	if result {
		t.Error("expected IsStale to return false when MaxAgeDays is 0 (disabled)")
	}
}

func TestBaselineStalenessConfig_IsStale_MissingTimestamp(t *testing.T) {
	// given: MaxAgeDays=7, checkedAt is zero time (missing timestamp)
	cfg := domain.BaselineStalenessConfig{MaxAgeDays: 7}
	var checkedAt time.Time // zero value

	// when
	result := cfg.IsStale(checkedAt)

	// then: zero time should not be considered stale (no prior check exists)
	if result {
		t.Error("expected IsStale to return false for missing (zero) timestamp")
	}
}

func TestDefaultConfig_BaselineStalenessDisabledByDefault(t *testing.T) {
	// given/when
	cfg := domain.DefaultConfig()

	// then: staleness check is disabled by default
	if cfg.BaselineStaleness.MaxAgeDays != 0 {
		t.Errorf("expected BaselineStaleness.MaxAgeDays to be 0 (disabled), got %d", cfg.BaselineStaleness.MaxAgeDays)
	}
}
