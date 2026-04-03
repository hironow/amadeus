package domain

import (
	"regexp"
	"strings"
	"time"
)

// ProviderErrorKind classifies the type of provider error.
type ProviderErrorKind int

const (
	// ProviderErrorNone indicates no provider-level error (normal failure).
	ProviderErrorNone ProviderErrorKind = iota
	// ProviderErrorRateLimit indicates a rate limit was hit.
	ProviderErrorRateLimit
	// ProviderErrorServer indicates a server-side error (5xx).
	ProviderErrorServer
)

// ProviderErrorInfo holds the classified result of a provider error.
type ProviderErrorInfo struct {
	Kind    ProviderErrorKind
	ResetAt time.Time // parsed reset time (zero if unknown)
}

// IsTrip returns true if the error should trip a circuit breaker.
func (i ProviderErrorInfo) IsTrip() bool {
	return i.Kind != ProviderErrorNone
}

// ClassifyProviderError inspects stderr output and classifies the error
// based on the provider. Returns ProviderErrorNone if the error is not
// a rate limit or server error.
func ClassifyProviderError(provider Provider, stderr string) ProviderErrorInfo {
	switch provider {
	case ProviderClaudeCode:
		return classifyClaudeError(stderr)
	case ProviderCodex:
		return classifyCodexError(stderr)
	default:
		// Unknown providers: fall back to generic patterns
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

func classifyClaudeError(stderr string) ProviderErrorInfo {
	lower := strings.ToLower(stderr)
	for _, p := range claudeRateLimitPatterns {
		if strings.Contains(lower, p) {
			return ProviderErrorInfo{
				Kind:    ProviderErrorRateLimit,
				ResetAt: parseResetTime(stderr),
			}
		}
	}
	for _, p := range claudeServerErrorPatterns {
		if strings.Contains(lower, p) {
			return ProviderErrorInfo{Kind: ProviderErrorServer}
		}
	}
	return ProviderErrorInfo{}
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

func classifyCodexError(stderr string) ProviderErrorInfo {
	lower := strings.ToLower(stderr)
	for _, p := range codexRateLimitPatterns {
		if strings.Contains(lower, p) {
			return ProviderErrorInfo{
				Kind:    ProviderErrorRateLimit,
				ResetAt: parseResetTime(stderr),
			}
		}
	}
	for _, p := range codexServerErrorPatterns {
		if strings.Contains(lower, p) {
			return ProviderErrorInfo{Kind: ProviderErrorServer}
		}
	}
	return ProviderErrorInfo{}
}

// Generic fallback for unknown providers
func classifyGenericError(stderr string) ProviderErrorInfo {
	lower := strings.ToLower(stderr)
	genericRatePatterns := []string{"rate limit", "too many requests", "hit your limit"}
	genericServerPatterns := []string{"overloaded", "internal server error", "service unavailable"}
	for _, p := range genericRatePatterns {
		if strings.Contains(lower, p) {
			return ProviderErrorInfo{
				Kind:    ProviderErrorRateLimit,
				ResetAt: parseResetTime(stderr),
			}
		}
	}
	for _, p := range genericServerPatterns {
		if strings.Contains(lower, p) {
			return ProviderErrorInfo{Kind: ProviderErrorServer}
		}
	}
	return ProviderErrorInfo{}
}

// resetTimeRe extracts the reset time from rate limit messages.
// Example: "resets Apr 3 at 1pm (Asia/Tokyo)"
var resetTimeRe = regexp.MustCompile(`resets?\s+(.+?)\s*\(([^)]+)\)`)

// parseResetTime extracts reset time from stderr.
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

	// Try with ordinal suffix removed
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
