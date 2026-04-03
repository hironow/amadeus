package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

func TestClassifyProviderError_Claude_RateLimit(t *testing.T) {
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "You've hit your limit")
	if info.Kind != domain.ProviderErrorRateLimit {
		t.Fatalf("expected RateLimit, got %v", info.Kind)
	}
}

func TestClassifyProviderError_Claude_ServerError(t *testing.T) {
	cases := []string{
		"Anthropic API is overloaded",
		"Error: 529 overloaded",
		"Error: 500 Internal Server Error",
		"Error: 502 Bad Gateway",
		"Error: 503 Service Unavailable",
	}
	for _, stderr := range cases {
		info := domain.ClassifyProviderError(domain.ProviderClaudeCode, stderr)
		if info.Kind != domain.ProviderErrorServer {
			t.Errorf("expected Server for %q, got %v", stderr, info.Kind)
		}
	}
}

func TestClassifyProviderError_Claude_NormalError(t *testing.T) {
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode, "some normal error")
	if info.Kind != domain.ProviderErrorNone {
		t.Fatalf("expected None, got %v", info.Kind)
	}
}

func TestClassifyProviderError_Codex_RateLimit(t *testing.T) {
	cases := []string{
		"rate_limit_exceeded: too many requests",
		"Error: Too Many Requests",
		"rate limit reached",
	}
	for _, stderr := range cases {
		info := domain.ClassifyProviderError(domain.ProviderCodex, stderr)
		if info.Kind != domain.ProviderErrorRateLimit {
			t.Errorf("expected RateLimit for %q, got %v", stderr, info.Kind)
		}
	}
}

func TestClassifyProviderError_Codex_ServerError(t *testing.T) {
	info := domain.ClassifyProviderError(domain.ProviderCodex, "internal server error")
	if info.Kind != domain.ProviderErrorServer {
		t.Fatalf("expected Server, got %v", info.Kind)
	}
}

func TestClassifyProviderError_UnknownProvider_FallsBackToGeneric(t *testing.T) {
	info := domain.ClassifyProviderError(domain.ProviderGeminiCLI, "rate limit exceeded")
	if info.Kind != domain.ProviderErrorRateLimit {
		t.Fatalf("expected RateLimit via generic, got %v", info.Kind)
	}
}

func TestClassifyProviderError_ParsesResetTime(t *testing.T) {
	info := domain.ClassifyProviderError(domain.ProviderClaudeCode,
		"You've hit your limit · resets Apr 3 at 1pm (Asia/Tokyo)")
	if info.Kind != domain.ProviderErrorRateLimit {
		t.Fatalf("expected RateLimit, got %v", info.Kind)
	}
	if info.ResetAt.IsZero() {
		t.Fatal("expected non-zero ResetAt")
	}
	loc, _ := time.LoadLocation("Asia/Tokyo")
	if info.ResetAt.Location().String() != loc.String() {
		t.Errorf("expected Asia/Tokyo, got %s", info.ResetAt.Location())
	}
}

func TestProviderErrorInfo_IsTrip(t *testing.T) {
	if (domain.ProviderErrorInfo{Kind: domain.ProviderErrorNone}).IsTrip() {
		t.Error("None should not trip")
	}
	if !(domain.ProviderErrorInfo{Kind: domain.ProviderErrorRateLimit}).IsTrip() {
		t.Error("RateLimit should trip")
	}
	if !(domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer}).IsTrip() {
		t.Error("Server should trip")
	}
}
