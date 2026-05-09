// Package actortype resolves the producer-side caller type taxonomy and
// injects it into D-Mail metadata so that gateway-side ADR 0035/0036/0037
// (= AI agent cannot approve another AI agent) can classify the original
// requester at button-click time.
//
// Resolution priority for `RUNOPS_ACTOR_TYPE`:
//  1. environment variable, exact match against one of the 4 canonical
//     values: human-operator / gateway-service / ai-agent / workspace-daemon.
//  2. unset / empty — legacy compat (frontmatter `requester_actor_type`
//     line is omitted; gateway HIGH path fails closed, non-HIGH path uses
//     migration fallback per gateway ADR 0036 §Migration).
//
// Invalid env values (= set but not canonical) return ErrInvalidActorType
// so the caller can fail the D-Mail emit. This is the silent-escalation
// guard mandated by gateway ADR 0037 §Producer-side validation.
//
// `requester_actor_source` is always written as "env" — producer-side
// "broker" attestation is forbidden by gateway ADR 0037 Axis 1 (broker
// is gateway-only). Writing "broker" would be reclassified as
// `spoofed_broker` on the gateway side and fail closed.
//
// This file is part of the substrate canonical lock (S0037) and must be
// byte-identical across the four producer tools (sightjack / paintress /
// amadeus / dominator) modulo the `package` declaration import path.
package actortype

import (
	"errors"
	"os"
	"strings"
)

// ErrInvalidActorType is returned when RUNOPS_ACTOR_TYPE env is set but
// not one of the 4 canonical values. Callers MUST fail the emit on this
// error per gateway ADR 0037 §Producer-side validation: silent escalation
// via migration fallback is disallowed.
var ErrInvalidActorType = errors.New("invalid RUNOPS_ACTOR_TYPE")

// ErrInvalidInitiatingActorType is returned when RUNOPS_INITIATING_ACTOR_TYPE
// env is set but not one of the 4 canonical values. Callers MUST fail the
// emit on this error.
var ErrInvalidInitiatingActorType = errors.New("invalid RUNOPS_INITIATING_ACTOR_TYPE")

const (
	envVarName            = "RUNOPS_ACTOR_TYPE"
	initiatingEnvName     = "RUNOPS_INITIATING_ACTOR_TYPE"
	sourceEnv             = "env"
	metadataKeyType       = "requester_actor_type"
	metadataKeySource     = "requester_actor_source"
	metadataKeyInitiating = "initiating_actor_type"

	actorHumanOperator   = "human-operator"
	actorGatewayService  = "gateway-service"
	actorAIAgent         = "ai-agent"
	actorWorkspaceDaemon = "workspace-daemon"
)

// canonicalActorTypes is the closed set of 4 caller-type values aligned
// with gateway-side domain.CallerType string values.
var canonicalActorTypes = map[string]struct{}{
	actorHumanOperator:   {},
	actorGatewayService:  {},
	actorAIAgent:         {},
	actorWorkspaceDaemon: {},
}

// IsValidActorType returns true if t is one of the 4 canonical caller-type
// values. Case-sensitive, exact match.
func IsValidActorType(t string) bool {
	_, ok := canonicalActorTypes[t]
	return ok
}

// Resolve reads RUNOPS_ACTOR_TYPE env and returns (actorType, source).
// Priority: env > empty.
//
//   - env unset / empty            → ("", "", nil)             [legacy compat path]
//   - env set + 4 canonical values → (canonical, "env", nil)
//   - env set + invalid value      → ("", "", ErrInvalidActorType)
func Resolve() (actorType, source string, err error) {
	raw := os.Getenv(envVarName)
	if raw == "" {
		return "", "", nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !IsValidActorType(trimmed) {
		return "", "", ErrInvalidActorType
	}
	return trimmed, sourceEnv, nil
}

// ResolveInitiating reads RUNOPS_INITIATING_ACTOR_TYPE env and returns the
// initiating actor type for the workspace-daemon path.
//
//   - env unset / empty            → ("", nil)             [HIGH path: gateway ADR 0036 fail-closed catches; non-HIGH: migration fallback]
//   - env set + 4 canonical values → (canonical, nil)
//   - env set + invalid value      → ("", ErrInvalidInitiatingActorType)
func ResolveInitiating() (string, error) {
	raw := os.Getenv(initiatingEnvName)
	if raw == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !IsValidActorType(trimmed) {
		return "", ErrInvalidInitiatingActorType
	}
	return trimmed, nil
}

// InjectActorType resolves the actor type from the current process
// context (RUNOPS_ACTOR_TYPE + optional RUNOPS_INITIATING_ACTOR_TYPE for
// daemon path) and writes the result into the provided metadata map.
// Returns the (possibly newly-allocated) map, or an error.
//
// On error the input map is returned unchanged (no partial mutation).
//
// Callers wire it as:
//
//	updated, err := actortype.InjectActorType(mail.Metadata)
//	if err != nil { return fmt.Errorf("dmail emit: %w", err) }
//	mail.Metadata = updated
func InjectActorType(metadata map[string]string) (map[string]string, error) {
	actor, source, err := Resolve()
	if err != nil {
		return metadata, err
	}
	if actor == "" {
		return metadata, nil
	}

	var initiating string
	if actor == actorWorkspaceDaemon {
		initiating, err = ResolveInitiating()
		if err != nil {
			return metadata, err
		}
	}

	if metadata == nil {
		metadata = make(map[string]string, 3)
	}
	metadata[metadataKeyType] = actor
	metadata[metadataKeySource] = source
	if actor == actorWorkspaceDaemon && initiating != "" {
		metadata[metadataKeyInitiating] = initiating
	}
	return metadata, nil
}
