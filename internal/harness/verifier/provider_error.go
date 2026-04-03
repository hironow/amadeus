package verifier

import (
	"regexp"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// ClassifyProviderError inspects stderr output and classifies the error
// based on the provider. Returns ProviderErrorNone if the error is not
// a rate limit or server error.
func ClassifyProviderError(provider domain.Provider, stderr string) domain.ProviderErrorInfo {
	switch provider {
	case domain.ProviderClaudeCode:
		return classifyClaudeError(stderr)
	case domain.ProviderCodex:
		return classifyCodexError(stderr)
	default:
		return classifyGenericError(stderr)
	}
}

// Claude-specific patterns
var claudeRateLimitPatterns = []string{
	"hit your limit",
	"hit your usage limit",
}

var claudeServerErrorPatterns = []string{
	"overloaded",
	"529",
	"500 ",
	"502 ",
	"503 ",
}

func classifyClaudeError(stderr string) domain.ProviderErrorInfo {
	lower := strings.ToLower(stderr)
	for _, p := range claudeRateLimitPatterns {
		if strings.Contains(lower, p) {
			return domain.ProviderErrorInfo{
				Kind:    domain.ProviderErrorRateLimit,
				ResetAt: parseResetTime(stderr),
			}
		}
	}
	for _, p := range claudeServerErrorPatterns {
		if strings.Contains(lower, p) {
			return domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer}
		}
	}
	return domain.ProviderErrorInfo{}
}

// Codex-specific patterns
var codexRateLimitPatterns = []string{
	"rate limit",
	"rate_limit_exceeded",
	"too many requests",
}

var codexServerErrorPatterns = []string{
	"internal server error",
	"service unavailable",
	"bad gateway",
}

func classifyCodexError(stderr string) domain.ProviderErrorInfo {
	lower := strings.ToLower(stderr)
	for _, p := range codexRateLimitPatterns {
		if strings.Contains(lower, p) {
			return domain.ProviderErrorInfo{
				Kind:    domain.ProviderErrorRateLimit,
				ResetAt: parseResetTime(stderr),
			}
		}
	}
	for _, p := range codexServerErrorPatterns {
		if strings.Contains(lower, p) {
			return domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer}
		}
	}
	return domain.ProviderErrorInfo{}
}

// Generic fallback for unknown providers
func classifyGenericError(stderr string) domain.ProviderErrorInfo {
	lower := strings.ToLower(stderr)
	genericRatePatterns := []string{"rate limit", "too many requests", "hit your limit"}
	genericServerPatterns := []string{"overloaded", "internal server error", "service unavailable"}
	for _, p := range genericRatePatterns {
		if strings.Contains(lower, p) {
			return domain.ProviderErrorInfo{
				Kind:    domain.ProviderErrorRateLimit,
				ResetAt: parseResetTime(stderr),
			}
		}
	}
	for _, p := range genericServerPatterns {
		if strings.Contains(lower, p) {
			return domain.ProviderErrorInfo{Kind: domain.ProviderErrorServer}
		}
	}
	return domain.ProviderErrorInfo{}
}

var resetTimeRe = regexp.MustCompile(`resets?\s+(.+?)\s*\(([^)]+)\)`)

func parseResetTime(stderr string) time.Time {
	matches := resetTimeRe.FindStringSubmatch(stderr)
	if len(matches) < 3 {
		return time.Time{}
	}
	dateStr := strings.TrimSpace(matches[1])
	tzName := strings.TrimSpace(matches[2])

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.Time{}
	}

	formats := []string{
		"Jan 2 at 3pm",
		"Jan 2 at 3:04pm",
		"Jan 2 at 3:04 PM",
		"Jan 2 at 3PM",
		"Jan 2 at 3 PM",
	}

	now := time.Now().In(loc)
	for _, f := range formats {
		t, err := time.ParseInLocation(f, dateStr, loc)
		if err == nil {
			t = t.AddDate(now.Year(), 0, 0)
			if t.Before(now.Add(-24 * time.Hour)) {
				t = t.AddDate(1, 0, 0)
			}
			return t
		}
	}

	cleaned := removeOrdinalSuffix(dateStr)
	if cleaned != dateStr {
		for _, f := range formats {
			t, err := time.ParseInLocation(f, cleaned, loc)
			if err == nil {
				t = t.AddDate(now.Year(), 0, 0)
				if t.Before(now.Add(-24 * time.Hour)) {
					t = t.AddDate(1, 0, 0)
				}
				return t
			}
		}
	}

	return time.Time{}
}

var ordinalRe = regexp.MustCompile(`(\d+)(st|nd|rd|th)\b`)

func removeOrdinalSuffix(s string) string {
	return ordinalRe.ReplaceAllString(s, "$1")
}
