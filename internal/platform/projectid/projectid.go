// Package projectid resolves the multiplex project identifier for the
// current producer context and injects it into D-Mail metadata maps.
//
// Resolution priority:
//  1. environment variable RUNOPS_PROJECT_ID
//  2. CWD path inference: <home>/projects/<id>/...
//  3. empty (legacy single-mode, frontmatter `project_id` line is omitted)
//
// The ID is validated against the gateway-side `domain.ValidateProjectID`
// regex (`^[a-zA-Z0-9_-]+$`, max 64 chars). Invalid values are treated as
// unresolved.
//
// This file is part of the substrate canonical lock (S0037) and must be
// byte-identical across the four producer tools (sightjack / paintress /
// amadeus / dominator) modulo the `package` declaration import path.
package projectid

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	envVarName  = "RUNOPS_PROJECT_ID"
	maxIDLen    = 64
	projectsDir = "projects"
	metadataKey = "project_id"
	sourceEnv   = "env"
	sourceCWD   = "cwd"
)

var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Resolve returns the project_id for the current producer context and the
// source it was derived from. Pass cwd="" to skip CWD inference.
//
// Returns ("", "") when no signal yields a valid id.
func Resolve(cwd string) (id, source string) {
	if envID := strings.TrimSpace(os.Getenv(envVarName)); envID != "" && IsValidProjectID(envID) {
		return envID, sourceEnv
	}
	if cwd == "" {
		return "", ""
	}
	if cwdID := inferFromCWD(cwd); cwdID != "" && IsValidProjectID(cwdID) {
		return cwdID, sourceCWD
	}
	return "", ""
}

// IsValidProjectID returns true if id matches the canonical project_id
// regex (`^[a-zA-Z0-9_-]+$`) and length constraint (1-64 chars). The rule
// mirrors gateway-side domain.ValidateProjectID for defence-in-depth.
func IsValidProjectID(id string) bool {
	if id == "" || len(id) > maxIDLen {
		return false
	}
	return validIDPattern.MatchString(id)
}

// InjectProjectID resolves the project_id from the current process
// context (env > os.Getwd > empty) and writes it into the provided
// metadata map. Returns the (possibly newly-allocated) map. No-op when
// the project_id cannot be resolved.
//
// Callers wire it as:
//
//	mail.Metadata = projectid.InjectProjectID(mail.Metadata)
func InjectProjectID(metadata map[string]string) map[string]string {
	cwd, _ := os.Getwd()
	id, _ := Resolve(cwd)
	if id == "" {
		return metadata
	}
	if metadata == nil {
		metadata = make(map[string]string, 1)
	}
	metadata[metadataKey] = id
	return metadata
}

// inferFromCWD walks up cwd looking for a `<home>/projects/<id>/...`
// pattern and returns the candidate id (un-validated). Returns "" when
// cwd does not match or HOME is unavailable.
func inferFromCWD(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	rel, err := filepath.Rel(home, cwd)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 || parts[0] != projectsDir {
		return ""
	}
	return parts[1]
}
