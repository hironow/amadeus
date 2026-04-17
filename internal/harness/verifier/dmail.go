package verifier

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// Kind validation uses domain.ParseKindString as the single canonical path.

// validSeverities is the set of valid Severity values per schema v1.
var validSeverities = map[domain.Severity]bool{
	domain.SeverityLow:    true,
	domain.SeverityMedium: true,
	domain.SeverityHigh:   true,
}

// validActions is the set of valid DMailAction values per schema v1.
var validActions = map[domain.DMailAction]bool{
	domain.ActionRetry:    true,
	domain.ActionEscalate: true,
	domain.ActionResolve:  true,
}

// ValidateDMail validates a D-Mail against schema v1 rules.
// Returns a list of validation errors (empty if valid).
func ValidateDMail(dmail domain.DMail) []string {
	var errs []string
	if dmail.SchemaVersion == "" {
		errs = append(errs, "dmail-schema-version is required")
	} else if dmail.SchemaVersion != domain.DMailSchemaVersion {
		errs = append(errs, fmt.Sprintf("unsupported dmail-schema-version: %q (want %q)", dmail.SchemaVersion, domain.DMailSchemaVersion))
	}
	if dmail.Name == "" {
		errs = append(errs, "name is required")
	}
	if dmail.Kind == "" {
		errs = append(errs, "kind is required")
	} else if _, err := domain.ParseKindString(string(dmail.Kind)); err != nil {
		errs = append(errs, err.Error())
	}
	if dmail.Description == "" {
		errs = append(errs, "description is required")
	}
	if dmail.Severity != "" && !validSeverities[dmail.Severity] {
		errs = append(errs, fmt.Sprintf("invalid severity: %q", dmail.Severity))
	}
	if dmail.Action != "" && !validActions[dmail.Action] {
		errs = append(errs, fmt.Sprintf("invalid action %q", dmail.Action))
	}
	if strings.TrimSpace(dmail.Body) == "" {
		errs = append(errs, "body is required")
	}
	errs = append(errs, validateTargets(dmail.Targets)...)
	return errs
}

// validateTargets checks D-Mail targets for path traversal and duplicates.
func validateTargets(targets []string) []string {
	var errs []string
	seen := make(map[string]bool)
	for _, target := range targets {
		if strings.TrimSpace(target) == "" {
			errs = append(errs, "target must not be empty")
			continue
		}
		if filepath.IsAbs(target) {
			errs = append(errs, fmt.Sprintf("target %q must be a relative path", target))
			continue
		}
		if containsDotDotElement(target) {
			errs = append(errs, fmt.Sprintf("target %q contains path traversal", target))
			continue
		}
		if seen[target] {
			errs = append(errs, fmt.Sprintf("duplicate target %q", target))
			continue
		}
		seen[target] = true
	}
	return errs
}

func containsDotDotElement(path string) bool {
	for _, elem := range strings.Split(filepath.ToSlash(path), "/") {
		if elem == ".." {
			return true
		}
	}
	return false
}
